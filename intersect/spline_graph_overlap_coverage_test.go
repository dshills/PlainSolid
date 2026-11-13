package intersect

import (
	"context"
	"testing"

	"plainsolid/geom"
)

func TestOriginalDomainOverlapCoverage(t *testing.T) {
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

	h, e := DiscoverOverlappingAffineXYGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, o.AffineGraphFallback)
	if e != nil {
		t.Fatal(e)
	}
	o.Coverage.OverlappingAffineXYFallback = true
	r, e := CertifySplinePairBranches(ctx, pa, pb, h.Seeds, o)
	if e != nil || !r.Complete {
		t.Fatal(r.Issues, e)
	}
	missing := h.Seeds
	missing.Attempts = append([]SplineSeedCell(nil), h.Seeds.Attempts[1:]...)
	o.Coverage.MaxCandidateTests = 32
	o.Coverage.Cell.MaxCells = 32
	o.Coverage.Cell.MaxDepth = 4
	bad, e := CertifySplinePairBranches(ctx, pa, pb, missing, o)
	if e == nil || bad.Complete {
		t.Fatal("missing tube accepted")
	}
}
