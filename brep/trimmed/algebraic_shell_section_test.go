package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestProjectedAlgebraicShellTerminal(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
	var poles []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			y := 0.0
			if i == 2 {
				y++
			}
			if j == 2 {
				y++
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: y, Z: float64(j) / 2})
		}
	}
	a, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	k = []float64{0, 0, 1, 1}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5}, {Y: .5, Z: 1}, {X: 1, Y: .5}, {X: 1, Y: .5, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	d := [2]geom.ParameterRectangle{{UMin: 7.0 / 32, UMax: .25, VMin: 5.0 / 8, VMax: 11.0 / 16}, {UMin: 3.0 / 16, UMax: 9.0 / 32, VMin: 19.0 / 32, VMax: 23.0 / 32}}
	var p [2]*geom.SurfacePatch
	var chart [2]intersect.Patch
	for i, s := range []*geom.BSplineSurface{a, b} {
		q, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p[i], e = q.Trim(d[i])
		if e != nil {
			t.Fatal(e)
		}
		chart[i] = intersect.Patch{Surface: s, Min: geom.Vec2{X: d[i].UMin, Y: d[i].VMin}, Max: geom.Vec2{X: d[i].UMax, Y: d[i].VMax}}
	}
	proof, e := geom.CertifySurfacePairTube(p[0], p[1], geom.SurfacePairTubeOptions{})
	if e != nil || !proof.ProvenBranch {
		t.Fatal(proof, e)
	}
	h := intersect.SplineSeedReport{Attempts: []intersect.SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: d[0], DomainB: d[1]}, Tube: &proof}}}
	x, e := SectionSplinePairAlgebraic(ctx, chart[0], chart[1], &h, intersect.SplinePairTraceOptions{}, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !x.Complete || len(x.Open) != 1 {
		t.Fatal(x.Trace.Issues, e)
	}
	sides := map[geom.SurfaceEdge]bool{}
	for _, z := range x.Open[0].Ends {
		r, e := z.Certificate.Parameters[1].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || r.Kind != geom.ExactRootKindIrrational || z.Operand != 0 || z.Side > geom.SurfaceEdgeUMax || sides[z.Side] {
			t.Fatal(z.Side, r, e)
		}
		sides[z.Side] = true
		p, _, e := z.Certificate.ModelEvent.CoordinateRoot(ctx, 1, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		y, e := p.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || y.Kind != geom.ExactRootKindRational || y.Rational.Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal(y, e)
		}
	}
	var f [2]Face
	for i, s := range []*geom.BSplineSurface{a, b} {
		p := [4]geom.Vec2{{X: 15.0 / 64, Y: d[i].VMin}, {X: 63.0 / 256, Y: d[i].VMin}, {X: 63.0 / 256, Y: d[i].VMax}, {X: 15.0 / 64, Y: d[i].VMax}}
		f[i].Surface = s
		for j, v := range p {
			f[i].Outer = append(f[i].Outer, Use{Edge: j, UVStart: v, UVEnd: p[(j+1)%4]})
		}
	}
	o := AlgebraicTrimmedPairSectionOptions{}
	axes := [2]int{0, 2}
	o.Clip.Projection = &axes
	section, e := SectionTrimmedSplinePair(ctx, chart, f, &h, o)
	if e != nil || !section.Complete || len(section.Branches) != 1 || len(section.Branches[0].Runs) != 1 {
		t.Fatalf("projected trim section %v branches=%d err=%v", section.Complete, len(section.Branches), e)
	}
	c, e := section.PartitionBothFacesByTubeRun(ctx, 0, 0, AlgebraicTubeFacePartitionOptions{})
	if e != nil || len(c.Faces[0].Cut) != 1 || len(c.Faces[1].Cut) != 1 {
		t.Fatalf("projected partition err=%v", e)
	}
	var file bytes.Buffer
	if e := WriteTrimmedSplinePairSectionJSON(&file, section); e != nil {
		t.Fatal(e)
	}
	back, e := ReadTrimmedSplinePairSectionJSON(ctx, bytes.NewReader(file.Bytes()), JSONLimits{}, AlgebraicTrimmedPairSectionOptions{})
	if e != nil || !back.Complete || back.Branches[0].Clips[0].FaceTrims[0].ProjectionAxes != axes {
		t.Fatalf("projected native reload %v err=%v", back.Complete, e)
	}
}

