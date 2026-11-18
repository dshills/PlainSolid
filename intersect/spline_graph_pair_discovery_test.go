package intersect

import (
	"context"
	"errors"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestAutomaticClosedCommonXYGraphPair(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: .015625}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	// The inner circle has dyadic axis-switch points .5±.1875 and
	// .5±.25. Knot lines at those coordinates give the trace its exact
	// junctions without numerical seeding or caller-proposed tube boxes.
	cuts := []float64{.25, .3125, .6875, .75}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var p []geom.Vec3
	for i := range x {
		for j := range x {
			p = append(p, geom.Vec3{X: x[i], Y: x[j], Z: z[i] + z[j] - 25.0/256 + x[i]*x[j]/64})
		}
	}
	knot := []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, len(x), len(x), p, knot, knot, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	o := SplinePairTraceOptions{}
	o.GraphFallback.Enabled = true
	o.GraphFallback.UCuts, o.GraphFallback.VCuts = cuts, cuts
	o.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Cell.MaxOperations = 16000000
	h, e := DiscoverCommonXYGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, o.GraphFallback)
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

	reversed, e := DiscoverCommonXYGraphPairTubes(ctx, pb, pa, geom.SurfacePatchOptions{}, o.GraphFallback)
	if e != nil {
		t.Fatal(e)
	}
	check, e := CertifySplinePairBranches(ctx, pb, pa, reversed, o)
	if e != nil || !check.Complete {
		t.Fatal(check.Issues, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = DiscoverCommonXYGraphPairTubes(canceled, pa, pb, geom.SurfacePatchOptions{}, o.GraphFallback); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	limited := o.GraphFallback
	limited.Tubes.MaxOperations = 1
	if _, e = DiscoverCommonXYGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, limited); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
	limited = o.GraphFallback
	limited.MaxTubes = 1
	if _, e = DiscoverCommonXYGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, limited); !errors.Is(e, ErrUnresolved) {
		t.Fatal(e)
	}

	incomplete := h
	incomplete.Attempts = append([]SplineSeedCell(nil), h.Attempts[1:]...)
	negativeOptions := o
	negativeOptions.Coverage.Cell.MaxCells = 16
	negativeOptions.Coverage.Cell.MaxDepth = 4
	check, e = CertifySplinePairBranches(ctx, pa, pb, incomplete, negativeOptions)
	if e == nil || check.Complete {
		t.Fatal("missing branch must not prove complete", check)
	}
	shifted := append([]geom.Vec3(nil), p...)
	for i := range shifted {
		shifted[i].X += .125
	}
	other, e := geom.NewBSplineSurface(2, 2, 3, 3, shifted, knot, knot, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = DiscoverCommonXYGraphPairTubes(ctx, pa, Patch{Surface: other, Max: pb.Max}, geom.SurfacePatchOptions{}, o.GraphFallback); !errors.Is(e, ErrUnresolved) {
		t.Fatal("different XY chart", e)
	}

}
