package intersect

import (
	"context"
	"errors"
	"plainsolid/geom"
)

// DiscoverUniformOverlappingGraphPairTubes adds uniform exact guidance on the
// proved projected overlap of common-XY graphs with different original domains.
// The returned clipped domains are proposal domains only. Callers requiring a
// complete original-pair result must independently certify coverage on their
// original inputs with OverlappingProjectedFallback (XY) enabled. No partial
// proposals are published on failure.
func DiscoverUniformOverlappingGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o UniformGraphDiscoveryOptions) (OverlappingGraphSeeds, error) {
	empty := OverlappingGraphSeeds{}
	if ctx == nil || !a.valid() || !b.valid() {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if e := validateUniformGraphOptions(o); e != nil {
		return empty, e
	}
	if _, e := retainedGraphCuts(o.SplinePlanePairDiscoveryOptions, a); e != nil {
		return empty, e
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return empty, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return empty, e
	}
	clip, e := geom.ClipAffineXYGraphOverlap(ctx, pa, pb, p)
	if e != nil {
		if errors.Is(e, geom.ErrDegenerate) {
			return empty, ErrUnresolved
		}
		return empty, e
	}
	if clip.Disjoint {
		return OverlappingGraphSeeds{Disjoint: true}, nil
	}
	a.Min, a.Max = geom.Vec2{X: clip.A.UMin, Y: clip.A.VMin}, geom.Vec2{X: clip.A.UMax, Y: clip.A.VMax}
	b.Min, b.Max = geom.Vec2{X: clip.B.UMin, Y: clip.B.VMin}, geom.Vec2{X: clip.B.UMax, Y: clip.B.VMax}
	o.SplinePlanePairDiscoveryOptions, e = retainedGraphCuts(o.SplinePlanePairDiscoveryOptions, a)
	if e != nil {
		return empty, e
	}
	seeds, e := DiscoverUniformGraphPairTubes(ctx, a, b, p, o)
	if e != nil {
		return empty, e
	}
	return OverlappingGraphSeeds{A: a, B: b, Seeds: seeds}, nil
}
