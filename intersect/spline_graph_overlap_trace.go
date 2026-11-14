package intersect

import (
	"context"

	"plainsolid/geom"
)

// OverlappingGraphTraceOptions keeps bounded clipping, discovery and independent
// coverage controls separate. Each underlying exact operation retains its work
// limits; these are not a shared global operation counter.
type OverlappingGraphTraceOptions struct {
	Surface   geom.SurfacePatchOptions
	Discovery SplinePlanePairDiscoveryOptions
	Trace     SplinePairTraceOptions
}

// OverlappingGraphTrace retains original XYZ supports and original UV domains
// restricted to their exact projected overlap. Complete requires independent
// tube coverage and topology on A/B, or strictly disjoint projections. Outside
// the clipped rectangles only projected exclusion is proved. Disjoint has no
// clipped domains or Trace; it is not a coincident/tangent classification.
type OverlappingGraphTrace struct {
	A, B               Patch
	Trace              SplinePairTrace
	ProjectionAxes     [2]int
	Complete, Disjoint bool
}

// CertifyOverlappingAffineXYGraphPair is the XY specialization of
// CertifyOverlappingProjectedGraphPair.
func CertifyOverlappingAffineXYGraphPair(ctx context.Context, a, b Patch, o OverlappingGraphTraceOptions) (OverlappingGraphTrace, error) {
	return CertifyOverlappingProjectedGraphPair(ctx, a, b, [2]int{0, 1}, o)
}

// CertifyOverlappingProjectedGraphPair accepts signed axis-aligned projected
// graph charts, with exactly representable clipped UV endpoints. Seeds and
// coverage are independently proved in original XYZ; model permutations are
// used only to establish exact overlap. Finite discovery cuts outside/on the
// retained A boundary are omitted; interior cuts are sorted and deduplicated.
// The raw guidance count remains bounded before filtering. Failed proofs
// return no partial result.
func CertifyOverlappingProjectedGraphPair(ctx context.Context, a, b Patch, axes [2]int, o OverlappingGraphTraceOptions) (OverlappingGraphTrace, error) {
	fail := func(e error) (OverlappingGraphTrace, error) { return OverlappingGraphTrace{}, e }
	seeds, e := DiscoverOverlappingProjectedGraphPairTubes(ctx, a, b, axes, o.Surface, o.Discovery)
	if e != nil {
		return fail(e)
	}
	if seeds.Disjoint {
		return OverlappingGraphTrace{ProjectionAxes: axes, Complete: true, Disjoint: true}, nil
	}
	traceOptions := o.Trace
	traceOptions.ProjectedGraphFallback.Enabled = true
	traceOptions.GraphProjectionAxes = axes
	traceOptions.Coverage.ProjectedAffineFallback = true
	traceOptions.Coverage.ProjectionAxes = axes
	trace, e := CertifySplinePairBranches(ctx, seeds.A, seeds.B, seeds.Seeds, traceOptions)
	if e != nil {
		return fail(e)
	}
	if !trace.Complete {
		return fail(ErrUnresolved)
	}
	return OverlappingGraphTrace{A: seeds.A, B: seeds.B, Trace: trace, ProjectionAxes: axes, Complete: true}, nil
}

// DiscoverOverlappingProjectedGraphPairTubes prepares independently verified
// local tube proposals on the exact projected overlap of unequal rectangles.
// It makes no completeness or topology claim for the whole source pair.
func DiscoverOverlappingProjectedGraphPairTubes(ctx context.Context, a, b Patch, axes [2]int, p geom.SurfacePatchOptions,
	o SplinePlanePairDiscoveryOptions) (OverlappingGraphSeeds, error) {
	fail := func(e error) (OverlappingGraphSeeds, error) { return OverlappingGraphSeeds{}, e }
	if ctx == nil || !a.valid() || !b.valid() || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if _, e := retainedGraphCuts(o, a); e != nil {
		return fail(e)
	}
	mapped := [2]Patch{a, b}
	for i := range mapped {
		permuted, e := permuteSplineSupport(mapped[i].Surface, [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}, p)
		if e != nil {
			return fail(e)
		}
		mapped[i].Surface = permuted
	}
	pa, e := exactSplinePatch(mapped[0], p)
	if e != nil {
		return fail(e)
	}
	pb, e := exactSplinePatch(mapped[1], p)
	if e != nil {
		return fail(e)
	}
	clip, e := geom.ClipAffineXYGraphOverlap(ctx, pa, pb, p)
	if e != nil {
		return fail(e)
	}
	if clip.Disjoint {
		return OverlappingGraphSeeds{Disjoint: true}, nil
	}
	a.Min, a.Max = geom.Vec2{X: clip.A.UMin, Y: clip.A.VMin}, geom.Vec2{X: clip.A.UMax, Y: clip.A.VMax}
	b.Min, b.Max = geom.Vec2{X: clip.B.UMin, Y: clip.B.VMin}, geom.Vec2{X: clip.B.UMax, Y: clip.B.VMax}
	guidance, e := retainedGraphCuts(o, a)
	if e != nil {
		return fail(e)
	}
	seeds, e := DiscoverProjectedAffineGraphPairTubes(ctx, a, b, p, axes, guidance)
	if e != nil {
		return fail(e)
	}
	return OverlappingGraphSeeds{A: a, B: b, Seeds: seeds}, nil
}
