package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestTrimmedSplinePairClosedAndHoled(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var poles []geom.Vec3
	for i := range x {
		for j := range x {
			poles = append(poles, geom.Vec3{X: x[i], Y: x[j], Z: z[i] + z[j] - 25.0/256 + x[i]*x[j]/64})
		}
	}
	k = []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patches := [2]intersect.Patch{{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, {Surface: b, Max: geom.Vec2{X: 1, Y: 1}}}
	loop := func(p [4]geom.Vec2) []Use {
		r := make([]Use, 4)
		for i := range r {
			r[i] = Use{Edge: i, UVStart: p[i], UVEnd: p[(i+1)%4]}
		}
		return r
	}
	outer := loop([4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}})
	faces := [2]Face{{Surface: a, Outer: outer}, {Surface: b, Outer: outer}}
	o := AlgebraicTrimmedPairSectionOptions{}
	cuts := []float64{.25, .3125, .6875, .75}
	o.Trace.GraphFallback.UCuts, o.Trace.GraphFallback.VCuts = cuts, cuts
	o.Trace.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	// Circle (u-1/2)^2+(v-1/2)^2=25/256; exact graph discovery uses
	// rational 3-4-5 switch points, followed by independent coverage.
	h, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, patches[0], patches[1], geom.SurfacePatchOptions{}, o.Trace.GraphFallback)
	if e != nil || len(h.Attempts) < 4 {
		t.Fatal(len(h.Attempts), e)
	}
	r, e := SectionTrimmedSplinePair(ctx, patches, faces, &h, o)
	if e != nil || !r.Complete || len(r.Branches) != 1 || len(r.Branches[0].Runs) != 1 ||
		!r.Branches[0].Runs[0].Closed || len(r.Branches[0].Runs[0].Intervals) != len(h.Attempts) {
		t.Fatalf("complete=%v branches=%d trace=%v err=%v", r.Complete, len(r.Branches), r.Trace.Issues, e)
	}
	for _, n := range r.Branches[0].Nodes {
		if !n.OnFaces || n.Isolated || len(n.Events) != 2 {
			t.Fatal(n)
		}
	}
	split, e := r.PartitionBothFacesByTubeRun(ctx, 0, 0, AlgebraicTubeFacePartitionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, p := range split.Faces {
		if !p.CutClosed || len(p.Cut) != len(h.Attempts) || !p.Fragments[0].CutIsOuter ||
			len(p.Fragments[0].Outer) != 0 || !p.Fragments[1].CutIsHole || len(p.Fragments[1].Outer) != 4 ||
			p.Fragments[0].CutReversed == p.Fragments[1].CutReversed || p.Support != patches[i].Surface {
			t.Fatalf("wrong closed fragment incidence in chart %d: %+v", i, p.Fragments)
		}
	}
	if split.Faces[0].FragmentHeightSigns != [2]int{1, -1} || split.Faces[1].FragmentHeightSigns != [2]int{-1, 1} {
		t.Fatal("height signs disagree with exact analytic graph residual")
	}
	lens, e := r.SealAlgebraicTubeLens(ctx, AlgebraicTubeLensOptions{})
	if e != nil || lens.Topology() != (AlgebraicTubeLensTopology{len(h.Attempts), len(h.Attempts), 2, 2}) {
		t.Fatalf("closed lens topology %+v err=%v", lens.Topology(), e)
	}
	checkLens := func(t *testing.T, l AlgebraicTubeLens) {
		t.Helper()
		f := l.Faces()
		if f[0].Reversed || !f[1].Reversed || (f[0].Reversed != f[0].CutReversed) == (f[1].Reversed != f[1].CutReversed) {
			t.Fatalf("outward lens faces %+v", f)
		}
		x, e := l.VerticalInterval(ctx, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !x.Complete || !x.Present || x.Rim || x.Lower.Cmp(big.NewRat(-24, 256)) != 0 || x.Upper.Cmp(big.NewRat(1, 256)) != 0 {
			t.Fatalf("analytic center interval %+v err=%v", x, e)
		}
		x, e = l.VerticalInterval(ctx, [2]*big.Rat{big.NewRat(13, 16), big.NewRat(1, 2)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !x.Complete || !x.Present || !x.Rim || x.Lower.Cmp(big.NewRat(13, 2048)) != 0 || x.Upper.Cmp(x.Lower) != 0 {
			t.Fatalf("analytic rim interval %+v err=%v", x, e)
		}
		x, e = l.VerticalInterval(ctx, [2]*big.Rat{big.NewRat(7, 8), big.NewRat(1, 2)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !x.Complete || x.Present {
			t.Fatalf("analytic exterior interval %+v err=%v", x, e)
		}
	}
	checkLens(t, lens)
	// This model point is exactly on the upper graph, independently
	// of the approximate boxes used by the lower-bound search.
	onTop := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 256)}
	dist, e := lens.BoundBoundaryDistance(ctx, onTop, AlgebraicTubeLensDistanceOptions{})
	if e != nil || !dist.Converged || dist.Squared.Lower.Sign() != 0 || dist.Squared.Upper.Sign() != 0 {
		t.Fatalf("on-face distance %+v: %v", dist, e)
	}
	// One step cannot resolve this distant point, but it must retain
	// a rigorous global lower bound and a valid model witness.
	query := [3]*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 2), big.NewRat(0, 1)}
	dist, e = lens.BoundBoundaryDistance(ctx, query, AlgebraicTubeLensDistanceOptions{MaxCells: 1})
	if e == nil || dist.Converged || dist.Squared.Lower == nil || dist.Squared.Upper == nil ||
		dist.Squared.Lower.Cmp(dist.Squared.Upper) > 0 || dist.Squared.Lower.Sign() <= 0 {
		t.Fatalf("limited distance %+v: %v", dist, e)
	}
	nativeSolid, e := lens.Solid(ctx, 1e-7, edgecheck.Options{})
	if e != nil || len(nativeSolid.Faces()) != 2 || nativeSolid.Topology().EulerCharacteristic != 2 {
		t.Fatalf("native lens topology %+v err=%v", nativeSolid.Topology(), e)
	}
	var solidFile bytes.Buffer
	if e := WriteJSON(&solidFile, nativeSolid); e != nil {
		t.Fatal(e)
	}
	solidBack, e := ReadJSON(bytes.NewReader(solidFile.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	agreement, e := solidBack.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete || len(agreement.Uses) != 2*len(h.Attempts) {
		t.Fatal(agreement.Complete, len(agreement.Uses), e)
	}
	base, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: -1}, {Y: 1, Z: -1}, {X: 1, Z: -1}, {X: 1, Y: 1, Z: -1}},
		[]float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	difference, e := r.SubtractAlgebraicTubeGraphSolids(ctx, base, 0, AlgebraicTubeGraphBooleanOptions{})
	if e != nil || len(difference.SelectedRegions()) != 1 || difference.ProofCells < 2 {
		t.Fatalf("graph subtraction components %v err=%v", difference.SelectedRegions(), e)
	}
	selected, e := difference.Solid(ctx, 0, 1e-7, edgecheck.Options{})
	if e != nil || selected.Topology().EulerCharacteristic != 2 {
		t.Fatal(selected.Topology(), e)
	}
	if d, e := r.SubtractAlgebraicTubeGraphSolids(ctx, base, 1, AlgebraicTubeGraphBooleanOptions{}); e == nil || len(d.SelectedRegions()) != 0 {
		t.Fatal("side-wall touching subtraction accepted", e)
	}
	if d, e := r.SubtractAlgebraicTubeGraphSolids(ctx, a, 0, AlgebraicTubeGraphBooleanOptions{}); e == nil || len(d.SelectedRegions()) != 0 {
		t.Fatal("zero-thickness input accepted", e)
	}
	var differenceFile bytes.Buffer
	if e := WriteAlgebraicTubeGraphDifferenceJSON(&differenceFile, difference); e != nil {
		t.Fatal(e)
	}
	differenceBack, e := ReadAlgebraicTubeGraphDifferenceJSON(ctx, bytes.NewReader(differenceFile.Bytes()), JSONLimits{}, o, AlgebraicTubeGraphBooleanOptions{})
	if e != nil || len(differenceBack.SelectedRegions()) != 1 {
		t.Fatal(differenceBack.SelectedRegions(), e)
	}
	point := [3]*big.Rat{big.NewRat(33, 64), big.NewRat(33, 64), big.NewRat(-1, 32)}
	where, e := differenceBack.LocateRational(ctx, point, geom.ExactCurvePlaneEventOptions{})
	if e != nil || where != parametric.Inside {
		t.Fatal(where, e)
	}
	var badDifference nativeTubeGraphDifference
	if e := json.Unmarshal(differenceFile.Bytes(), &badDifference); e != nil {
		t.Fatal(e)
	}
	badDifference.Minuend = 1
	badFile, e := json.Marshal(badDifference)
	if e != nil {
		t.Fatal(e)
	}
	if d, e := ReadAlgebraicTubeGraphDifferenceJSON(ctx, bytes.NewReader(badFile), JSONLimits{}, o, AlgebraicTubeGraphBooleanOptions{}); e == nil || len(d.SelectedRegions()) != 0 {
		t.Fatal("unsupported side-wall selection reloaded", e)
	}
	analytic := math.Pi * 625 / 131072
	exterior, e := r.SubtractAlgebraicTubeGraphSolidsWithExterior(ctx, base, 1, AlgebraicTubeGraphBooleanOptions{})
	if e != nil || !exterior.IncludesExterior() || len(exterior.SelectedRegions()) != 1 || exterior.SelectedRegions()[0] != -1 {
		t.Fatalf("exterior selection %v err=%v", exterior.SelectedRegions(), e)
	}
	exteriorSolid, e := exterior.Solid(ctx, 0, 1e-7, edgecheck.Options{})
	if e != nil || exteriorSolid.Topology().EulerCharacteristic != 0 || len(exteriorSolid.Faces()) != 6 {
		t.Fatalf("exterior topology %+v err=%v", exteriorSolid.Topology(), e)
	}
	volume, e := exteriorSolid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-4})
	if e != nil || math.Abs(volume.Volume-(1.0/6-25.0/256+analytic)) > 2e-6 {
		t.Fatal(volume, e)
	}
	where, e = exterior.LocateRational(ctx, [3]*big.Rat{big.NewRat(0, 1), big.NewRat(33, 64), big.NewRat(1, 32)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || where != parametric.Boundary {
		t.Fatal("original side wall classification", where, e)
	}
	var exteriorFile bytes.Buffer
	if e := WriteAlgebraicTubeGraphDifferenceJSON(&exteriorFile, exterior); e != nil {
		t.Fatal(e)
	}
	exteriorBack, e := ReadAlgebraicTubeGraphDifferenceJSON(ctx, bytes.NewReader(exteriorFile.Bytes()), JSONLimits{}, o, AlgebraicTubeGraphBooleanOptions{})
	if e != nil || !exteriorBack.IncludesExterior() {
		t.Fatal(e)
	}
	for _, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection} {
		s, history, e := r.AssembleAlgebraicTubeGraphEnvelope(ctx, base, op, 1e-7, AlgebraicTubeGraphBooleanOptions{}, edgecheck.Options{})
		if e != nil || s.Topology().EulerCharacteristic != 2 || len(s.Faces()) != 7 || len(history.Walls) != 4 || len(history.SectionEdges) != len(h.Attempts) {
			t.Fatalf("envelope op=%v history=%+v topology=%+v err=%v", op, history, s.Topology(), e)
		}
		// Integrals of both top candidates over the unit chart differ
		// by 1/6-25/256; adding/subtracting the analytic lens selects
		// the top height envelope.
		want := 1 + 1.0/256 - analytic
		if op == parametric.Union {
			want = 1 + 1.0/256 + 1.0/6 - 25.0/256 + analytic
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-4})
		if e != nil || math.Abs(m.Volume-want) > 2e-6 {
			t.Fatalf("envelope op=%v volume=%g want=%g err=%v", op, m.Volume, want, e)
		}
	}
	var lensFile bytes.Buffer
	if e := WriteAlgebraicTubeLensJSON(&lensFile, lens); e != nil {
		t.Fatal(e)
	}
	reloadedLens, e := ReadAlgebraicTubeLensJSON(ctx, bytes.NewReader(lensFile.Bytes()), JSONLimits{}, o, AlgebraicTubeLensOptions{})
	if e != nil {
		t.Fatal(e)
	}
	checkLens(t, reloadedLens)
	// Integrate (25/256-r²) over the circle independently: pi*(25/256)²/2.
	var previous geom.ExactParameterInterval
	for _, depth := range []int{6, 10} {
		v, e := lens.BoundVolume(ctx, AlgebraicTubeLensVolumeOptions{MaxDepth: depth})
		if e != nil {
			t.Fatal(v, e)
		}
		lo, _ := v.Volume.Lower.Float64()
		hi, _ := v.Volume.Upper.Float64()
		if lo <= 0 || lo > analytic || hi < analytic {
			t.Fatalf("exact volume [%g,%g] does not enclose analytic %g", lo, hi, analytic)
		}
		if previous.Lower != nil && (v.Volume.Lower.Cmp(previous.Lower) <= 0 || v.Volume.Upper.Cmp(previous.Upper) >= 0) {
			t.Fatal("depth refinement did not tighten both volume bounds")
		}
		previous = v.Volume
	}
	if v, e := lens.BoundVolume(ctx, AlgebraicTubeLensVolumeOptions{MaxCells: 1}); e == nil || v.Volume.Lower != nil {
		t.Fatal("partial bounded volume published", e)
	}
	// Disk symmetry gives cx=cy=1/2. The height-weighted average of
	// r² is R²/3, so cz=1/256-R²/3=-11/384.
	mass, e := lens.BoundMassProperties(ctx, AlgebraicTubeLensVolumeOptions{MaxDepth: 10})
	if e != nil {
		t.Fatal(mass, e)
	}
	for i, z := range [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(-11, 384)} {
		b := mass.Centroid[i]
		if b.Lower == nil || b.Lower.Cmp(z) > 0 || b.Upper.Cmp(z) < 0 {
			t.Fatalf("centroid coordinate %d: %+v does not enclose %s", i, b, z)
		}
	}
	for _, x := range []struct {
		i, j  int
		exact float64
	}{
		{0, 1, -analytic / 4},
		{1, 0, -analytic / 4},
		{2, 2, analytic * (.5 + 25.0/768)},
	} {
		b := mass.Inertia[x.i][x.j]
		lo, _ := b.Lower.Float64()
		hi, _ := b.Upper.Float64()
		if lo > x.exact || hi < x.exact {
			t.Fatalf("inertia (%d,%d) [%g,%g] excludes %g", x.i, x.j, lo, hi, x.exact)
		}
	}
	limits := [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(2, 1)}
	direction := geom.Vec3{X: 1, Z: 1.0 / 128}
	line, e := lens.ClipMaterialLine(ctx, geom.Vec3{Y: .5, Z: -1.0 / 32}, direction, limits, AlgebraicLensLineOptions{})
	if e != nil || !line.Complete || len(line.Intervals) != 1 || len(line.Isolated) != 0 {
		t.Fatalf("line %+v: %v", line, e)
	}
	for i, event := range []AlgebraicTubeLensLineEvent{line.Intervals[0].Start, line.Intervals[0].End} {
		bounds := [2][2]string{{"0", "1/2"}, {"1/2", "1"}}[i]
		want, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"47", "-256", "256"},
			Lower: bounds[0], Upper: bounds[1]}, geom.ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		c, e := event.Parameter.Compare(ctx, want, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !c.Proven || c.Order != 0 || event.BoundaryMask != 2 {
			t.Fatal(event, c, e)
		}
	}
	line, e = lens.ClipMaterialLine(ctx, geom.Vec3{Y: .5, Z: -25.0 / 256}, direction, limits, AlgebraicLensLineOptions{})
	if e != nil || !line.Complete || len(line.Intervals) != 0 || len(line.Isolated) != 1 || line.Isolated[0].BoundaryMask != 2 {
		t.Fatalf("isolated tangent %+v: %v", line, e)
	}
	line, e = lens.ClipMaterialLine(ctx, geom.Vec3{Y: .5, Z: 1.0 / 32}, direction, limits, AlgebraicLensLineOptions{})
	if e != nil || !line.Complete || len(line.Intervals) != 0 || len(line.Isolated) != 0 {
		t.Fatalf("outside graph roots wrongly retained %+v: %v", line, e)
	}
	line, e = lens.ClipMaterialLine(ctx, geom.Vec3{Y: .5}, direction, limits, AlgebraicLensLineOptions{})
	if e != nil || !line.Complete || len(line.Intervals) != 1 || len(line.Isolated) != 0 ||
		line.Intervals[0].BoundaryMask != 1 || line.Intervals[0].Start.BoundaryMask != 7 || line.Intervals[0].End.BoundaryMask != 7 {
		t.Fatalf("coincident top graph %+v: %v", line, e)
	}
	cut, e := lens.SectionVerticalPlane(ctx, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(33, 64)},
		[2]*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1)}, limits, AlgebraicLensLineOptions{})
	if e != nil || !cut.Complete || len(cut.Regions) != 1 || len(cut.Isolated) != 0 || cut.Regions[0].Clipped != [2]bool{} {
		t.Fatalf("vertical plane section %+v err=%v", cut, e)
	}
	region := cut.Regions[0]
	for i, x := range region.Parameter {
		bounds := [2][2]string{{"0", "1/2"}, {"1/2", "1"}}[i]
		want, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"625", "-4096", "4096"},
			Lower: bounds[0], Upper: bounds[1]}, geom.ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		q, e := x.Compare(ctx, want, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !q.Proven || q.Order != 0 {
			t.Fatal(q, e)
		}
	}
	for i, c := range region.Curves {
		m, e := c.Cells[0].EvaluateRational(ctx, big.NewRat(1, 2), geom.ExactCurvePlaneEventOptions{})
		if e != nil || m.Point[0].Cmp(big.NewRat(1, 2)) != 0 || m.Point[1].Cmp(big.NewRat(33, 64)) != 0 {
			t.Fatalf("original section chart %d: %+v %v", i, m, e)
		}
	}
	cut, e = lens.SectionVerticalPlane(ctx, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(13, 16)},
		[2]*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1)}, limits, AlgebraicLensLineOptions{})
	if e != nil || !cut.Complete || len(cut.Regions) != 0 || len(cut.Isolated) != 1 {
		t.Fatalf("tangent vertical plane %+v err=%v", cut, e)
	}
	inside := loop([4]geom.Vec2{{X: 7.0 / 16, Y: 7.0 / 16}, {X: 7.0 / 16, Y: 9.0 / 16}, {X: 9.0 / 16, Y: 9.0 / 16}, {X: 9.0 / 16, Y: 7.0 / 16}})
	outside := loop([4]geom.Vec2{{X: 1.0 / 16, Y: 1.0 / 16}, {X: 1.0 / 16, Y: 1.0 / 8}, {X: 1.0 / 8, Y: 1.0 / 8}, {X: 1.0 / 8, Y: 1.0 / 16}})
	faces[0].Holes = [][]Use{inside, outside}
	withHoles, e := SectionTrimmedSplinePair(ctx, patches, faces, &h, o)
	if e != nil {
		t.Fatal(e)
	}
	p, e := withHoles.PartitionFaceByTubeRun(ctx, 0, 0, 0, AlgebraicTubeFacePartitionOptions{})
	if e != nil || len(p.Fragments[0].Holes) != 1 || len(p.Fragments[1].Holes) != 1 ||
		p.Fragments[0].Holes[0][0].Loop != 1 || p.Fragments[1].Holes[0][0].Loop != 2 {
		t.Fatalf("hole assignment %+v: %v", p.Fragments, e)
	}
	// A clockwise hole removes the arc above v=3/4. Its vertical walls
	// are outside that arc: the endpoints are exactly u=5/16 and 11/16.
	faces[0].Holes = [][]Use{loop([4]geom.Vec2{{X: .25, Y: .75}, {X: .25, Y: .875}, {X: .75, Y: .875}, {X: .75, Y: .75}})}
	r, e = SectionTrimmedSplinePair(ctx, patches, faces, &h, o)
	if e != nil || !r.Complete || len(r.Branches) != 1 || len(r.Branches[0].Runs) != 1 ||
		r.Branches[0].Runs[0].Closed {
		t.Fatalf("complete=%v branches=%d trace=%v err=%v", r.Complete, len(r.Branches), r.Trace.Issues, e)
	}
	b0 := r.Branches[0]
	run := b0.Runs[0].Intervals
	for _, n := range []int{run[0].Start, run[len(run)-1].End} {
		found := false
		for _, ref := range b0.Nodes[n].Events {
			for _, h := range b0.Clips[ref.Segment].Events[ref.Event].Contacts[0] {
				found = found || h.Source.Loop == 1
			}
		}
		if !found {
			t.Fatal("missing hole ownership", n)
		}
	}
	var file bytes.Buffer
	// The stored original geometry is independent of public proof results
	// and of the caller's editable loop and seed slices.
	r.Branches[0].Runs = nil
	faces[0].Holes[0][0].Edge = -1
	if e := WriteTrimmedSplinePairSectionJSON(&file, r); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadTrimmedSplinePairSectionJSON(ctx, bytes.NewReader(file.Bytes()), JSONLimits{}, o)
	if e != nil || !loaded.Complete || len(loaded.Branches) != 1 || len(loaded.Branches[0].Runs) != 1 ||
		loaded.Branches[0].Runs[0].Closed || len(loaded.Branches[0].Runs[0].Intervals) != len(run) {
		t.Fatalf("reload complete=%v branches=%d err=%v", loaded.Complete, len(loaded.Branches), e)
	}
	var native nativeTrimmedPairSection
	if e := json.Unmarshal(file.Bytes(), &native); e != nil {
		t.Fatal(e)
	}
	native.Tubes = nil
	malformed, e := json.Marshal(native)
	if e != nil {
		t.Fatal(e)
	}
	if q, e := ReadTrimmedSplinePairSectionJSON(ctx, bytes.NewReader(malformed), JSONLimits{}, o); e == nil || q.Complete {
		t.Fatal("missing tube coverage accepted", e)
	}
	if q, e := ReadTrimmedSplinePairSectionJSON(ctx, bytes.NewReader(file.Bytes()), JSONLimits{MaxEntities: 3}, o); e == nil || q.Complete {
		t.Fatal("entity budget accepted", e)
	}
}
