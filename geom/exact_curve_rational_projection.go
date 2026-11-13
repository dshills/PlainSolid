package geom

import (
	"context"
	"math/big"
	"sort"
)

type rationalProjectionSpan struct {
	fields             [4]rationalPolynomial
	start, end, lo, hi *big.Rat
}

func intersectExactCurvesRationalProjection(ctx context.Context, a, c *ExactSplineCurve, covector Vec3, o ExactCurveSegmentOptions) (r AffineProjectionCurveIntersection, err error) {
	if ctx == nil || a == nil || c == nil || a.patch == nil || c.patch == nil || !covector.IsFinite() {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		r.Operations = b.operations
		sort.SliceStable(r.Hits, func(i, j int) bool { return r.Hits[i].Evidence.Lower.Cmp(r.Hits[j].Evidence.Lower) < 0 })
		sort.SliceStable(r.Coincident, func(i, j int) bool {
			return r.Coincident[i].ExactProjectionSpan[0].Cmp(r.Coincident[j].ExactProjectionSpan[0]) < 0
		})
	}()
	var spans [2][]rationalProjectionSpan
	project := func(t *big.Rat, index int) *big.Rat {
		return new(big.Rat).Add(r.Maps[index].Offset, new(big.Rat).Mul(r.Maps[index].Scale, t))
	}
	for index, curve := range []*ExactSplineCurve{a, c} {
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		local := o
		local.MaxOperations = remaining
		// Self-comparison proves the global affine projection with no root isolation;
		// the coincident span report is discarded after retaining its exact map.
		proof, e := IntersectExactCurvesSharedAffineCoordinate(ctx, curve, curve, covector, local)
		b.operations += proof.Operations
		if e != nil {
			return r, e
		}
		r.Maps[index] = CurveProjectionParameterMap{Offset: proof.ProjectionOffset, Scale: proof.ProjectionScale}
		stack := []*SurfacePatch{curve.patch}
		for len(stack) > 0 {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if knot, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
				l, h, e := p.split(knot, true, b)
				if e != nil {
					return r, e
				}
				stack = append(stack, h, l)
				continue
			}
			fields, e := sharedProjectionPowers(p, b)
			if e != nil {
				return r, e
			}
			d := p.Domain()
			start, end := project(new(big.Rat).SetFloat64(d.UMin), index), project(new(big.Rat).SetFloat64(d.UMax), index)
			if e := b.check(start); e != nil {
				return r, e
			}
			if e := b.check(end); e != nil {
				return r, e
			}
			lo, hi := start, end
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			spans[index] = append(spans[index], rationalProjectionSpan{fields, start, end, lo, hi})
		}
	}
	var domain [2][2]*big.Rat
	for index, curve := range []*ExactSplineCurve{a, c} {
		lo, hi := curve.Domain()
		domain[index] = [2]*big.Rat{project(new(big.Rat).SetFloat64(lo), index), project(new(big.Rat).SetFloat64(hi), index)}
	}
	globalLo, globalHi := new(big.Rat).Set(spans[0][0].lo), new(big.Rat).Set(spans[0][0].hi)
	for _, s := range spans[0] {
		if s.lo.Cmp(globalLo) < 0 {
			globalLo.Set(s.lo)
		}
		if s.hi.Cmp(globalHi) > 0 {
			globalHi.Set(s.hi)
		}
	}
	blo, bhi := new(big.Rat).Set(spans[1][0].lo), new(big.Rat).Set(spans[1][0].hi)
	for _, s := range spans[1] {
		if s.lo.Cmp(blo) < 0 {
			blo.Set(s.lo)
		}
		if s.hi.Cmp(bhi) > 0 {
			bhi.Set(s.hi)
		}
	}
	if blo.Cmp(globalLo) > 0 {
		globalLo.Set(blo)
	}
	if bhi.Cmp(globalHi) < 0 {
		globalHi.Set(bhi)
	}
	if globalLo.Cmp(globalHi) > 0 {
		r.Complete = true
		return r, nil
	}
	inverse := func(q *big.Rat, index int) (*big.Rat, error) {
		x := new(big.Rat).Quo(new(big.Rat).Sub(q, r.Maps[index].Offset), r.Maps[index].Scale)
		if e := b.check(x); e != nil {
			return nil, e
		}
		return x, nil
	}
	bounds := func(lo, hi *big.Rat) (lower, upper [2]*big.Rat, err error) {
		for index := 0; index < 2; index++ {
			lower[index], err = inverse(lo, index)
			if err != nil {
				return
			}
			upper[index], err = inverse(hi, index)
			if err != nil {
				return
			}
			if lower[index].Cmp(upper[index]) > 0 {
				lower[index], upper[index] = upper[index], lower[index]
			}
		}
		return
	}
	if globalLo.Cmp(globalHi) == 0 {
		var points [2][3]*big.Rat
		var parameters [2]*big.Rat
		for index, curve := range []*ExactSplineCurve{a, c} {
			parameters[index], err = inverse(globalLo, index)
			if err != nil {
				return r, err
			}
			points[index], err = exactCurveRationalPoint(curve.patch, parameters[index], b)
			if err != nil {
				return r, err
			}
		}
		if points[0][0].Cmp(points[1][0]) == 0 && points[0][1].Cmp(points[1][1]) == 0 && points[0][2].Cmp(points[1][2]) == 0 {
			lo, hi, e := bounds(globalLo, globalHi)
			if e != nil {
				return r, e
			}
			r.Hits = append(r.Hits, AffineProjectionCurveHit{Lower: lo, Upper: hi, Parameters: parameters, ExactProjectionSpan: [2]*big.Rat{new(big.Rat).Set(globalLo), new(big.Rat).Set(globalHi)}, Evidence: SharedProjectionCurveHit{Lower: new(big.Rat).Set(globalLo), Upper: new(big.Rat).Set(globalHi), Parameter: new(big.Rat).Set(globalLo), Point: points[0], SharedEndpoint: true}})
		}
		r.Complete = true
		return r, nil
	}
	pairIndex := 0
	for _, sa := range spans[0] {
		for _, sc := range spans[1] {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			if e := b.check(sa.lo); e != nil {
				return r, e
			}
			lo, hi := sa.lo, sa.hi
			if sc.lo.Cmp(lo) > 0 {
				lo = sc.lo
			}
			if sc.hi.Cmp(hi) < 0 {
				hi = sc.hi
			}
			if lo.Cmp(hi) >= 0 {
				continue
			}
			var fields [2][4]rationalPolynomial
			for index, s := range []rationalProjectionSpan{sa, sc} {
				width := new(big.Rat).Sub(s.end, s.start)
				linear := rationalPolynomial{new(big.Rat).Quo(new(big.Rat).Sub(lo, s.start), width), new(big.Rat).Quo(new(big.Rat).Sub(hi, lo), width)}
				for axis := 0; axis < 4; axis++ {
					fields[index][axis], err = composeProjectionPolynomial(s.fields[axis], linear, b)
					if err != nil {
						return r, err
					}
				}
			}
			var common rationalPolynomial
			for axis := 0; axis < 3; axis++ {
				x, e := polynomialProduct(fields[0][axis], fields[1][3], b)
				if e != nil {
					return r, e
				}
				y, e := polynomialProduct(fields[1][axis], fields[0][3], b)
				if e != nil {
					return r, e
				}
				difference, e := sharedProjectionSubtract(x, y, b)
				if e != nil {
					return r, e
				}
				common, e = polynomialGCD(common, difference, b)
				if e != nil {
					return r, e
				}
			}
			display := ParameterSpan{Start: rationalFloatBound(lo, true), End: rationalFloatBound(hi, false), Index: pairIndex}
			pairIndex++
			exactSpan := [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}
			if len(common) == 0 {
				lower, upper, e := bounds(lo, hi)
				if e != nil {
					return r, e
				}
				r.Coincident = append(r.Coincident, AffineProjectionCurveCoincidence{Lower: lower, Upper: upper, Projection: display, ExactProjectionSpan: exactSpan})
				continue
			}
			if len(common) == 1 {
				continue
			}
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			mapQ := func(t *big.Rat) *big.Rat { return new(big.Rat).Add(lo, new(big.Rat).Mul(new(big.Rat).Sub(hi, lo), t)) }
			n, e := isolatePolynomialRoots(common, new(big.Rat), big.NewRat(1, 1), b, remaining, func(l, h *big.Rat, m int) (bool, error) {
				if e := ctx.Err(); e != nil {
					return false, e
				}
				candidate, e := recoverRationalPolynomialRoot(common, l, h, b)
				if e != nil {
					return false, e
				}
				ql, qh := mapQ(l), mapQ(h)
				if candidate == nil && new(big.Rat).Sub(qh, ql).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
					return false, nil
				}
				hit := AffineProjectionCurveHit{ExactProjectionSpan: exactSpan, Evidence: SharedProjectionCurveHit{Lower: ql, Upper: qh, Span: display, Multiplicity: m, Transverse: m == 1, Tangent: m > 1}}
				if candidate != nil {
					q := mapQ(candidate)
					hit.Evidence.Parameter = q
					hit.Evidence.Lower = new(big.Rat).Set(q)
					hit.Evidence.Upper = new(big.Rat).Set(q)
					for index := 0; index < 2; index++ {
						hit.Parameters[index], e = inverse(q, index)
						if e != nil {
							return false, e
						}
					}
					hit.Evidence.Point, e = exactCurveRationalPoint(a.patch, hit.Parameters[0], b)
					if e != nil {
						return false, e
					}
					hit.Evidence.SharedEndpoint = (q.Cmp(domain[0][0]) == 0 || q.Cmp(domain[0][1]) == 0) && (q.Cmp(domain[1][0]) == 0 || q.Cmp(domain[1][1]) == 0)
				}
				hit.Lower, hit.Upper, e = bounds(hit.Evidence.Lower, hit.Evidence.Upper)
				if e != nil {
					return false, e
				}
				r.Hits = append(r.Hits, hit)
				return true, nil
			})
			r.Subdivisions += n
			if e != nil {
				return r, e
			}
		}
	}
	r.Complete = true
	return r, nil
}
func composeProjectionPolynomial(p, linear rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	var out rationalPolynomial
	for i := len(p) - 1; i >= 0; i-- {
		next, e := polynomialProduct(out, linear, b)
		if e != nil {
			return nil, e
		}
		out = next
		if len(out) == 0 {
			out = rationalPolynomial{new(big.Rat)}
		}
		out[0].Add(out[0], p[i])
		if e := b.check(out[0]); e != nil {
			return nil, e
		}
	}
	return trimPolynomial(out), nil
}
