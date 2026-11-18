package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func disjointRegionMeshFixture(t *testing.T) AlgebraicTubeRegions {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	var net []geom.Vec3
	for i := range 5 {
		for j, z := range [3]float64{.25, -.25, .25} {
			u := 1.0 / 16
			if i%2 == 1 {
				u = -u
			}
			net = append(net, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 2, Z: u + z - 25.0/1024})
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 5, 3, net, []float64{0, 0, 0, .5, .5, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	v := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	loop := make([]Use, 4)
	for i, p := range v {
		loop[i] = Use{Edge: i, UVStart: p, UVEnd: v[(i+1)%4]}
	}
	p := [2]intersect.Patch{{Surface: a, Max: v[2]}, {Surface: b, Max: v[2]}}
	f := [2]Face{{Surface: a, Outer: loop}, {Surface: b, Outer: loop}}
	o := AlgebraicTrimmedPairSectionOptions{}
	o.Trace.GraphFallback.UCuts = []float64{.125, .15625, .34375, .375, .625, .65625, .84375, .875}
	o.Trace.GraphFallback.VCuts = []float64{.375, .40625, .59375, .625}
	o.Trace.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 32000000
	h, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, p[0], p[1], geom.SurfacePatchOptions{}, o.Trace.GraphFallback)
	if e != nil {
		t.Fatal(len(h.Attempts), e)
	}
	s, e := SectionTrimmedSplinePair(ctx, p, f, &h, o)
	if e != nil || !s.Complete {
		t.Fatalf("section complete=%v branches=%d issues=%v err=%v", s.Complete, len(s.Branches), s.Trace.Issues, e)
	}
	r, e := s.SealAlgebraicTubeRegions(ctx, AlgebraicTubeRegionOptions{})
	if e != nil || len(r.Regions()) != 2 {
		t.Fatalf("regions=%+v err=%v", r.Regions(), e)
	}
	return r
}

func TestCertifiedDisjointTubeRegionNetwork(t *testing.T) {
	slowtest.Skip(t)
	r := disjointRegionMeshFixture(t)
	o := AlgebraicTubeRegionsBoundaryOptions{Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .04}}}}
	n, e := r.TessellateBoundaryCertified(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(n.Loops) != 2 {
		t.Fatal("missing loops")
	}
	seen := map[int]int{}
	for loop, ids := range n.LoopVertices {
		for _, id := range ids {
			if old, ok := seen[id]; ok && old != loop {
				t.Fatal("cross loop weld")
			}
			seen[id] = loop
			p := n.Vertices[id]
			center := .25
			if p.X > .5 {
				center = .75
			}
			if math.Abs(math.Hypot(p.X-center, p.Y-.5)-5.0/32) > .003 {
				t.Fatal("independent circle", p)
			}
		}
	}
	for _, u := range n.Uses {
		if u.Hole {
			t.Fatal("unexpected hole")
		}
		found := false
		for _, v := range n.Uses {
			if v.Region == u.Region && v.Chart != u.Chart && v.Loop == u.Loop && v.Piece == u.Piece {
				found = true
				if u.Reversed == v.Reversed {
					t.Fatal("same incidence")
				}
				for j, id := range u.Vertices {
					if id != v.Vertices[len(v.Vertices)-1-j] {
						t.Fatal("owner identity")
					}
				}
			}
		}
		if !found {
			t.Fatal("missing owner")
		}
	}
	t.Log(len(n.Vertices), len(n.Uses), n.ExactOperations)
	for _, limit := range []AlgebraicTubeRegionsBoundaryOptions{{Loop: o.Loop, MaxLoops: 1}, {Loop: o.Loop, MaxVertices: 1}, {Loop: o.Loop, MaxOperations: 1}} {
		n, e := r.TessellateBoundaryCertified(context.Background(), limit)
		if e == nil || len(n.Vertices) > 0 {
			t.Fatal("partial network", e)
		}
	}
}
