package geom

import "context"

// ProjectedTubeTrimIntersection states which original model component supplies
// SideSigns. ProjectionAxes order does not flip the residual: SideSigns always
// describes source[ResidualAxis]-opposite[ResidualAxis] along original trim t.
type ProjectedTubeTrimIntersection struct {
	TubeTrimIntersection
	ProjectionAxes [2]int
	ResidualAxis   int
}

// IntersectQuadraticTrimProjected uses an exact coordinate permutation to prove
// trim contacts in XY, XZ, YZ or reversed axis order. Both original supports
// are retained and rechecked. All returned model events/bounds use original XYZ;
// source UV controls and original parameter roots are never permuted.
func (t VerifiedSurfaceTube) IntersectQuadraticTrimProjected(ctx context.Context, a, b *SurfacePatch, chart int, trim *ExactSplineCurve, axes [2]int, o TubeTrimIntersectionOptions) (r ProjectedTubeTrimIntersection, err error) {
	return t.intersectRationalBezierTrimProjected(ctx, a, b, chart, trim, axes, o, true)
}

// IntersectRationalBezierTrimProjected proves exact Bézier trim contacts of
// degree one through four using the selected original model projection.
func (t VerifiedSurfaceTube) IntersectRationalBezierTrimProjected(ctx context.Context, a, b *SurfacePatch, chart int, trim *ExactSplineCurve, axes [2]int, o TubeTrimIntersectionOptions) (ProjectedTubeTrimIntersection, error) {
	return t.intersectRationalBezierTrimProjected(ctx, a, b, chart, trim, axes, o, false)
}

func (t VerifiedSurfaceTube) intersectRationalBezierTrimProjected(ctx context.Context, a, b *SurfacePatch, chart int, trim *ExactSplineCurve, axes [2]int, o TubeTrimIntersectionOptions, quadraticOnly bool) (r ProjectedTubeTrimIntersection, err error) {
	r.ProjectionAxes = axes
	order, e := modelProjectionPermutation(axes)
	if e != nil || a == nil || b == nil {
		return r, ErrInvalid
	}
	r.ResidualAxis = order[2]
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	proof, _, used, e := t.projectedGraphTube(ctx, axes, TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: o.SurfaceCurveAgreementOptions, Tolerance: o.Tolerance})
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: used}
	defer func() { r.ExactOperations += budget.operations }()
	pa, e := permuteGraphPatch(a, order, budget)
	if e != nil {
		return r, e
	}
	pb, e := permuteGraphPatch(b, order, budget)
	if e != nil {
		return r, e
	}
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	r.TubeTrimIntersection, err = proof.intersectRationalBezierTrim(ctx, pa, pb, chart, trim, o, quadraticOnly)
	inverse := [3]int{}
	for i, j := range order {
		inverse[j] = i
	}
	restored := map[*ExactComposedCurveEvent]*ExactComposedCurveEvent{}
	restore := func(event *ExactComposedCurveEvent) *ExactComposedCurveEvent {
		if event == nil {
			return nil
		}
		if copy := restored[event]; copy != nil {
			return copy
		}
		copy := *event
		copy.cell = permuteGraphCell(event.cell, inverse)
		restored[event] = &copy
		return &copy
	}
	for i := range r.Hits {
		hit := &r.Hits[i]
		hit.TrimEvent = restore(hit.TrimEvent)
		lo, hi := [3]float64{}, [3]float64{}
		for j, k := range inverse {
			lo[j] = coordinate(hit.Bounds.Min, k)
			hi[j] = coordinate(hit.Bounds.Max, k)
		}
		hit.Bounds = Box{Min: Vec3{lo[0], lo[1], lo[2]}, Max: Vec3{hi[0], hi[1], hi[2]}}
	}
	for i := range r.Pending {
		r.Pending[i] = restore(r.Pending[i])
	}
	for i := range r.Coincident {
		piece := &r.Coincident[i]
		for j, event := range piece.Events {
			piece.Events[j] = restore(event)
		}
		for j, event := range piece.Boundary {
			piece.Boundary[j] = restore(event)
		}
	}
	return r, err
}
