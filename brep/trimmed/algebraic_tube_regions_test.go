package trimmed

import (
	"context"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicTubeConcentricContainment(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	support, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var cut [2][]AlgebraicTubeFaceCutPiece
	for i, radius := range []float64{5.0 / 16, 5.0 / 32} {
		z := [3]float64{.25, -.25, .25}
		var poles []geom.Vec3
		for u := range z {
			for v := range z {
				poles = append(poles, geom.Vec3{X: float64(u) / 2, Y: float64(v) / 2, Z: z[u] + z[v] - radius*radius})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		b, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		v := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
		var outer []Use
		for j, x := range v {
			outer = append(outer, Use{Edge: j, UVStart: x, UVEnd: v[(j+1)%4]})
		}
		p := [2]intersect.Patch{{Surface: a, Max: v[2]}, {Surface: b, Max: v[2]}}
		f := [2]Face{{Surface: a, Outer: outer}, {Surface: b, Outer: outer}}
		o := AlgebraicTrimmedPairSectionOptions{}
		d := []float64{.5 - 4*radius/5, .5 - 3*radius/5, .5 + 3*radius/5, .5 + 4*radius/5}
		o.Trace.GraphFallback.UCuts, o.Trace.GraphFallback.VCuts = d, d
		o.Trace.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
		o.Trace.Coverage.Isolation.SeparatingProjections = true
		o.Trace.Coverage.Isolation.ParameterTolerance = .125
		o.Trace.Coverage.Cell.MaxOperations = 16000000
		h, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, p[0], p[1], geom.SurfacePatchOptions{}, o.Trace.GraphFallback)
		if e != nil {
			t.Fatal(e)
		}
		s, e := SectionTrimmedSplinePair(ctx, p, f, &h, o)
		if e != nil || !s.Complete {
			t.Fatal(s.Trace.Issues, e)
		}
		q, e := s.PartitionFaceByTubeRun(ctx, 0, 0, 0, AlgebraicTubeFacePartitionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cut[i] = q.Cut
	}
	o := geom.ExactCurvePlaneEventOptions{MaxOperations: 8000000, MaxCoefficientBits: 16384}
	inside, _, e := tubeLoopContainment(ctx, support, cut[0], cut[1], o)
	if e != nil || !inside {
		t.Fatal(inside, e)
	}
	outside, _, e := tubeLoopContainment(ctx, support, cut[1], cut[0], o)
	if e != nil || outside {
		t.Fatal(outside, e)
	}
	loops := []AlgebraicTubeRegionLoop{{Parent: -1, InteriorHeightSign: 1}, {Parent: -1, InteriorHeightSign: -1}}
	regions, e := assembleTubeRegionHierarchy(loops, [][]bool{{false, inside}, {outside, false}})
	if e != nil || len(regions) != 2 || len(regions[0].Holes) != 1 || regions[0].Holes[0] != 1 ||
		loops[1].Parent != 0 || regions[0].UpperChart != 0 || regions[1].UpperChart != 1 {
		t.Fatal(loops, regions, e)
	}
}
