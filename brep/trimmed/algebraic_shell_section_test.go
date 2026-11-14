package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicShellSectionSeparated(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, _ := makeOpenSplineBox(t, [3]float64{3, 3, 3}, [3]float64{4, 4, 4}, 0)
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{})
	if e != nil || !r.Complete || len(r.Wires) != 0 || len(r.Pairs) != 0 {
		t.Fatal(r, e)
	}
	common, e := r.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if b, empty, e := common.Bounds(); e != nil || !empty || b != (geom.Box{}) {
		t.Fatal("a disjoint result must report proved empty bounds", b, empty, e)
	}
	if _, e := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{MaxFacePairs: 1}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
}

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

func TestAlgebraicShellSectionAcrossOriginalEdges(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, _ := makeOpenSplineBox(t, [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}, .125)
	o := AlgebraicShellSectionOptions{}
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !r.Complete || len(r.Branches) != 6 || len(r.Wires) != 1 || len(r.Wires[0]) != 6 {
		t.Fatalf("complete=%v pairs=%v branches=%d wires=%v err=%v", r.Complete, r.Pairs, len(r.Branches), r.Wires, e)
	}
	for _, q := range r.Branches {
		if q.Closed || q.EdgeParameters[0] == nil || q.EdgeParameters[1] == nil ||
			q.Ends[0].Certificate.ModelEvent == nil || q.Ends[1].Certificate.ModelEvent == nil {
			t.Fatal(q)
		}
	}
	var native bytes.Buffer
	if e := WriteAlgebraicShellSectionJSON(&native, r); e != nil {
		t.Fatal(e)
	}
	back, e := ReadAlgebraicShellSectionJSON(ctx, bytes.NewReader(native.Bytes()), JSONLimits{}, o)
	if e != nil || !back.Complete || len(back.Wires) != 1 || len(back.Wires[0]) != 6 {
		t.Fatalf("whole shell reload %+v err=%v", back.Wires, e)
	}
	sub, e := back.SubdivideOriginalEdges(ctx, AlgebraicSeamOptions{})
	if e != nil || sub[0].Topology().Vertices != 11 || sub[1].Topology().Vertices != 11 ||
		sub[0].Topology().EulerCharacteristic != 2 || sub[1].Topology().EulerCharacteristic != 2 {
		t.Fatal("warped shell subdivisions", sub[0].Topology(), sub[1].Topology(), e)
	}
	var projectedRecipe *AlgebraicSeamRecipe
	for _, d := range sub {
		var f bytes.Buffer
		if e := WriteAlgebraicSubdivisionJSON(&f, d); e != nil {
			t.Fatal(e)
		}
		q, e := ReadAlgebraicSubdivisionJSON(ctx, &f, JSONLimits{}, AlgebraicSeamOptions{})
		if e != nil || q.Topology() != d.Topology() {
			t.Fatal("warped shell subdivision reload", e)
		}
		for _, r := range q.Recipes() {
			if r.Version == 4 {
				projectedRecipe = &r
				wrong := r
				bad := [2]int{0, 0}
				wrong.Projection = &bad
				if _, e := q.Source().VerifyAlgebraicSeamRecipe(ctx, wrong, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrInvalid) {
					t.Fatal("accepted malformed projection", e)
				}
			}
		}
	}
	if projectedRecipe == nil {
		t.Fatal("missing bounded curved graph constraint")
	}
	// Reassembled intersection is the original box overlap under
	// z'=z+xy/8, a determinant-one warp. The exact volume is 1/8.
	s, h, e := back.CombineRepresentable(ctx, parametric.Intersection, SplineShellOpenOptions{Trace: o.Trace})
	if e != nil || s.Topology().EulerCharacteristic != 2 || len(h.SectionEdges) == 0 {
		t.Fatalf("reassembled cross-shell topology %+v err=%v", s.Topology(), e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-5})
	if e != nil || p.Volume < .1249999 || p.Volume > .1250001 {
		t.Fatal(p.Volume, e)
	}
	indexed, e := back.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal("exact algebraic source boundary", e)
	}
	indexed.Edges[0].Start = -1 // Mutable diagnostics cannot alter native reconstruction.
	indexed.Faces[0].Outer[0].Edge = -1
	if bad, e := indexed.NativeRepresentable(ctx, 1e-8, NativeAlgebraicBooleanOptions{MaxEdges: 1}); !errors.Is(e, geom.ErrConvergence) || len(bad.faces) != 0 {
		t.Fatal("native conversion work cap leaked partial topology", e)
	}
	converted, e := indexed.NativeRepresentable(ctx, 1e-8, NativeAlgebraicBooleanOptions{})
	if e != nil || converted.Topology().EulerCharacteristic != 2 || len(converted.faces) != len(indexed.faces) || len(converted.edges) != len(indexed.index) {
		t.Fatal("reconstructed closed native source", converted.Topology(), e)
	}
	for i, f := range converted.faces {
		original, e := indexed.OriginalFace(i)
		if e != nil || f.Surface != original.Surface {
			t.Fatal("lost original face chart", i, e)
		}
	}
	var nativeBoundary bytes.Buffer
	if e := WriteJSON(&nativeBoundary, converted); e != nil {
		t.Fatal("ordinary native boundary persistence", e)
	}
	reloaded, e := ReadJSONContext(ctx, &nativeBoundary, JSONLimits{})
	if e != nil || reloaded.Topology().EulerCharacteristic != 2 {
		t.Fatal("ordinary native reload", reloaded.Topology(), e)
	}
	if mass, e := reloaded.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-5}); e != nil ||
		mass.Volume < .1249999 || mass.Volume > .1250001 {
		t.Fatal("native reload changed independent mass", mass.Volume, e)
	}
	var malformed nativeAlgebraicShellSection
	if e := json.Unmarshal(native.Bytes(), &malformed); e != nil {
		t.Fatal(e)
	}
	malformed.Pairs[0].Faces[0] = (malformed.Pairs[0].Faces[0] + 1) % 6
	changed, e := json.Marshal(malformed)
	if e != nil {
		t.Fatal(e)
	}
	if q, e := ReadAlgebraicShellSectionJSON(ctx, bytes.NewReader(changed), JSONLimits{}, o); e == nil || q.Complete {
		t.Fatal("misowned face pair accepted", e)
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

func TestAlgebraicShellSectionIrrationalOriginalEdges(t *testing.T) {
	ctx := context.Background()
	a, af := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1.25, 1}, 0)
	b, bp := irrationalRoofSplineShell(t)
	pa, e := geom.NewSurfacePatch(af[4].surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := AlgebraicShellSectionOptions{}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	for _, face := range []int{1, 2, 3, 5} {
		q, e := geom.NewSurfacePatch(bp[face], geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s := AlgebraicShellSectionSeed{Faces: [2]int{4, face}}
		makeCell := func(axis int, da, db geom.ParameterRectangle) bool {
			x, e := pa.Trim(da)
			if e != nil {
				t.Fatal("source tube", face, axis, da, e)
			}
			y, e := q.Trim(db)
			if e != nil {
				t.Fatal("target tube", face, axis, db, e)
			}
			p, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: axis})
			if e != nil {
				t.Fatal(e)
			}
			if p.ProvenBranch {
				s.Tubes.Attempts = append(s.Tubes.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &p})
			}
			return p.ProvenBranch
		}
		// B's roof is Z=Y²-1/2. Its two X walls interpolate the roof
		// from Z=-7/16. The guides below are only tube proposals; the
		// whole-shell call independently checks every original pair.
		if face == 1 || face == 5 {
			dy := [2]float64{.68, .74}
			d := [2]float64{.68, .74}
			if face == 5 {
				dy = [2]float64{.98, 1.02}
				d = [2]float64{.4, .55}
			}
			if !makeCell(2, shellSectionRect(.2, .8, dy[0], dy[1]), shellSectionRect(.25, .75, d[0], d[1])) {
				t.Fatal("no roof/end-wall tube", face)
			}
		} else {
			x := .25 + .5*float64(face-2)
			var add func(int, float64, float64, int)
			add = func(axis int, lo, hi float64, depth int) {
				f := func(y float64) float64 { return (7.0 / 16) / (y*y - 1.0/16) }
				inv := func(v float64) float64 { return math.Sqrt(1.0/16 + (7.0/16)/v) }
				yl, yh, vl, vh := lo, hi, f(hi), f(lo)
				if axis == 3 {
					yl, yh, vl, vh = inv(hi), inv(lo), lo, hi
				}
				p := min((hi-lo)/2, .015625)
				dy, dv := [2]float64{yl, yh}, [2]float64{vl, vh}
				if axis == 2 {
					dv = [2]float64{vl - p, vh + p}
				} else {
					dy = [2]float64{yl - p, yh + p}
				}
				if makeCell(axis, shellSectionRect(x-.03, x+.03, yl-p, yh+p), shellSectionRect(dy[0], dy[1], dv[0], dv[1])) {
					return
				}
				if depth == 7 {
					t.Fatal("no side-wall tube", face, axis, lo, hi)
				}
				mid := (lo + hi) / 2
				add(axis, lo, mid, depth+1)
				add(axis, mid, hi, depth+1)
			}
			add(3, .875, 1, 0)
			add(2, .75, 1, 0)
		}
		o.Seeds = append(o.Seeds, s)
	}
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !r.Complete || len(r.Branches) != 4 || len(r.Wires) != 1 || len(r.Wires[0]) != 4 {
		for i, s := range r.Sections {
			t.Log(r.Pairs[i], s.Trace.Issues, len(s.Trace.Curves), len(s.Open), len(s.Trace.Coverage.Unresolved))
		}
		t.Fatalf("complete=%v pairs=%v branches=%d wires=%v err=%v", r.Complete, r.Pairs, len(r.Branches), r.Wires, e)
	}
	counts := map[int]int{}
	for _, branch := range r.Branches {
		for j, root := range branch.EdgeParameters {
			class, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if class.Kind != geom.ExactRootKindIrrational {
				continue
			}
			counts[branch.EndEdges[j]]++
			p := branch.Ends[j].Certificate.ModelEvent
			y, _, e := p.CoordinateRoot(ctx, 1, geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			yBounds, e := y.Enclose(ctx, big.NewRat(1, 10000), geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			z, _, e := p.CoordinateRoot(ctx, 2, geom.ExactCurvePlaneEventOptions{})
			if e != nil || z.Interval().Lower.Sign() != 0 || z.Interval().Upper.Sign() != 0 ||
				yBounds.Interval.Lower.Cmp(big.NewRat(7, 10)) <= 0 || yBounds.Interval.Upper.Cmp(big.NewRat(71, 100)) >= 0 {
				t.Fatal("wrong physical endpoint", yBounds.Interval, z, e)
			}
		}
	}
	if len(counts) != 2 {
		t.Fatal(counts)
	}
	for _, n := range counts {
		if n != 2 {
			t.Fatal(counts)
		}
	}
	// Reload reconstructs the complete wire before the original edge
	// subdivision; no in-memory terminal or display coordinates are reused.
	var wire bytes.Buffer
	if e := WriteAlgebraicShellSectionJSON(&wire, r); e != nil {
		t.Fatal(e)
	}
	o.Seeds = nil
	r, e = ReadAlgebraicShellSectionJSON(ctx, &wire, JSONLimits{}, o)
	if e != nil || len(r.Wires) != 1 || len(r.Wires[0]) != 4 {
		t.Fatal(r.Wires, e)
	}
	// Public diagnostics are caller-owned; edits cannot change authoritative
	// terminal ownership on the same immutable section result.
	r.Branches[0].EndEdges[0] = -1
	sub, e := r.SubdivideOriginalEdges(ctx, AlgebraicSeamOptions{})
	if e != nil || sub[0].Topology().Vertices != 8 || sub[1].Topology().Vertices != 12 ||
		sub[1].Topology().Edges != 16 || sub[1].Topology().EulerCharacteristic != 2 {
		t.Fatal(sub[0].Topology(), sub[1].Topology(), e)
	}
	var seam bytes.Buffer
	if e := WriteAlgebraicSubdivisionJSON(&seam, sub[1]); e != nil {
		t.Fatal(e)
	}
	q, e := ReadAlgebraicSubdivisionJSON(ctx, &seam, JSONLimits{}, AlgebraicSeamOptions{})
	if e != nil || q.Topology() != sub[1].Topology() {
		t.Fatal(q.Topology(), e)
	}
	checked := 0
	for i, x := range q.Edges() {
		if counts[x.Original] == 0 {
			continue
		}
		owners := 0
		for _, f := range q.Faces() {
			for _, use := range f.Outer {
				if use.Edge != i {
					continue
				}
				c, e := q.CoedgeChart(ctx, i, f.Original, AlgebraicSeamOptions{})
				if e != nil || c.UV == nil || !c.Model.Complete || c.Reverse != use.Reversed {
					t.Fatal(i, f.Original, e)
				}
				side, want := 0, x.Lower
				if want == nil {
					side, want = 1, x.Upper
				}
				root, e := c.UV.EndpointRoot(side)
				if e != nil {
					t.Fatal(e)
				}
				p, e := root.Compare(ctx, want, geom.ExactCurvePlaneEventOptions{})
				if e != nil || !p.Proven || p.Order != 0 {
					t.Fatal("lost exact owner parameter", p, e)
				}
				owners++
			}
		}
		if owners != 2 {
			t.Fatal("lost original coedge", i, owners)
		}
		checked++
	}
	if checked != 4 {
		t.Fatal("expected two pieces on each irrational seam", checked)
	}
	parts, e := r.ImprintOriginalShells(ctx, AlgebraicShellImprintOptions{})
	if e != nil || parts[0].Topology.Faces != 7 || parts[0].Topology.InnerWires != 1 || parts[1].Topology.Faces != 10 || parts[1].Topology.InnerWires != 0 ||
		parts[0].Topology.EulerCharacteristic != 2 || parts[1].Topology.EulerCharacteristic != 2 {
		t.Fatal("mixed shell imprint", parts[0].Topology, parts[1].Topology, e)
	}
	for i, p := range parts[0].Section {
		if parts[1].Section[i] != p {
			t.Fatal("lost shared paired section geometry", i)
		}
	}
	if q, e := r.ImprintOriginalShells(ctx, AlgebraicShellImprintOptions{MaxPieces: 1}); !errors.Is(e, geom.ErrConvergence) || len(q[0].Faces)+len(q[1].Faces) != 0 {
		t.Fatal("imprint piece cap was not atomic", e)
	}
	for _, row := range []struct {
		op           parametric.BooleanOp
		faces, holes int
	}{{parametric.Intersection, 5, 0}, {parametric.Union, 12, 1}, {parametric.Difference, 10, 1}} {
		q, e := r.CombineAlgebraic(ctx, row.op, AlgebraicShellBooleanOptions{})
		if e != nil || q.Topology.EulerCharacteristic != 2 || q.Topology.Faces != row.faces || q.Topology.InnerWires != row.holes {
			t.Fatal("irrational Boolean", row.op, q.Topology, e)
		}
		sections := 0
		for i, edge := range q.Edges {
			if edge.SectionPiece >= 0 {
				s, pair, e := q.SectionSegment(i)
				if e != nil || pair[0] != 4 || s.TrimA().Support() == nil || s.TrimB().Support() == nil {
					t.Fatal("lost paired source chart", i, pair, e)
				}
				sections++
			} else if c, e := q.SourceCurveCells(ctx, i, AlgebraicSeamOptions{}); e != nil || !c.Complete {
				t.Fatal("lost original edge", i, e)
			}
		}
		if sections != len(parts[0].Section) {
			t.Fatal("lost section pieces", sections, len(parts[0].Section))
		}
		history := q.History()
		if len(history.SectionEdges) != sections {
			t.Fatal("generated edge ancestry", history.SectionEdges)
		}
		seenEdges, seenFaces := make(map[int]bool), make(map[int]bool)
		for _, id := range history.SectionEdges {
			seenEdges[id] = true
		}
		for operand := range history.Edges {
			for original, row := range history.Edges[operand] {
				prev := -1
				for _, id := range row {
					x := q.index[id]
					if seenEdges[id] || x.Operand != operand || x.OriginalSubdivisionEdge <= prev ||
						parts[operand].Subdivision.edges[x.OriginalSubdivisionEdge].Original != original {
						t.Fatal("incorrect original edge ancestry or traversal order", operand, original, id)
					}
					seenEdges[id], prev = true, x.OriginalSubdivisionEdge
				}
			}
			for original, row := range history.Faces[operand] {
				for _, id := range row {
					if seenFaces[id] || q.faces[id].Operand != operand || q.faces[id].Original != original {
						t.Fatal("incorrect original face ancestry", operand, original, id)
					}
					seenFaces[id] = true
				}
			}
		}
		if len(seenEdges) != len(q.index) || len(seenFaces) != len(q.faces) {
			t.Fatal("history did not cover the result", len(seenEdges), len(seenFaces))
		}
		if row.op == parametric.Difference {
			owners := map[int]AlgebraicShellBooleanCoedge{}
			for fi, face := range q.faces {
				original, e := q.OriginalFace(fi)
				if e != nil {
					t.Fatal(e)
				}
				for li, ring := range append([][]AlgebraicShellBooleanUse{face.Outer}, face.Holes...) {
					for ui, use := range ring {
						x, e := q.Coedge(ctx, fi, li, ui, AlgebraicSeamOptions{})
						if e != nil || x.Edge != use.Edge || x.Reverse != use.Reversed || x.PhysicalReverse != (use.Reversed != face.Reversed) {
							t.Fatal("coedge direction or indexed owner", fi, li, ui, x, e)
						}
						if y, ok := owners[x.Edge]; ok && y.PhysicalReverse == x.PhysicalReverse {
							t.Fatal("opposite owners traverse one physical direction", x.Edge)
						}
						owners[x.Edge] = x
						if q.index[use.Edge].SectionPiece >= 0 {
							if x.Source != nil || x.Section == nil || !x.Section.HasSourceSurface(original.Surface) {
								t.Fatal("lost original chart of section edge", x.Edge)
							}
						} else if x.Source == nil || x.Section != nil || x.Source.UV == nil || !x.Source.Model.Complete {
							t.Fatal("lost restricted original edge chart", x.Edge)
						}
					}
				}
			}
		}
		if row.op == parametric.Intersection {
			b, e := q.BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 11})
			// Integrate roof thickness y²−1/2 on sqrt(1/2)<y<1
			// across width 1/2: volume is (sqrt(2)−1)/12.
			lower, upper := big.NewRat(414213562373095, 12000000000000000), big.NewRat(414213562373096, 12000000000000000)
			if e != nil || b.Volume.Lower.Cmp(lower) > 0 || b.Volume.Upper.Cmp(upper) < 0 || b.Volume.Lower.Sign() <= 0 {
				t.Fatal("certified Boolean volume", b, e)
			}
			lo, _ := b.Volume.Lower.Float64()
			hi, _ := b.Volume.Upper.Float64()
			t.Log("irrational intersection volume", lo, hi, "cells", b.VisitedCells, "ambiguous", b.AmbiguousCells)
			var f bytes.Buffer
			q.Faces[0].Original = -1 // reload must reconstruct from original sources
			if e := WriteAlgebraicShellBooleanJSON(&f, q); e != nil {
				t.Fatal(e)
			}
			back, e := ReadAlgebraicShellBooleanJSON(ctx, bytes.NewReader(f.Bytes()), JSONLimits{}, o, AlgebraicShellBooleanOptions{})
			if e != nil || back.Topology != q.Topology || back.Faces[0].Original < 0 {
				t.Fatal("Boolean native reconstruction", back.Topology, e)
			}
			if !reflect.DeepEqual(back.History(), history) {
				t.Fatal("history changed after native reconstruction")
			}
			history.SectionEdges[0] = -1
			if !reflect.DeepEqual(back.History(), q.History()) {
				t.Fatal("caller changed the private history snapshot")
			}
			if back, e = ReadAlgebraicShellBooleanJSON(ctx, bytes.NewReader(f.Bytes()), JSONLimits{MaxBytes: 16}, o, AlgebraicShellBooleanOptions{}); !errors.Is(e, geom.ErrConvergence) || len(back.Faces) != 0 {
				t.Fatal("Boolean native byte cap", e)
			}
		}
	}
}

