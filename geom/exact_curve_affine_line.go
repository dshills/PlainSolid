package geom

import (
	"context"
	"math/big"
)

// IntersectExactCurveAffineLine first proves that line equals its endpoint
// affine interpolation at every parameter, using exact homogeneous curve
// identity across all knots and degrees. It then performs the spatial GCD
// intersection. OtherParameter contains the line's original parameter.
// A geometrically straight curve with nonlinear parameter speed is outside
// this sufficient subdomain and returns ErrConvergence before intersection.
func IntersectExactCurveAffineLine(ctx context.Context, curve, line *ExactSplineCurve, o ExactCurveSegmentOptions) (r ExactCurveSegmentIntersection, err error) {
	if ctx == nil || curve == nil || curve.patch == nil || line == nil || line.patch == nil {
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
	if o.MaxDegree == 0 {
		o.MaxDegree = 12
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxDegree < 1 || o.MaxDegree > 64 || o.MaxControlPoints < 2 {
		return r, ErrInvalid
	}
	if line.patch.degreeU > o.MaxDegree || line.patch.countU > o.MaxControlPoints {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations += budget.operations }()
	lo, hi := line.Domain()
	start, e := exactCurveRationalPoint(line.patch, new(big.Rat).SetFloat64(lo), budget)
	if e != nil {
		return r, e
	}
	end, e := exactCurveRationalPoint(line.patch, new(big.Rat).SetFloat64(hi), budget)
	if e != nil {
		return r, e
	}
	net := make([]exactHomogeneous, 4)
	for i, point := range [][3]*big.Rat{start, end} {
		for j := 0; j < 2; j++ {
			for axis := 0; axis < 3; axis++ {
				net[2*i+j][axis] = new(big.Rat).Set(point[axis])
			}
			net[2*i+j][3] = big.NewRat(1, 1)
		}
	}
	p := &SurfacePatch{degreeU: 1, degreeV: 1, countU: 2, countV: 2, knotsU: []float64{lo, lo, hi, hi}, knotsV: []float64{0, 0, 1, 1}, net: net, options: SurfacePatchOptions{MaxPoles: o.MaxControlPoints * 2, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}}
	if e := p.computeBounds(budget); e != nil {
		return r, e
	}
	remaining := o.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	identity, e := CompareExactCurvesContext(ctx, line, &SurfaceIsoCurve{patch: p}, ExactCurveComparisonOptions{MaxSpans: o.MaxControlPoints, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += identity.ExactOperations
	if e != nil {
		return r, e
	}
	if !identity.Complete || !identity.Equal {
		return r, ErrConvergence
	}
	remaining = o.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	local := o
	local.MaxOperations = remaining
	r, err = IntersectExactCurveSegment(ctx, curve, start, end, local)
	budget.maxOperations = o.MaxOperations - r.Operations
	for i := range r.Hits {
		r.Hits[i].OtherParameter = parameterMap(ParameterSpan{Start: lo, End: hi}, r.Hits[i].SegmentParameter)
		if e := budget.check(r.Hits[i].OtherParameter); e != nil {
			r.Complete = false
			return r, e
		}
	}
	return r, err
}
