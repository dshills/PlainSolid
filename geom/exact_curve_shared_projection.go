package geom

import (
	"context"
	"math/big"
	"sort"
)

// SharedProjectionCurveHit isolates a proved common point at the same original
// parameter of both curves. Parameter and Point are populated only for recovered
// rational roots; Lower/Upper always enclose the exact root. Multiplicity is the
// order of the vector difference on this knot span. A simple root is transverse;
// higher order means equal tangents, since the shared affine projection has
// nonzero speed. Knot endpoint hits may appear once per incident span.
type SharedProjectionCurveHit struct {
	Lower, Upper, Parameter             *big.Rat
	Point                               [3]*big.Rat
	Span                                ParameterSpan
	Multiplicity                        int
	Transverse, Tangent, SharedEndpoint bool
}

// SharedProjectionCurveIntersection is exhaustive when Complete, including
// algebraic isolated hits and whole coincident parameter spans. It does not
// approximate algebraic points by rational representatives.
type SharedProjectionCurveIntersection struct {
	Hits                              []SharedProjectionCurveHit
	Coincident                        []ParameterSpan
	ProjectionOffset, ProjectionScale *big.Rat
	Complete                          bool
	Operations, Subdivisions          int
}

// IntersectExactCurvesSharedAffineCoordinate proves covector·A(t) and
// covector·B(t) equal the same nonconstant affine function on their entire
// respective domains. Hence any spatial intersection must have equal parameters.
// On the union knot partition it isolates the exact GCD of all three homogeneous
// difference polynomials. Spatial nonplanar rational curves are supported.
// Nonaffine or mismatched parameter projections are explicitly unresolved.
func IntersectExactCurvesSharedAffineCoordinate(ctx context.Context, a, c *ExactSplineCurve, covector Vec3, o ExactCurveSegmentOptions) (r SharedProjectionCurveIntersection, err error) {
	if ctx == nil || a == nil || c == nil || a.patch == nil || c.patch == nil || !covector.IsFinite() {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 12
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxDegree < 1 || o.MaxDegree > 64 || o.MaxControlPoints < 2 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	if covector == (Vec3{}) {
		return r, ErrDegenerate
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	direction := [3]*big.Rat{new(big.Rat).SetFloat64(covector.X), new(big.Rat).SetFloat64(covector.Y), new(big.Rat).SetFloat64(covector.Z)}
	projection := func(point [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := 0; i < 3; i++ {
			v.Add(v, new(big.Rat).Mul(direction[i], point[i]))
		}
		return v
	}
	var offsets, scales [2]*big.Rat
	for index, curve := range []*ExactSplineCurve{a, c} {
		if curve.patch.degreeU > o.MaxDegree || curve.patch.countU > o.MaxControlPoints {
			return r, ErrConvergence
		}
		lo, hi := curve.Domain()
		start, e := exactCurveRationalPoint(curve.patch, new(big.Rat).SetFloat64(lo), b)
		if e != nil {
			return r, e
		}
		end, e := exactCurveRationalPoint(curve.patch, new(big.Rat).SetFloat64(hi), b)
		if e != nil {
			return r, e
		}
		scales[index] = new(big.Rat).Quo(new(big.Rat).Sub(projection(end), projection(start)), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)))
		offsets[index] = new(big.Rat).Sub(projection(start), new(big.Rat).Mul(scales[index], new(big.Rat).SetFloat64(lo)))
		if e := b.check(scales[index]); e != nil {
			return r, e
		}
		if e := b.check(offsets[index]); e != nil {
			return r, e
		}
		if scales[index].Sign() == 0 {
			return r, ErrConvergence
		}
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
			linear := rationalPolynomial{new(big.Rat).Add(offsets[index], new(big.Rat).Mul(scales[index], new(big.Rat).SetFloat64(d.UMin))), new(big.Rat).Mul(scales[index], new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin)))}
			expected, e := polynomialProduct(linear, fields[3], b)
			if e != nil {
				return r, e
			}
			actual := make(rationalPolynomial, p.degreeU+1)
			for j := range actual {
				actual[j] = new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					if j < len(fields[axis]) {
						actual[j].Add(actual[j], new(big.Rat).Mul(direction[axis], fields[axis][j]))
					}
				}
			}
			difference, e := sharedProjectionSubtract(actual, expected, b)
			if e != nil {
				return r, e
			}
			if len(difference) != 0 {
				return r, ErrConvergence
			}
		}
	}
	if offsets[0].Cmp(offsets[1]) != 0 || scales[0].Cmp(scales[1]) != 0 {
		return r, ErrConvergence
	}
	r.ProjectionOffset, r.ProjectionScale = new(big.Rat).Set(offsets[0]), new(big.Rat).Set(scales[0])
	al, ah := a.Domain()
	cl, ch := c.Domain()
	lo, hi := max(al, cl), min(ah, ch)
	if lo > hi {
		r.Complete = true
		return r, nil
	}
	if lo == hi {
		x := new(big.Rat).SetFloat64(lo)
		ap, e := exactCurveRationalPoint(a.patch, x, b)
		if e != nil {
			return r, e
		}
		cp, e := exactCurveRationalPoint(c.patch, x, b)
		if e != nil {
			return r, e
		}
		if ap[0].Cmp(cp[0]) == 0 && ap[1].Cmp(cp[1]) == 0 && ap[2].Cmp(cp[2]) == 0 {
			r.Hits = append(r.Hits, SharedProjectionCurveHit{Lower: new(big.Rat).Set(x), Upper: new(big.Rat).Set(x), Parameter: x, Point: ap, SharedEndpoint: true})
		}
		r.Complete = true
		return r, nil
	}
	cuts := []float64{lo, hi}
	for _, curve := range []*ExactSplineCurve{a, c} {
		for _, k := range curve.patch.knotsU {
			if k > lo && k < hi {
				cuts = append(cuts, k)
			}
		}
	}
	sort.Float64s(cuts)
	unique := cuts[:0]
	for _, x := range cuts {
		if len(unique) == 0 || x != unique[len(unique)-1] {
			unique = append(unique, x)
		}
	}
	cuts = unique
	for i := 0; i < len(cuts)-1; i++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		span := ParameterSpan{Start: cuts[i], End: cuts[i+1], Index: i}
		pa, e := trimPatchBudget(a.patch, ParameterRectangle{span.Start, span.End, 0, 1}, b)
		if e != nil {
			return r, e
		}
		pc, e := trimPatchBudget(c.patch, ParameterRectangle{span.Start, span.End, 0, 1}, b)
		if e != nil {
			return r, e
		}
		fa, e := sharedProjectionPowers(pa, b)
		if e != nil {
			return r, e
		}
		fc, e := sharedProjectionPowers(pc, b)
		if e != nil {
			return r, e
		}
		var common rationalPolynomial
		for axis := 0; axis < 3; axis++ {
			x, e := polynomialProduct(fa[axis], fc[3], b)
			if e != nil {
				return r, e
			}
			y, e := polynomialProduct(fc[axis], fa[3], b)
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
		if len(common) == 0 {
			r.Coincident = append(r.Coincident, span)
			continue
		}
		if len(common) == 1 {
			continue
		}
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		n, e := isolatePolynomialRoots(common, new(big.Rat), big.NewRat(1, 1), b, remaining, func(l, h *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			candidate, e := recoverRationalPolynomialRoot(common, l, h, b)
			if e != nil {
				return false, e
			}
			lower, upper := parameterMap(span, l), parameterMap(span, h)
			if candidate == nil && new(big.Rat).Sub(upper, lower).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
				return false, nil
			}
			hit := SharedProjectionCurveHit{Lower: lower, Upper: upper, Span: span, Multiplicity: m, Transverse: m == 1, Tangent: m > 1}
			if candidate != nil {
				parameter := parameterMap(span, candidate)
				point, _, e := exactSurfaceJet(pa, parameter, new(big.Rat), b)
				if e != nil {
					return false, e
				}
				hit.Parameter = parameter
				hit.Point = point
				hit.Lower = new(big.Rat).Set(parameter)
				hit.Upper = new(big.Rat).Set(parameter)
				hit.SharedEndpoint = (parameter.Cmp(new(big.Rat).SetFloat64(al)) == 0 || parameter.Cmp(new(big.Rat).SetFloat64(ah)) == 0) && (parameter.Cmp(new(big.Rat).SetFloat64(cl)) == 0 || parameter.Cmp(new(big.Rat).SetFloat64(ch)) == 0)
			}
			r.Hits = append(r.Hits, hit)
			return true, nil
		})
		r.Subdivisions += n
		if e != nil {
			return r, e
		}
	}
	r.Complete = true
	return r, nil
}

func sharedProjectionPowers(p *SurfacePatch, b *polynomialBudget) (out [4]rationalPolynomial, err error) {
	for axis := 0; axis < 4; axis++ {
		values := make([]*big.Rat, p.countU)
		for i := range values {
			values[i] = p.net[i*p.countV][axis]
		}
		out[axis], err = bernsteinToPower(values, b)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
func sharedProjectionSubtract(a, c rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	r := make(rationalPolynomial, max(len(a), len(c)))
	for i := range r {
		r[i] = new(big.Rat)
		if i < len(a) {
			r[i].Set(a[i])
		}
		if i < len(c) {
			r[i].Sub(r[i], c[i])
		}
		if e := b.check(r[i]); e != nil {
			return nil, e
		}
	}
	return trimPolynomial(r), nil
}
