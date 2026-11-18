package intersect

import (
	"context"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestAutomaticNonfactorizedQuarticGraphPair(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: .015625}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	// The genuinely nonfactorized quartic has exact lattice switch points.
	// The discovery API finds them; no cuts or semiaxes are supplied here.
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var p []geom.Vec3
	for i := range x {
		for j := range x {
			p = append(p, geom.Vec3{X: x[i], Y: x[j], Z: z[i] + z[j] + z[i]*z[j] - 409.0/4096 + x[i]*x[j]/64})
		}
	}
	knot := []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, len(x), len(x), p, knot, knot, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	o := SplinePairTraceOptions{}
	o.DyadicGraphFallback.Enabled = true
	o.DyadicGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Cell.MaxOperations = 16000000
	h, e := DiscoverDyadicGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, o.DyadicGraphFallback)
	if e != nil || len(h.Attempts) == 0 {
		t.Fatal(len(h.Attempts), e)
	}
	pruned := o.Branches
	pruned.PruneSeparatedEndpoints = true
	pruned.MaxComparisons = 32
	graph, e := JoinSplineTubeBranches(ctx, pa, pb, h, pruned)
	if e != nil || graph.Truncated || len(graph.Branches) != 1 || !graph.Branches[0].Closed {
		t.Fatal("pruned graph", graph, e)
	}
	legacy := pruned
	legacy.PruneSeparatedEndpoints = false
	limitedGraph, e := JoinSplineTubeBranches(ctx, pa, pb, h, legacy)
	if e != nil || !limitedGraph.Truncated {
		t.Fatal("comparison budget regression", limitedGraph, e)
	}
	r, e := CertifySplinePairBranches(ctx, pa, pb, h, o)
	if e != nil || !r.Complete || len(r.Curves) != 1 || !r.BranchGraph.Branches[0].Closed {
		t.Fatalf("issues=%v graph=%+v coverage=%v seeds=%d err=%v", r.Issues, r.BranchGraph, r.Coverage.Covered, len(h.Attempts), e)
	}

	r, e = TraceSplinePair(ctx, pa, pb, o)
	if e != nil || !r.Complete || len(r.Curves) != 1 || !r.BranchGraph.Branches[0].Closed {
		t.Fatal(r.Issues, e)
	}

	exactA, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	exactB, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if _, e := geom.DiscoverSplineGraphConcentricEllipses(ctx, exactA, exactB, geom.GraphPointContactOptions{}); e == nil {
		t.Fatal("fixture accidentally recognized as ellipse pair")
	}
	limited := o.DyadicGraphFallback
	limited.Tubes.MaxOperations = 1
	if _, e := DiscoverDyadicGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, limited); e == nil {
		t.Fatal("budget")
	}
	invalid := o.DyadicGraphFallback
	invalid.GridDivisions = 3
	if _, e := DiscoverDyadicGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, invalid); e == nil {
		t.Fatal("nondyadic grid")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := DiscoverDyadicGraphPairTubes(cancelled, pa, pb, geom.SurfacePatchOptions{}, o.DyadicGraphFallback); e == nil {
		t.Fatal("cancellation")
	}

}
