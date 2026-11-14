package geom

import (
	"context"
	"math/big"
	"sort"
)

// LocateWrappedFrustum exactly classifies material inside a certified capped
// homothetic rational frustum, including noncircular positive-weight profiles.
func LocateWrappedFrustum(ctx context.Context, w *WrappedSurfacePatch, point Vec3, o SurfacePatchOptions) (CylinderLocation, error) {
	if !point.IsFinite() {
		return CylinderUnknown, ErrInvalid
	}
	p, e := CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return CylinderUnknown, e
	}
	options, _ := patchOptions(o)
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: p.ExactOperations}
	z := new(big.Rat).SetFloat64(point.Z)
	if z.Cmp(p.LowerZ) < 0 || z.Cmp(p.UpperZ) > 0 {
		return CylinderOutside, nil
	}
	slope := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperRadius, p.LowerRadius), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	radius := new(big.Rat).Add(p.LowerRadius, new(big.Rat).Mul(slope, new(big.Rat).Sub(z, p.LowerZ)))
	if e := budget.check(radius); e != nil {
		return CylinderUnknown, e
	}
	u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(point.X), p.CenterX), radius)
	v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(point.Y), p.CenterY), radius)
	for _, n := range []*big.Rat{z, u, v} {
		if e := budget.check(n); e != nil {
			return CylinderUnknown, e
		}
	}
	quadrant := 0
	if u.Sign() < 0 {
		if v.Sign() >= 0 {
			quadrant = 1
			u, v = new(big.Rat).Set(v), new(big.Rat).Neg(u)
		} else {
			quadrant = 2
			u.Neg(u)
			v.Neg(v)
		}
	} else if v.Sign() < 0 {
		quadrant = 3
		u, v = new(big.Rat).Neg(v), new(big.Rat).Set(u)
	}
	one := big.NewRat(1, 1)
	if u.Cmp(one) > 0 || v.Cmp(one) > 0 {
		return CylinderOutside, nil
	}
	sum := new(big.Rat).Sub(new(big.Rat).Add(u, v), one)
	sign := -1
	if sum.Sign() >= 0 {
		j := 4 * quadrant
		a, b, c := w.patch.net[j][3], w.patch.net[j+2][3], w.patch.net[j+4][3]
		left := new(big.Rat).Mul(new(big.Rat).Mul(a, c), new(big.Rat).Mul(sum, sum))
		right := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(b, b))
		right.Mul(right, new(big.Rat).Sub(one, u))
		right.Mul(right, new(big.Rat).Sub(one, v))
		for _, n := range []*big.Rat{u, v, sum, left, right} {
			if e := budget.check(n); e != nil {
				return CylinderUnknown, e
			}
		}
		sign = left.Cmp(right)
	}
	if sign > 0 {
		return CylinderOutside, nil
	}
	if sign == 0 || z.Cmp(p.LowerZ) == 0 || z.Cmp(p.UpperZ) == 0 {
		return CylinderBoundary, nil
	}
	return CylinderInside, nil
}

