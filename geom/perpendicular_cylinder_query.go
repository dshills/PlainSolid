package geom

import (
	"context"
	"math/big"
)

const (
	PerpendicularCylinderXSide BoreBoundaryMask = 1 << iota
	PerpendicularCylinderZSide
	PerpendicularCylinderXLower
	PerpendicularCylinderXUpper
	PerpendicularCylinderZLower
	PerpendicularCylinderZUpper
)

// PerpendicularCylinderCommon is the exact closed intersection of two finite
// cylinders centered on the intersecting X and Z axes. The common origin is
// local. Each cylinder uses its own radius and closed axial interval.
type PerpendicularCylinderCommon struct {
	xRadius, zRadius *big.Rat
	xRange, zRange   [2]*big.Rat
}

func NewPerpendicularCylinderCommon(xRadius, zRadius float64, xRange, zRange [2]float64) (PerpendicularCylinderCommon, error) {
	var empty PerpendicularCylinderCommon
	if !finite(xRadius) || !finite(zRadius) || xRadius <= 0 || zRadius <= 0 {
		return empty, ErrInvalid
	}
	for _, a := range [][2]float64{xRange, zRange} {
		if !finite(a[0]) || !finite(a[1]) || a[0] >= a[1] {
			return empty, ErrInvalid
		}
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	return PerpendicularCylinderCommon{q(xRadius), q(zRadius), [2]*big.Rat{q(xRange[0]), q(xRange[1])}, [2]*big.Rat{q(zRange[0]), q(zRange[1])}}, nil
}

func perpendicularCylinderLocation(signs []int) (BorePointLocation, BoreBoundaryMask) {
	for _, s := range signs {
		if s > 0 {
			return BoreOutside, 0
		}
	}
	var mask BoreBoundaryMask
	for i, s := range signs {
		if s == 0 {
			mask |= 1 << i
		}
	}
	if mask != 0 {
		return BoreBoundary, mask
	}
	return BoreInside, 0
}

// Classify returns exact represented-point location and all exposed support
// owners. Lower-dimensional contacts are retained as boundary points.
func (s PerpendicularCylinderCommon) Classify(p Vec3) (BorePointLocation, BoreBoundaryMask, error) {
	if s.xRadius == nil || !p.IsFinite() {
		return BoreOutside, 0, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	x, y, z := q(p.X), q(p.Y), q(p.Z)
	a := new(big.Rat).Sub(new(big.Rat).Add(sq(y), sq(z)), sq(s.xRadius))
	b := new(big.Rat).Sub(new(big.Rat).Add(sq(x), sq(y)), sq(s.zRadius))
	l, m := perpendicularCylinderLocation([]int{a.Sign(), b.Sign(), s.xRange[0].Cmp(x), x.Cmp(s.xRange[1]), s.zRange[0].Cmp(z), z.Cmp(s.zRange[1])})
	return l, m, nil
}

// IntersectLineBoundary isolates and orders exact parameter roots of all six
// support equations, and retains continuous contact on caps and cylinder
// generators. Masks in the returned record use the PerpendicularCylinder
// constants.
func (s PerpendicularCylinderCommon) IntersectLineBoundary(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (BoreLineBoundary, error) {
	var empty BoreLineBoundary
	if ctx == nil || s.xRadius == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) ||
		!finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return empty, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	affine := [3]rationalPolynomial{{q(origin.X), q(direction.X)}, {q(origin.Y), q(direction.Y)}, {q(origin.Z), q(direction.Z)}}
	quad := [3]rationalPolynomial{}
	for i, p := range affine {
		x, e := polynomialProduct(p, p, b)
		if e != nil {
			return empty, e
		}
		quad[i] = x
	}
	polynomials := make([]rationalPolynomial, 0, 6)
	for i, m := range [2]int{2, 0} {
		a := rationalPolynomial{new(big.Rat), new(big.Rat), new(big.Rat)}
		for _, k := range [2]int{1, m} {
			for j, v := range quad[k] {
				a[j].Add(a[j], v)
			}
		}
		r := s.xRadius
		if i == 1 {
			r = s.zRadius
		}
		a[0].Sub(a[0], new(big.Rat).Mul(r, r))
		polynomials = append(polynomials, trimPolynomial(a))
	}
	for i, a := range [2][2]*big.Rat{s.xRange, s.zRange} {
		p := affine[2*i]
		polynomials = append(polynomials,
			trimPolynomial(rationalPolynomial{new(big.Rat).Sub(a[0], p[0]), new(big.Rat).Neg(p[1])}),
			trimPolynomial(rationalPolynomial{new(big.Rat).Sub(p[0], a[1]), p[1]}))
	}
	for _, p := range polynomials {
		for _, v := range p {
			if e := b.check(v); e != nil {
				return empty, e
			}
		}
	}
	return intersectMaterialBoundary(ctx, polynomials, minT, maxT, o, b, ro, perpendicularCylinderLocation)
}
