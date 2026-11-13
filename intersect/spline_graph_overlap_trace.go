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
// used only to establish exact overlap. Failed proofs return no partial result.
func CertifyOverlappingProjectedGraphPair(ctx context.Context, a, b Patch, axes [2]int, o OverlappingGraphTraceOptions) (OverlappingGraphTrace, error) {
	fail := func(e error) (OverlappingGraphTrace, error) { return OverlappingGraphTrace{}, e }
	if ctx == nil || !a.valid() || !b.valid() || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	mapped := [2]Patch{a, b}
	for i := range mapped {
		surface, ok := mapped[i].Surface.(*geom.BSplineSurface)
		if !ok {
			return fail(geom.ErrInvalid)
		}
		permuted, e := surface.PermutedModelAxes([3]int{axes[0], axes[1], 3 - axes[0] - axes[1]})
		if e != nil {
			return fail(e)
		}
		mapped[i].Surface = permuted
	}
	pa, e := exactCoincidencePatch(mapped[0], o.Surface)
	if e != nil {
		return fail(e)
	}
	pb, e := exactCoincidencePatch(mapped[1], o.Surface)
	if e != nil {
		return fail(e)
	}
	clip, e := geom.ClipAffineXYGraphOverlap(ctx, pa, pb, o.Surface)
	if e != nil {
		return fail(e)
	}
	if clip.Disjoint {
		return OverlappingGraphTrace{ProjectionAxes: axes, Complete: true, Disjoint: true}, nil
	}
	a.Min, a.Max = geom.Vec2{X: clip.A.UMin, Y: clip.A.VMin}, geom.Vec2{X: clip.A.UMax, Y: clip.A.VMax}
	b.Min, b.Max = geom.Vec2{X: clip.B.UMin, Y: clip.B.VMin}, geom.Vec2{X: clip.B.UMax, Y: clip.B.VMax}
	seeds, e := DiscoverProjectedAffineGraphPairTubes(ctx, a, b, o.Surface, axes, o.Discovery)
	if e != nil {
		return fail(e)
	}
	traceOptions := o.Trace
	traceOptions.ProjectedGraphFallback.Enabled = true
	traceOptions.GraphProjectionAxes = axes
	traceOptions.Coverage.ProjectedAffineFallback = true
	traceOptions.Coverage.ProjectionAxes = axes
	trace, e := CertifySplinePairBranches(ctx, a, b, seeds, traceOptions)
	if e != nil {
		return fail(e)
	}
	if !trace.Complete {
		return fail(ErrUnresolved)
	}
	return OverlappingGraphTrace{A: a, B: b, Trace: trace, ProjectionAxes: axes, Complete: true}, nil
}
