package geom

import (
	"context"
	"math/big"
)

// ClipRationalLine returns the complete intersection of regularized sphere-pair
// material with origin+t*direction over the exact closed rational range. Raw t
// units, original support masks, isolated touches and clipped interior endpoints
// are retained. No input is rounded to float64. Empty material gives an empty
// result; a singleton interior range is a Point with Boundary=0.
//
// ParameterTolerance controls algebraic enclosures only. Polynomial building,
// isolation and sign classification share Surface.MaxOperations. As in ClipLine,
// exact root codec/order/enclosure calls have individually bounded Surface work
// and bit limits: at most six distinct events, fifteen insertion comparisons,
// and ten*MaxSubdivisions adjacent endpoint enclosure calls. Failure is atomic;
// no manifold or separately regularized one-dimensional material is asserted.
func (s SpherePairMaterial) ClipRationalLine(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (SpherePairMaterialLine, error) {
	var zero SpherePairMaterialLine
	if ctx == nil || s.radius2[0] == nil || s.radius2[1] == nil || minT == nil || maxT == nil || minT.Cmp(maxT) > 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return zero, ErrInvalid
	}
	nonzero := false
	for j := 0; j < 3; j++ {
		if origin[j] == nil || direction[j] == nil {
			return zero, ErrInvalid
		}
		nonzero = nonzero || direction[j].Sign() != 0
	}
	if !nonzero {
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
	limits, e := patchOptions(o.Surface)
	if e != nil {
		return zero, e
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	ro := ExactCurvePlaneEventOptions{MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	a := sphereOverlapArithmetic{ctx: ctx, b: budget}
	a.check(minT)
	a.check(maxT)
	for j := 0; j < 3; j++ {
		a.check(origin[j])
		a.check(direction[j])
	}
	for i := 0; i < 2; i++ {
		a.check(s.radius2[i])
		for _, v := range s.center[i] {
			a.check(v)
		}
	}
	if a.err != nil {
		return zero, a.err
	}
	polynomials := make([]rationalPolynomial, 2)
	for i := 0; i < 2; i++ {
		p := rationalPolynomial{new(big.Rat).Neg(s.radius2[i]), new(big.Rat), new(big.Rat)}
		for j := 0; j < 3; j++ {
			v := a.sub(origin[j], s.center[i][j])
			if a.err != nil {
				return zero, a.err
			}
			product, e := polynomialProduct(rationalPolynomial{v, direction[j]}, rationalPolynomial{v, direction[j]}, budget)
			if e != nil {
				return zero, e
			}
			for k, c := range product {
				p[k] = a.add(p[k], c)
			}
		}
		if a.err != nil {
			return zero, a.err
		}
		polynomials[i] = trimPolynomial(p)
	}
	result, e := intersectMaterialBoundaryRational(ctx, polynomials, new(big.Rat).Set(minT), new(big.Rat).Set(maxT), o, budget, ro, func(signs []int) (BorePointLocation, BoreBoundaryMask) {
		loc, mask := s.location(signs)
		if loc == BoreInside {
			loc = BoreBoundary
		}
		return loc, mask
	})
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return SpherePairMaterialLine{Intervals: result.Intervals, Points: result.Hits}, nil
}
