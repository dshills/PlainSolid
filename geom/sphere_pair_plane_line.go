package geom

import (
	"context"
	"math/big"
)

// ClipRationalLine partitions uv=origin+t*direction over exact rational bounds.
// Parameters retain these original units; neither chart embedding nor limits
// are rounded to float64. ParameterTolerance controls root enclosure only.
// Global regularized material and exposed original source masks are preserved.
// Failures are atomic; no manifold or planar re-regularization is asserted.
func (s SpherePairPlaneMaterial) ClipRationalLine(ctx context.Context, origin, direction [2]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (SpherePairMaterialLine, error) {
	var zero SpherePairMaterialLine
	if ctx == nil || !s.valid || minT == nil || maxT == nil || minT.Cmp(maxT) > 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return zero, ErrInvalid
	}
	for j := 0; j < 2; j++ {
		if origin[j] == nil || direction[j] == nil {
			return zero, ErrInvalid
		}
	}
	if direction[0].Sign() == 0 && direction[1].Sign() == 0 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return zero, ErrInvalid
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return zero, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	for _, v := range []*big.Rat{origin[0], origin[1], direction[0], direction[1], minT, maxT} {
		if e := b.check(v); e != nil {
			return zero, e
		}
	}
	var org, dir [3]*big.Rat
	for j := 0; j < 3; j++ {
		org[j] = new(big.Rat).Set(s.origin[j])
		dir[j] = new(big.Rat)
		for k := 0; k < 2; k++ {
			a, d := new(big.Rat).Mul(s.basis[k][j], origin[k]), new(big.Rat).Mul(s.basis[k][j], direction[k])
			for _, v := range []*big.Rat{a, d} {
				if e := b.check(v); e != nil {
					return zero, e
				}
			}
			org[j].Add(org[j], a)
			dir[j].Add(dir[j], d)
		}
		for _, v := range []*big.Rat{org[j], dir[j]} {
			if e := b.check(v); e != nil {
				return zero, e
			}
		}
	}
	ps := make([]rationalPolynomial, 2)
	for i := 0; i < 2; i++ {
		p := rationalPolynomial{new(big.Rat).Neg(s.source.radius2[i]), new(big.Rat), new(big.Rat)}
		for j := 0; j < 3; j++ {
			v := new(big.Rat).Sub(org[j], s.source.center[i][j])
			product, e := polynomialProduct(rationalPolynomial{v, dir[j]}, rationalPolynomial{v, dir[j]}, b)
			if e != nil {
				return zero, e
			}
			for k, c := range product {
				p[k].Add(p[k], c)
			}
		}
		for _, v := range p {
			if e := b.check(v); e != nil {
				return zero, e
			}
		}
		ps[i] = trimPolynomial(p)
	}
	r, e := intersectMaterialBoundaryRational(ctx, ps, new(big.Rat).Set(minT), new(big.Rat).Set(maxT), o, b, ro, func(v []int) (BorePointLocation, BoreBoundaryMask) {
		loc, mask := s.source.location(v)
		if loc == BoreInside {
			loc = BoreBoundary
		}
		return loc, mask
	})
	if e != nil {
		return zero, e
	}
	return SpherePairMaterialLine{Intervals: r.Intervals, Points: r.Hits}, nil
}
