package intersect

import (
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestOverlappingAffineGraphDiscovery(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var poles []geom.Vec3
	for i := range x {
		for j := range x {
			poles = append(poles, geom.Vec3{X: x[i] + .125, Y: x[j], Z: z[i] + z[j] - 25.0/256})
		}
	}
	knot := []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knot, knot, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	o := SplinePairTraceOptions{}
	o.AffineGraphFallback.Enabled = true
	o.AffineGraphFallback.UCuts = []float64{.375, .4375, .8125, .875}
	o.AffineGraphFallback.VCuts = []float64{.25, .3125, .6875, .75}
	o.AffineGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Coverage.AffineXYFallback = true
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Cell.MaxOperations = 16000000
	r, e := DiscoverOverlappingAffineXYGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, o.AffineGraphFallback)
	if e != nil || r.Disjoint || len(r.Seeds.Attempts) == 0 {
		t.Fatal(e, r.Disjoint, len(r.Seeds.Attempts))
	}
	if r.A.Min.X != .125 || r.A.Max.X != 1 || r.B.Min.X != 0 || r.B.Max.X != .875 {
		t.Fatal(r.A, r.B)
	}
	proof, e := CertifySplinePairBranches(ctx, r.A, r.B, r.Seeds, o)
	if e != nil || !proof.Complete || len(proof.Curves) != 1 || !proof.BranchGraph.Branches[0].Closed {
		t.Fatal(e, proof.Issues)
	}
	limited := geom.SurfacePatchOptions{MaxOperations: 1}
	bad, e := DiscoverOverlappingAffineXYGraphPairTubes(ctx, pa, pb, limited, o.AffineGraphFallback)
	if e == nil || len(bad.Seeds.Attempts) > 0 {
		t.Fatal("budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = DiscoverOverlappingAffineXYGraphPairTubes(canceled, pa, pb, geom.SurfacePatchOptions{}, o.AffineGraphFallback)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
