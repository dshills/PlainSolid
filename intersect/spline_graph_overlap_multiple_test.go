package intersect

import (
	"context"
	"testing"

	"plainsolid/geom"
)

func TestOverlapTwoClosedContours(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	xs := []float64{.0625, .28125, .5, .71875, .9375}
	ys := []float64{.25, .5, .75}
	var poles []geom.Vec3
	for i, x := range xs {
		xx := .21875 * .21875
		if i%2 == 1 {
			xx = -xx
		}
		for j, y := range ys {
			yy := .0625
			if j == 1 {
				yy = -yy
			}
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: 25.0/1024 - xx - yy})
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 5, 3, poles, []float64{.0625, .0625, .0625, .5, .5, .9375, .9375, .9375}, []float64{.25, .25, .25, .75, .75, .75}, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa := Patch{Surface: a, Min: geom.Vec2{X: .09375}, Max: geom.Vec2{X: .90625, Y: 1}}
	pb := Patch{Surface: b, Min: geom.Vec2{X: .0625, Y: .25}, Max: geom.Vec2{X: .9375, Y: .75}}
	o := OverlappingGraphTraceOptions{}
	o.Discovery.UCuts = []float64{0, .0625, .09375, .90625, .9375, 1, .375, .15625, .1875, .375, .40625, .59375, .625, .8125, .84375}
	o.Discovery.VCuts = []float64{0, .25, .75, 1, .375, .40625, .59375, .625}
	o.Discovery.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	r, e := CertifyOverlappingAffineXYGraphPair(context.Background(), pa, pb, o)
	if e != nil || !r.Complete || len(r.Trace.Curves) != 2 {
		t.Fatal(e, r.Complete, len(r.Trace.Curves))
	}
	for _, branch := range r.Trace.BranchGraph.Branches {
		if !branch.Closed {
			t.Fatal("open branch")
		}
	}
}
