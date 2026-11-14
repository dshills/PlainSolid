package intersect

import (
	"context"
	"errors"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestUniformOverlapDiscoveryGuards(t *testing.T) {
	makePatch := func(x float64) Patch {
		k := []float64{0, 0, 1, 1}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: x}, {X: x, Y: 1}, {X: x + 1}, {X: x + 1, Y: 1}}, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}
	}
	a, b := makePatch(0), makePatch(2)
	ctx := context.Background()
	r, e := DiscoverUniformOverlappingGraphPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, UniformGraphDiscoveryOptions{})
	if e != nil || !r.Disjoint || len(r.Seeds.Attempts) != 0 {
		t.Fatal("exact projected exclusion", e)
	}
	for _, o := range []UniformGraphDiscoveryOptions{{GridDivisions: 3}, {AxisDivisions: [2]int{65, 0}}, {SplinePlanePairDiscoveryOptions: SplinePlanePairDiscoveryOptions{UCuts: []float64{math.NaN()}}}} {
		r, e := DiscoverUniformOverlappingGraphPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, o)
		if !errors.Is(e, geom.ErrInvalid) || r.Disjoint || len(r.Seeds.Attempts) != 0 {
			t.Fatal("disjoint path skipped validation", e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = DiscoverUniformOverlappingGraphPairTubes(canceled, a, b, geom.SurfacePatchOptions{}, UniformGraphDiscoveryOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e = DiscoverUniformOverlappingGraphPairTubes(ctx, a, b, geom.SurfacePatchOptions{MaxOperations: 1}, UniformGraphDiscoveryOptions{})
	if !errors.Is(e, geom.ErrConvergence) || r.Disjoint || len(r.Seeds.Attempts) != 0 {
		t.Fatal("work exhaustion", e)
	}
}
