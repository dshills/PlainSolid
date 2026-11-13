package intersect

import (
	"context"

	"plainsolid/geom"
)

// OverlappingGraphSeeds retains original support identities and exactly clipped
// UV domains. CertifySplinePairBranches must use A and B from this result:
// seeds alone do not prove branch coverage. Strictly disjoint projections need
// no branch proof; boundary-only projected contacts remain unresolved.
type OverlappingGraphSeeds struct {
	A, B     Patch
	Seeds    SplineSeedReport
	Disjoint bool
}

// DiscoverOverlappingAffineXYGraphPairTubes extends graph discovery to unequal
// projected rectangles. Cuts are in the original A parameter units and must
// lie inside the resulting clipped domain. Failure publishes no partial seeds.
// SurfacePatchOptions limits each exact patch operation; discovery retains its
// separate existing arc, tube and per-proof arithmetic limits.
func DiscoverOverlappingAffineXYGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (OverlappingGraphSeeds, error) {
	fail := func(e error) (OverlappingGraphSeeds, error) { return OverlappingGraphSeeds{}, e }
	if ctx == nil || !a.valid() || !b.valid() {
		return fail(geom.ErrInvalid)
	}
	pa, e := exactCoincidencePatch(a, p)
	if e != nil {
		return fail(e)
	}
	pb, e := exactCoincidencePatch(b, p)
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
	seeds, e := DiscoverAffineXYGraphPairTubes(ctx, a, b, p, o)
	if e != nil {
		return fail(e)
	}
	return OverlappingGraphSeeds{A: a, B: b, Seeds: seeds}, nil
}
