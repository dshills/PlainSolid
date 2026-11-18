package intersect

import (
	"context"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestAutomaticClosedKnotGraphPair(t *testing.T) {
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
	a, e = a.InsertUKnot(.5, 1)
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.InsertUKnot(.5, 2)
	if e != nil {
		t.Fatal(e)
	}
	bend := func(source *geom.BSplineSurface) *geom.BSplineSurface {
		d := source.Data()
		for i := range d.Poles {
			x := d.Poles[i].X - .5
			if x < 0 {
				x = -x
			}
			d.Poles[i].Z += x / 8
		}
		out, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	a, b = bend(a), bend(b)
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
	r, e := CertifySplinePairBranches(ctx, pa, pb, h, o)
	if e != nil || !r.Complete || len(r.Curves) != 1 || !r.BranchGraph.Branches[0].Closed {
		t.Fatalf("issues=%v graph=%+v coverage=%v seeds=%d err=%v", r.Issues, r.BranchGraph, r.Coverage.Covered, len(h.Attempts), e)
	}

	r, e = TraceSplinePair(ctx, pa, pb, o)
	if e != nil || !r.Complete || len(r.Curves) != 1 || !r.BranchGraph.Branches[0].Closed {
		t.Fatal(r.Issues, e)
	}

}

func TestAutomaticTrueC0GraphBranch(t *testing.T) {
	ctx := context.Background()
	ku := []float64{0, 0, .5, 1, 1}
	kv := []float64{0, 0, 1, 1}
	var ap, bp []geom.Vec3
	for i, u := range []float64{0, .5, 1} {
		f := []float64{.25, .5, .625}[i]
		for _, v := range []float64{0, 1} {
			ap = append(ap, geom.Vec3{X: u, Y: v})
			bp = append(bp, geom.Vec3{X: u, Y: v, Z: v - f})
		}
	}
	a, e := geom.NewBSplineSurface(1, 1, 3, 2, ap, ku, kv, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewBSplineSurface(1, 1, 3, 2, bp, ku, kv, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	o := SplinePairTraceOptions{}
	o.GraphFallback.Enabled = true
	o.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Cell.MaxOperations = 16000000
	seeds, e := DiscoverCommonXYGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, o.GraphFallback)
	if e != nil {
		t.Fatal(len(seeds.Attempts), e)
	}
	anchored := 0
	for _, s := range seeds.Attempts {
		if s.BoundaryAnchor != nil {
			anchored++
		}
	}
	if anchored < 2 {
		t.Fatal("missing knot anchors", anchored)
	}
	report, e := CertifySplinePairBranches(ctx, pa, pb, seeds, o)
	if e != nil || !report.Complete || !report.Coverage.CoversAllContacts || len(report.BranchGraph.Branches) != 1 {
		t.Fatal(report.Issues, report.BranchGraph, e)
	}
}
