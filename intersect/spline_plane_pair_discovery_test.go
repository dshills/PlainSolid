package intersect

import (
	"context"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestDiscoverSplinePlanePairTubes(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	p := func(s *geom.BSplineSurface, a, b geom.Vec2) Patch { return Patch{s, a, b} }
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: -.375}, {Y: 1, Z: -.375}, {X: 1, Z: .625}, {X: 1, Y: 1, Z: .625}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	l := []float64{-.25, -.25, 1.25, 1.25}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: -.25, Y: -.25}, {X: -.25, Y: 1.25}, {X: 1.25, Y: -.25}, {X: 1.25, Y: 1.25}}, l, l, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, y := p(a, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}), p(b, geom.Vec2{X: -.25, Y: -.25}, geom.Vec2{X: 1.25, Y: 1.25})
	for i, pair := range [2][2]Patch{{x, y}, {y, x}} {
		h, e := DiscoverSplinePlanePairTubes(ctx, pair[0], pair[1], geom.SurfacePatchOptions{}, SplinePlanePairDiscoveryOptions{})
		if e != nil || len(h.Attempts) == 0 || !h.Attempts[0].Tube.ProvenBranch || h.Attempts[0].Tube.FreeAxis != 1+2*i {
			t.Fatal(h, e)
		}
		r, e := CertifySplinePairBranches(ctx, pair[0], pair[1], h, SplinePairTraceOptions{})
		if e != nil || !r.Complete || len(r.Curves) != 1 {
			t.Fatal(r, e)
		}
	}
}

func TestAutomaticClosedSplinePlanePair(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
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
			p = append(p, geom.Vec3{X: x[i], Y: x[j], Z: z[i] + z[j] - 25.0/256})
		}
	}
	knot := []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, len(x), len(x), p, knot, knot, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	o := SplinePairTraceOptions{}
	o.PlaneFallback.Enabled = true
	o.PlaneFallback.UCuts, o.PlaneFallback.VCuts = cuts, cuts
	o.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Cell.MaxOperations = 16000000
	h, e := DiscoverSplinePlanePairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, o.PlaneFallback)
	if e != nil || len(h.Attempts) == 0 {
		t.Fatal(len(h.Attempts), e)
	}
	r, e := CertifySplinePairBranches(ctx, pa, pb, h, o)
	if e != nil || !r.Complete || len(r.Curves) != 1 || !r.BranchGraph.Branches[0].Closed {
		t.Fatal(r.Issues, len(r.Curves), e)
	}
	r, e = TraceSplinePair(ctx, pa, pb, o)
	if e != nil || !r.Complete || len(r.Curves) != 1 || !r.BranchGraph.Branches[0].Closed {
		t.Fatal(r.Issues, len(r.Curves), e)
	}
}

func TestDiscoverObliqueSplinePlanePairTubes(t *testing.T) {
	ctx := context.Background()
	knot := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: -.375}, {Y: 1, Z: -.375}, {X: 1, Z: 1.625}, {X: 1, Y: 1, Z: 1.625}}, knot, knot, nil)
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{-.25, -.25, 1.25, 1.25}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: -.25, Y: -.25, Z: -.25}, {X: -.25, Y: 1.25, Z: -.25}, {X: 1.25, Y: -.25, Z: 1.25}, {X: 1.25, Y: 1.25, Z: 1.25}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Min: geom.Vec2{X: -.25, Y: -.25}, Max: geom.Vec2{X: 1.25, Y: 1.25}}
	for _, pair := range [][2]Patch{{pa, pb}, {pb, pa}} {
		r, e := DiscoverSplinePlanePairTubes(ctx, pair[0], pair[1], geom.SurfacePatchOptions{}, SplinePlanePairDiscoveryOptions{})
		if e != nil || len(r.Attempts) == 0 {
			t.Fatal(r, e)
		}
		trace, e := CertifySplinePairBranches(ctx, pair[0], pair[1], r, SplinePairTraceOptions{})
		if e != nil || !trace.Complete || len(trace.Curves) != 1 {
			t.Fatal(trace, e)
		}
	}
}

func TestDiscoverRawObliqueCurvedPlanePair(t *testing.T) {
	var poles []geom.Vec3
	for _, u := range []float64{0, 1} {
		for j, z := range []float64{-.375, -1.4375, -2.375} {
			poles = append(poles, geom.Vec3{X: 5 * u, Y: 2.5 * float64(j), Z: z})
		}
	}
	s, e := geom.NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	poles = nil
	for _, u := range []float64{-.25, 1.25} {
		for _, v := range []float64{-.25, 1.25} {
			poles = append(poles, geom.Vec3{X: 5 * u, Y: 5 * v, Z: -u - 2*v})
		}
	}
	k := []float64{-.25, -.25, 1.25, 1.25}
	p, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, b := Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: p, Min: geom.Vec2{X: -.25, Y: -.25}, Max: geom.Vec2{X: 1.25, Y: 1.25}}
	options := SplinePlanePairDiscoveryOptions{Tubes: geom.SurfaceArcTubeDiscoveryOptions{Padding: .25}}
	for _, pair := range [][2]Patch{{a, b}, {b, a}} {
		result, e := DiscoverSplinePlanePairSections(context.Background(), pair[0], pair[1], geom.SurfacePatchOptions{}, options)
		if e != nil || result.OpenContours != 1 || len(result.Terminals) != 2 || len(result.Seeds.Attempts) == 0 {
			t.Fatal(result, e)
		}
		for _, terminal := range result.Terminals {
			if !terminal.Certificate.Proven {
				t.Fatal(terminal)
			}
		}
		proof, e := CertifySplinePairBranches(context.Background(), pair[0], pair[1], result.Seeds, SplinePairTraceOptions{})
		if e != nil || !proof.Complete || len(proof.Curves) != 1 {
			t.Fatal(proof, e)
		}
	}
}
