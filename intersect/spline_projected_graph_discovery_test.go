package intersect

import (
	"context"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestAutomaticClosedProjectedGraphPair(t *testing.T) {
	slowtest.Skip(t)
	testProjectedGraphPair(t, [3]int{0, 2, 1}, [2]int{0, 2})
}
func TestAutomaticClosedYZGraphPair(t *testing.T) {
	slowtest.Skip(t)
	testProjectedGraphPair(t, [3]int{2, 0, 1}, [2]int{1, 2})
}
func testProjectedGraphPair(t *testing.T, permutation [3]int, axes [2]int) {
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
	// Reauthor the same B model with U=2+2*v, V=3-4*u.
	swapped := make([]geom.Vec3, len(p))
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			swapped[3*i+j] = p[3*(2-j)+i]
		}
	}
	b, e = geom.NewBSplineSurface(2, 2, 3, 3, swapped, []float64{2, 2, 2, 4, 4, 4}, []float64{-1, -1, -1, 3, 3, 3}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, e = a.PermutedModelAxes(permutation)
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.PermutedModelAxes(permutation)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Min: geom.Vec2{X: 2, Y: -1}, Max: geom.Vec2{X: 4, Y: 3}}
	o := SplinePairTraceOptions{GraphProjectionAxes: axes}
	o.ProjectedGraphFallback.Enabled = true
	o.ProjectedGraphFallback.UCuts, o.ProjectedGraphFallback.VCuts = cuts, cuts
	o.ProjectedGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Isolation.ParameterTolerance = .125
	o.Coverage.Cell.MaxOperations = 16000000
	o.Coverage.ProjectedAffineFallback = true
	o.Coverage.ProjectionAxes = axes
	h, e := DiscoverProjectedAffineGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, axes, o.ProjectedGraphFallback)
	if e != nil || len(h.Attempts) == 0 {
		t.Fatal(len(h.Attempts), e)
	}
	r, e := CertifySplinePairBranches(ctx, pa, pb, h, o)
	if e != nil || !r.Complete || len(r.BranchGraph.Branches) != 1 || !r.BranchGraph.Branches[0].Closed {
		t.Fatalf("issues=%v graph=%+v coverage=%v seeds=%d err=%v", r.Issues, r.BranchGraph, r.Coverage.Covered, len(h.Attempts), e)
	}

	r, e = TraceSplinePair(ctx, pa, pb, o)
	if e != nil || !r.Complete {
		t.Fatal(r.Issues, e)
	}

}