// IntersectWrappedCircularFrustumLine intersects a bounded original-parameter
// line with a certified circular-profile frustum and its two disk caps. The
// side equation remains quadratic because radius is affine in height. Exact
// axial clipping removes extraneous roots of the unbounded double cone.
// Generator-coincident intervals, cap overlaps, tangencies and rim ancestry
// follow CylinderLineIntersections semantics. No apex or zero radius admitted.
func IntersectWrappedCircularFrustumLine(ctx context.Context, w *WrappedSurfacePatch, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (CylinderLineIntersections, error) {
	if !origin.IsFinite() || !direction.IsFinite() || !finite(minT) || !finite(maxT) {
		return CylinderLineIntersections{}, ErrInvalid
	}
	return IntersectWrappedCircularFrustumLineRational(ctx, w, rationalLineVec3(origin), rationalLineVec3(direction),
		new(big.Rat).SetFloat64(minT), new(big.Rat).SetFloat64(maxT), o)
}

// IntersectWrappedCircularFrustumLineRational uses exact line coefficients and
// bounds and returns contacts in the supplied unnormalized parameter.
func IntersectWrappedCircularFrustumLineRational(ctx context.Context, w *WrappedSurfacePatch, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o ToroidLineOptions) (CylinderLineIntersections, error) {
	empty := CylinderLineIntersections{}
	options, e := validateRationalCircularLine(ctx, origin[:], direction[:], minT, maxT, o)
	if e != nil {
		return empty, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	p, e := CertifyWrappedFrustumEmbedding(ctx, w, o.Surface)
	if e != nil {
		return empty, e
	}
	if !p.Circular {
		return empty, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: p.ExactOperations}
	x := new(big.Rat).Sub(origin[0], p.CenterX)
	y := new(big.Rat).Sub(origin[1], p.CenterY)
	dx, dy, dz := direction[0], direction[1], direction[2]
	z := origin[2]
	slope := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperRadius, p.LowerRadius), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	r0 := new(big.Rat).Add(p.LowerRadius, new(big.Rat).Mul(slope, new(big.Rat).Sub(z, p.LowerZ)))
	rd := new(big.Rat).Mul(slope, dz)
	poly := rationalPolynomial{
		new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y)), new(big.Rat).Mul(r0, r0)),
		new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(x, dx), new(big.Rat).Mul(y, dy)), new(big.Rat).Mul(r0, rd))),
		new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy)), new(big.Rat).Mul(rd, rd)),
	}
	for _, n := range append([]*big.Rat{x, y, dx, dy, dz, z}, poly...) {
		if e := budget.check(n); e != nil {
			return empty, e
		}
	}
	poly = trimPolynomial(poly)
	lo, hi := new(big.Rat).Set(minT), new(big.Rat).Set(maxT)
	tol := new(big.Rat).SetFloat64(o.ParameterTolerance)
	exact := func(t *big.Rat) ToroidLineRoot { return ToroidLineRoot{new(big.Rat).Set(t), new(big.Rat).Set(t), 1} }
	isolate := func(a, b *big.Rat) ([]ToroidLineRoot, error) {
		var roots []ToroidLineRoot
		left, e := polynomialValue(poly, a, budget)
		if e != nil {
			return nil, e
		}
		right, e := polynomialValue(poly, b, budget)
		if e != nil {
			return nil, e
		}
		_, e = isolatePolynomialRoots(poly, a, b, budget, o.MaxSubdivisions, func(l, h *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if l.Cmp(h) != 0 && (new(big.Rat).Sub(h, l).Cmp(tol) > 0 ||
				left.Sign() == 0 && l.Cmp(a) == 0 || right.Sign() == 0 && h.Cmp(b) == 0) {
				// A distinct root bracket cannot touch a known endpoint root.
				return false, nil
			}
			roots = append(roots, ToroidLineRoot{new(big.Rat).Set(l), new(big.Rat).Set(h), m})
			return true, nil
		})
		sort.Slice(roots, func(i, j int) bool { return roots[i].Min.Cmp(roots[j].Min) < 0 })
		return roots, e
	}
	result := CylinderLineIntersections{}
	// Exact intersection of the parameter interval with the cylinder height.
	sideLo, sideHi := new(big.Rat).Set(lo), new(big.Rat).Set(hi)
	if dz.Sign() == 0 {
		if z.Cmp(p.LowerZ) < 0 || z.Cmp(p.UpperZ) > 0 {
			sideLo.SetInt64(1)
			sideHi.SetInt64(0)
		}
	} else {
		a := new(big.Rat).Quo(new(big.Rat).Sub(p.LowerZ, z), dz)
		b := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperZ, z), dz)
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		if a.Cmp(sideLo) > 0 {
			sideLo.Set(a)
		}
		if b.Cmp(sideHi) < 0 {
			sideHi.Set(b)
		}
	}
	for _, n := range []*big.Rat{lo, hi, sideLo, sideHi} {
		if e := budget.check(n); e != nil {
			return empty, e
		}
	}
	if sideLo.Cmp(sideHi) <= 0 {
		if len(poly) == 0 {
			result.Overlaps = append(result.Overlaps, CylinderLineOverlap{exact(sideLo), exact(sideHi), CylinderLateral})
		} else {
			roots, e := isolate(sideLo, sideHi)
			if e != nil {
				return empty, e
			}
			for _, r := range roots {
				result.Hits = append(result.Hits, CylinderLineHit{r, CylinderLateral})
			}
		}
	}
	for i, height := range []*big.Rat{p.LowerZ, p.UpperZ} {
		part := CylinderLowerCap
		if i == 1 {
			part = CylinderUpperCap
		}
		if dz.Sign() != 0 {
			t := new(big.Rat).Quo(new(big.Rat).Sub(height, z), dz)
			if e := budget.check(t); e != nil {
				return empty, e
			}
			if t.Cmp(lo) < 0 || t.Cmp(hi) > 0 {
				continue
			}
			v, e := polynomialValue(poly, t, budget)
			if e != nil {
				return empty, e
			}
			if v.Sign() > 0 {
				continue
			}
			merged := false
			if v.Sign() == 0 {
				for j := range result.Hits {
					r := &result.Hits[j]
					// The cap plane clips the side interval. Its exact root is
					// already present; a neighboring enclosure is a distinct root.
					if r.Parameter.Min.Cmp(t) == 0 && r.Parameter.Max.Cmp(t) == 0 {
						r.Parts |= part
						multiplicity := r.Parameter.Multiplicity
						r.Parameter = exact(t)
						r.Parameter.Multiplicity = multiplicity
						merged = true
						break
					}
				}
			}
			if !merged {
				if v.Sign() == 0 {
					part |= CylinderLateral
				}
				result.Hits = append(result.Hits, CylinderLineHit{exact(t), part})
			}
			continue
		}
		if z.Cmp(height) != 0 {
			continue
		}
		for j := range result.Hits {
			result.Hits[j].Parts |= part
		}
		if len(poly) == 0 {
			result.Overlaps = append(result.Overlaps, CylinderLineOverlap{exact(lo), exact(hi), part})
			continue
		}
		roots, e := isolate(lo, hi)
		if e != nil {
			return empty, e
		}
		a, b := exact(lo), exact(hi)
		low, e := polynomialValue(poly, lo, budget)
		if e != nil {
			return empty, e
		}
		high, e := polynomialValue(poly, hi, budget)
		if e != nil {
			return empty, e
		}
		if low.Sign() > 0 {
			if len(roots) == 0 {
				continue
			}
			a = roots[0]
		}
		if high.Sign() > 0 {
			if len(roots) == 0 {
				continue
			}
			b = roots[len(roots)-1]
		}
		if len(roots) == 1 && roots[0].Multiplicity == 2 {
			for j := range result.Hits {
				result.Hits[j].Parts |= part
			}
			continue
		}
		result.Overlaps = append(result.Overlaps, CylinderLineOverlap{a, b, part})
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	sort.Slice(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
	return result, nil
}