func TestAlgebraicShellSectionClosedWire(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, _ := transverseSplineCaps(t)
	o := AlgebraicShellSectionOptions{}
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.UCuts = []float64{.25, .3125, .6875, .75}
	o.Trace.PlaneFallback.VCuts = o.Trace.PlaneFallback.UCuts
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !r.Complete || len(r.Branches) != 1 || len(r.Wires) != 1 || len(r.Wires[0]) != 1 ||
		!r.Branches[0].Closed || r.Branches[0].Faces != [2]int{0, 1} {
		t.Fatalf("complete=%v pairs=%v branches=%d wires=%v err=%v", r.Complete, r.Pairs, len(r.Branches), r.Wires, e)
	}
	parts, e := r.ImprintOriginalShells(ctx, AlgebraicShellImprintOptions{})
	if e != nil || parts[0].Topology.Faces != 7 || parts[1].Topology.Faces != 7 || parts[0].Topology.InnerWires != 1 || parts[1].Topology.InnerWires != 1 {
		t.Fatal("interior loop imprint", parts[0].Topology, parts[1].Topology, e)
	}
}

func TestJoinAlgebraicShellSectionBranches(t *testing.T) {
	ctx := context.Background()
	curve, e := geom.NewExactSplineCurve(mustBezierForAlgebraicShell(t), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, e := geom.NewExactComposedBezierCell(ctx, curve, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Real immutable curve events with one irrational endpoint model
	// coordinate. Only topology owners are synthetic in this join test.
	left, e := c.IntersectRawPlane(ctx, [4]*big.Rat{nil, big.NewRat(1, 1), nil, big.NewRat(-1, 4)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(left.Events) != 1 {
		t.Fatal(left, e)
	}
	right, e := c.IntersectRawPlane(ctx, [4]*big.Rat{nil, big.NewRat(1, 1), nil, big.NewRat(-1, 2)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(right.Events) != 1 {
		t.Fatal(right, e)
	}
	terminal := func(side geom.SurfaceEdge, event *geom.ExactComposedCurveEvent) AlgebraicSplineTerminal {
		return AlgebraicSplineTerminal{Operand: 0, Side: side, Certificate: geom.TubeAlgebraicGraphEndpointCertificate{Proven: true, ModelEvent: event}}
	}
	branches := []AlgebraicShellSectionBranch{
		{Faces: [2]int{0, 0}, Ends: [2]AlgebraicSplineTerminal{terminal(geom.SurfaceEdgeUMin, left.Events[0]), terminal(geom.SurfaceEdgeUMax, right.Events[0])}},
		{Faces: [2]int{1, 0}, Ends: [2]AlgebraicSplineTerminal{terminal(geom.SurfaceEdgeUMax, right.Events[0]), terminal(geom.SurfaceEdgeUMin, left.Events[0])}},
	}
	faces := [2][]splineEmbeddingFace{{{edge: [4]int{4, 5, 6, 7}}, {edge: [4]int{4, 5, 8, 9}}}, {{}}}
	r, e := joinAlgebraicShellSectionBranches(ctx, branches, faces, AlgebraicShellSectionOptions{MaxComparisons: 10})
	if e != nil || len(r) != 1 || len(r[0]) != 2 {
		t.Fatal(r, e)
	}
	branches[1].Ends[0].Certificate.ModelEvent = left.Events[0]
	if _, e := joinAlgebraicShellSectionBranches(ctx, branches, faces, AlgebraicShellSectionOptions{MaxComparisons: 10}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
}

func mustBezierForAlgebraicShell(t *testing.T) *geom.BSpline {
	t.Helper()
	c, e := geom.NewBezier([]geom.Vec3{{}, {X: .5}, {X: 1, Y: 1}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