func shellSectionRect(u0, u1, v0, v1 float64) geom.ParameterRectangle {
	return geom.ParameterRectangle{UMin: u0, UMax: u1, VMin: v0, VMax: v1}
}

func irrationalRoofSplineShell(t *testing.T) (Solid, []*geom.BSplineSurface) {
	t.Helper()
	const low, high, floor = 5.0 / 8, 1, -7.0 / 16
	x, y, roof := [2]float64{.25, .75}, [3]float64{low, (low + high) / 2, high}, [3]float64{-7.0 / 64, 1.0 / 8, .5}
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	add := func(nu, nv int, d geom.ParameterRectangle, rev bool, pole func(i, j int) geom.Vec3) {
		var p []geom.Vec3
		for i := range nu {
			for j := range nv {
				p = append(p, pole(i, j))
			}
		}
		knots := func(n int, lo, hi float64) []float64 {
			k := make([]float64, 2*n)
			for i := range n {
				k[i], k[i+n] = lo, hi
			}
			return k
		}
		s, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, p, knots(nu, d.UMin, d.UMax), knots(nv, d.VMin, d.VMax), nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces, domains, reversed = append(surfaces, s), append(domains, d), append(reversed, rev)
	}
	add(2, 2, shellSectionRect(x[0], x[1], low, high), true, func(i, j int) geom.Vec3 {
		return geom.Vec3{X: x[i], Y: y[2*j], Z: floor}
	})
	add(2, 3, shellSectionRect(x[0], x[1], low, high), false, func(i, j int) geom.Vec3 {
		return geom.Vec3{X: x[i], Y: y[j], Z: roof[j]}
	})
	for n := range 2 {
		add(3, 2, shellSectionRect(low, high, 0, 1), n == 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: x[n], Y: y[i], Z: [2]float64{floor, roof[i]}[j]}
		})
	}
	for n := range 2 {
		add(2, 2, shellSectionRect(x[0], x[1], 0, 1), n == 1, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: x[i], Y: y[2*n], Z: [2]float64{floor, roof[2*n]}[j]}
		})
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, surfaces
}

func mustBezierForAlgebraicShell(t *testing.T) *geom.BSpline {
	t.Helper()
	c, e := geom.NewBezier([]geom.Vec3{{}, {X: .5}, {X: 1, Y: 1}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
