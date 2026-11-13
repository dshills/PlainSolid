package geom

import (
	"context"
	"errors"
	"math/big"
)

// IntersectExactCurveAffineLineAlgebraic adds proved algebraic contacts to the
// exact affine-curve intersection. OtherLower/Upper use line's original domain.
// Both affine identity verification and root refinement share the work budget.
func IntersectExactCurveAffineLineAlgebraic(ctx context.Context, curve, line *ExactSplineCurve, o ExactCurveSegmentOptions) (r ExactCurveSegmentAlgebraicIntersection, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	proof, e := IntersectExactCurveAffineLine(ctx, curve, line, o)
	r.ExactCurveSegmentIntersection = proof
	if e != nil && !errors.Is(e, ErrConvergence) {
		return r, e
	}
	if !proof.SupportingRootsComplete || len(proof.Pending) == 0 {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations - proof.Operations, maxBits: o.MaxCoefficientBits}
	if budget.maxOperations < 1 {
		return r, ErrConvergence
	}
	lo, hi := line.Domain()
	start, e := exactCurveRationalPoint(line.patch, new(big.Rat).SetFloat64(lo), budget)
	if e != nil {
		r.Operations += budget.operations
		return r, e
	}
	end, e := exactCurveRationalPoint(line.patch, new(big.Rat).SetFloat64(hi), budget)
	if e != nil {
		r.Operations += budget.operations
		return r, e
	}
	local := o
	local.MaxOperations -= proof.Operations + budget.operations
	local.MaxSubdivisions -= proof.Subdivisions
	if local.MaxOperations < 1 || local.MaxSubdivisions < 1 {
		r.Operations += budget.operations
		return r, ErrConvergence
	}
	r, err = IntersectExactCurveSegmentAlgebraic(ctx, curve, start, end, local)
	r.Operations += proof.Operations + budget.operations
	r.Subdivisions += proof.Subdivisions
	mappingBudget := &polynomialBudget{maxOperations: o.MaxOperations - r.Operations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations += mappingBudget.operations }()
	span := ParameterSpan{Start: lo, End: hi}
	for i := range r.Hits {
		r.Hits[i].OtherParameter = parameterMap(span, r.Hits[i].SegmentParameter)
		if e := mappingBudget.check(r.Hits[i].OtherParameter); e != nil {
			r.Complete = false
			return r, e
		}
	}
	for i := range r.Algebraic {
		r.Algebraic[i].OtherLower = parameterMap(span, r.Algebraic[i].SegmentLower)
		r.Algebraic[i].OtherUpper = parameterMap(span, r.Algebraic[i].SegmentUpper)
		for _, x := range []*big.Rat{r.Algebraic[i].OtherLower, r.Algebraic[i].OtherUpper} {
			if e := mappingBudget.check(x); e != nil {
				r.Complete = false
				return r, e
			}
		}
	}
	return r, err
}
