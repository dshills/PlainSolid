package geom

import (
	"context"
	"math/big"
)

// ProjectedCurveEventOrder certifies independent event order in one model
// projection q. Equal q is not a claim of model-point equality. Intervals bound
// q; Parameters bound each original curve parameter. Maps prove q=Offset+Scale*t.
type ProjectedCurveEventOrder struct {
	ExactCurveEventOrder
	Parameters [2][2]*big.Rat
	Maps       [2]CurveProjectionParameterMap
}

// CompareProjectedExactCurvePlaneEvents compares plane events on independent
// single rational Bezier spans with exactly affine nonconstant projections.
// Degrees, parameter ranges and speeds may differ; negative speeds reverse the
// original parameter order. Polynomial composition retains nonbinary rational
// coefficients rather than rounding projected endpoints or knots.
func CompareProjectedExactCurvePlaneEvents(ctx context.Context, curves [2]*ExactSplineCurve, covector Vec3, planes [2][4]*big.Rat, intervals [2][2]*big.Rat, o ExactCurvePlaneEventOptions) (r ProjectedCurveEventOrder, err error) {
	if ctx == nil || !covector.IsFinite() {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if covector == (Vec3{}) {
		return r, ErrDegenerate
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = budget.operations }()
	var polys [2]rationalPolynomial
	var bounds [2][2]*big.Rat
	for i, c := range curves {
		if c == nil || c.patch == nil {
			return r, ErrInvalid
		}
		p := c.patch
		if p.countU != p.degreeU+1 {
			return r, ErrConvergence
		}
		lo, hi := c.Domain()
		if intervals[i][0] == nil || intervals[i][1] == nil || intervals[i][0].Cmp(intervals[i][1]) > 0 {
			return r, ErrInvalid
		}
		if intervals[i][0].Cmp(new(big.Rat).SetFloat64(lo)) < 0 || intervals[i][1].Cmp(new(big.Rat).SetFloat64(hi)) > 0 {
			return r, ErrDomain
		}
		remaining := o.MaxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		proof, e := IntersectExactCurvesSharedAffineCoordinate(ctx, c, c, covector, ExactCurveSegmentOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions, MaxDegree: 64})
		budget.operations += proof.Operations
		if e != nil {
			return r, e
		}
		r.Maps[i] = CurveProjectionParameterMap{Offset: proof.ProjectionOffset, Scale: proof.ProjectionScale}
		project := func(t *big.Rat) *big.Rat {
			return new(big.Rat).Add(r.Maps[i].Offset, new(big.Rat).Mul(r.Maps[i].Scale, t))
		}
		for j, t := range intervals[i] {
			bounds[i][j] = project(t)
			if e := budget.check(bounds[i][j]); e != nil {
				return r, e
			}
		}
		if bounds[i][0].Cmp(bounds[i][1]) > 0 {
			bounds[i][0], bounds[i][1] = bounds[i][1], bounds[i][0]
		}
		start, end := project(new(big.Rat).SetFloat64(lo)), project(new(big.Rat).SetFloat64(hi))
		width := new(big.Rat).Sub(end, start)
		polynomial, e := exactEventPlanePolynomial(p, planes[i], budget)
		if e != nil {
			return r, e
		}
		if len(polynomial) == 0 {
			return r, ErrConvergence
		}
		linear := rationalPolynomial{new(big.Rat).Quo(new(big.Rat).Neg(start), width), new(big.Rat).Inv(width)}
		polys[i], e = composeProjectionPolynomial(polynomial, linear, budget)
		if e != nil {
			return r, e
		}
	}
	r.ExactCurveEventOrder, err = compareExactPolynomialEvents(ctx, polys, bounds, budget, o.MaxSubdivisions)
	if r.Proven {
		for i, interval := range r.Intervals {
			for j, q := range interval {
				r.Parameters[i][j] = new(big.Rat).Quo(new(big.Rat).Sub(q, r.Maps[i].Offset), r.Maps[i].Scale)
				if e := budget.check(r.Parameters[i][j]); e != nil {
					r.Proven = false
					return r, e
				}
			}
			if r.Parameters[i][0].Cmp(r.Parameters[i][1]) > 0 {
				r.Parameters[i][0], r.Parameters[i][1] = r.Parameters[i][1], r.Parameters[i][0]
			}
		}
	}
	return r, err
}
