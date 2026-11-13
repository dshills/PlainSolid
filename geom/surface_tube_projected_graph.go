package geom

import (
	"context"
	"math/big"
)

func modelProjectionPermutation(axes [2]int) (order [3]int, err error) {
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return order, ErrInvalid
	}
	order = [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}
	return order, nil
}
func permuteGraphPatch(p *SurfacePatch, order [3]int, b *polynomialBudget) (*SurfacePatch, error) {
	copy := *p
	copy.net = make([]exactHomogeneous, len(p.net))
	for i, h := range p.net {
		copy.net[i] = exactHomogeneous{h[order[0]], h[order[1]], h[order[2]], h[3]}
	}
	if e := copy.computeBounds(b); e != nil {
		return nil, e
	}
	return &copy, nil
}
func permuteGraphCell(c *ExactComposedCurveCell, order [3]int) *ExactComposedCurveCell {
	copy := *c
	for i := 0; i < 3; i++ {
		copy.power[i] = c.power[order[i]]
	}
	return &copy
}
func (t VerifiedSurfaceTube) projectedGraphTube(ctx context.Context, axes [2]int, o TubeAlgebraicGraphEndpointOptions) (VerifiedSurfaceTube, [3]int, int, error) {
	var zero VerifiedSurfaceTube
	order, e := modelProjectionPermutation(axes)
	if e != nil || ctx == nil || t.a == nil || t.b == nil {
		return zero, order, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, order, 0, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return zero, order, 0, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	a, e := permuteGraphPatch(t.a, order, budget)
	if e != nil {
		return zero, order, budget.operations, e
	}
	b, e := permuteGraphPatch(t.b, order, budget)
	if e != nil {
		return zero, order, budget.operations, e
	}
	remaining := options.MaxOperations - budget.operations
	if remaining < 1 {
		return zero, order, budget.operations, ErrConvergence
	}
	child := SurfacePairTubeOptions{FreeAxis: t.axis, MaxOperations: remaining, MaxCoefficientBits: options.MaxCoefficientBits}
	if t.anchor != nil {
		ranges := pairParameterRanges(SurfacePairParameterBox{t.a.Domain(), t.b.Domain()})
		o := SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: child, AtUpper: t.anchor.free == ranges[t.axis][1], Anchor: t.anchor.parameters}
		used := 0
		proof, e := newBoundaryAnchoredSurfaceTube(ctx, a, b, o, &used)
		return proof, order, budget.operations + used, e
	}
	certificate, e := CertifySurfacePairTube(a, b, child)
	used := budget.operations + certificate.Test.ExactOperations
	if e != nil {
		return zero, order, used, e
	}
	if !certificate.ProvenBranch {
		return zero, order, used, ErrConvergence
	}
	return VerifiedSurfaceTube{a: a, b: b, axis: t.axis, tangentSign: certificate.FreeTangentSign}, order, used, nil
}

// BindProjectedGraphEvent generalizes exact graph membership to any ordered pair
// of model coordinate axes: 0=X,1=Y,2=Z. Both supports must have affine invertible
// projection in those coordinates. Model permutation is exact, the transformed
// tube is independently re-proved, and original model-event lineage is retained.
func (t VerifiedSurfaceTube) BindProjectedGraphEvent(ctx context.Context, event *ExactComposedCurveEvent, axes [2]int, o TubeAlgebraicGraphEndpointOptions) (*TubeGraphEventBinding, error) {
	if event == nil || event.cell == nil || event.root == nil {
		return nil, ErrInvalid
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	o.SurfacePatchOptions = options
	proof, order, used, e := t.projectedGraphTube(ctx, axes, o)
	if e != nil {
		return nil, e
	}
	o.MaxOperations -= used
	if o.MaxOperations < 1 {
		return nil, ErrConvergence
	}
	transformed := *event
	transformed.cell = permuteGraphCell(event.cell, order)
	result, e := proof.BindAffineGraphEvent(ctx, &transformed, o)
	if e != nil {
		return nil, e
	}
	used += result.operations
	inverse := [3]int{}
	for i, j := range order {
		inverse[j] = i
	}
	bounds := result.bounds
	var lo, hi [3]float64
	for i, j := range inverse {
		lo[i] = coordinate(bounds.Min, j)
		hi[i] = coordinate(bounds.Max, j)
	}
	result.bounds = Box{Min: Vec3{lo[0], lo[1], lo[2]}, Max: Vec3{hi[0], hi[1], hi[2]}}
	result.event = event
	result.operations = used
	return result, nil
}

// EvaluateRationalProjectedGraph isolates the exact fixed-free section after a
// model coordinate permutation. Only the opposite support needs affine chosen
// projection; the source may be curved. Returned chart roots remain in original
// parameter units and the model event is restored to original XYZ coordinates.
func (t VerifiedSurfaceTube) EvaluateRationalProjectedGraph(ctx context.Context, free *big.Rat, axes [2]int, o TubeAlgebraicGraphEndpointOptions) (r TubeAlgebraicGraphPointCertificate, err error) {
	if free == nil {
		return r, ErrInvalid
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	o.SurfacePatchOptions = options
	proof, order, used, e := t.projectedGraphTube(ctx, axes, o)
	if e != nil {
		return r, e
	}
	o.MaxOperations -= used
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	r, e = proof.EvaluateRationalGraph(ctx, free, o)
	r.ExactOperations += used
	if e != nil {
		return r, e
	}
	inverse := [3]int{}
	for i, j := range order {
		inverse[j] = i
	}
	event := *r.ModelEvent
	event.cell = permuteGraphCell(event.cell, inverse)
	r.ModelEvent = &event
	bounds := r.PointBounds
	var lo, hi [3]float64
	for i, j := range inverse {
		lo[i] = coordinate(bounds.Min, j)
		hi[i] = coordinate(bounds.Max, j)
	}
	r.PointBounds = Box{Min: Vec3{lo[0], lo[1], lo[2]}, Max: Vec3{hi[0], hi[1], hi[2]}}
	return r, nil
}

// DiscoverAlgebraicProjectedGraphEndpoint uses the exact represented lower or
// upper original free bound, then isolates the section in the chosen model
// projection. It requires no rational dependent-coordinate proposal.
func (t VerifiedSurfaceTube) DiscoverAlgebraicProjectedGraphEndpoint(ctx context.Context, upper bool, axes [2]int, o TubeAlgebraicGraphEndpointOptions) (TubeAlgebraicGraphEndpointCertificate, error) {
	_, domain, e := t.ExactFreeDomain()
	if e != nil {
		return TubeAlgebraicGraphEndpointCertificate{}, e
	}
	free := domain.Lower
	if upper {
		free = domain.Upper
	}
	return t.EvaluateRationalProjectedGraph(ctx, free, axes, o)
}
