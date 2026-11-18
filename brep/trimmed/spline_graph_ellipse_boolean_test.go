package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestEllipticalContactSplineGraphBoolean(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	graph := func(z, f float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				// A-B=1-16(u-.5)^2-64(v-.5)^2.
				u, v := float64(i)/2, float64(j)/2
				term := []float64{.25, -.25, .25}
				p = append(p, geom.Vec3{X: u, Y: v, Z: z + f*(1-16*term[i]-64*term[j])})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := graph(0, 0), graph(24, 1), graph(24, 0)
	hint := geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 64)}}
	opts := intersect.SplinePlaneOptions{}
	material, e := NewSharedBaseSplineGraphMaterial(ctx, base, a, b, opts)
	if e != nil {
		t.Fatal(e)
	}
	eval := geom.SurfacePatchEvaluationOptions{}
	contact := [3]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2), big.NewRat(24, 1)}
	if _, e := material.LocateDifferenceRational(ctx, contact, eval); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("unproved closure should stay unresolved", e)
	}
	ellipseForm := geom.GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{
		big.NewRat(19, 1), big.NewRat(-16, 1), big.NewRat(16, 1), big.NewRat(-64, 1), big.NewRat(64, 1), new(big.Rat),
	}}
	if loc, e := material.LocateDifferenceRationalWithEllipse(ctx, contact, ellipseForm, geom.GraphPointContactOptions{}, eval); e != nil || loc != parametric.Boundary {
		t.Fatal("ellipse contact belongs to closed difference", loc, e)
	}
	if loc, e := material.LocateDifferenceRational(ctx, [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(49, 2)}, eval); e != nil || loc != parametric.Inside {
		t.Fatal(loc, e)
	}
	lens := math.Pi / 64
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineEllipticalContactSplineGraphs(ctx, base, a, b, op, 1e-9, opts, hint, geom.GraphPointContactOptions{})
		if e != nil || h.Ellipse == nil || len(h.ContourEdges) != 4 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(h, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		want := [3]float64{24 + lens, 24 - 17.0/3 - lens, lens}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-8 {
			t.Fatal(p, want, e)
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		q, e := ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil || q.Topology().EulerCharacteristic != 2 {
			t.Fatal(q.Topology(), e)
		}
		if e := validateExactGraphPartitionEdges(ctx, q, geom.GraphPointContactOptions{}); e != nil {
			t.Fatal(e)
		}
	}
	forced := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	if _, h, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Union, 1e-9, forced); e != nil || h.Ellipse == nil {
		t.Fatal("automatic ellipse fallback", h, e)
	}
	s, h, e := CombineEllipticalContactSplineGraphs(ctx, base, b, a, parametric.Difference, 1e-9, opts, hint, geom.GraphPointContactOptions{})
	if e != nil || h.Ellipse == nil || len(h.ContourEdges) != 4 || s.Topology().InnerWires != 2 || s.Topology().EulerCharacteristic != 0 {
		t.Fatal(h, s.Topology(), e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(p.Volume-(17.0/3+lens)) > 1e-8 {
		t.Fatal(p, e)
	}
	// Reparameterize the same four exact conic quadrants with a rational
	// projective scale of 1/3. Endpoints stay dyadic, while the weight
	// ratios [1,1/3,2/9] cannot use the native float64 spline encoding.
	loop, e := ellipseGraphContactLoop(hint)
	if e != nil {
		t.Fatal(e)
	}
	for i, edge := range loop {
		p := edge.c.(SplineUV).Curve.Poles()
		w := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 3), big.NewRat(2, 9)}
		var net [3][3]*big.Rat
		for j, c := range p {
			net[j] = [3]*big.Rat{new(big.Rat).Mul(ratInset(c.X), w[j]), new(big.Rat).Mul(ratInset(c.Y), w[j]), w[j]}
		}
		loop[i].c, e = NewExactQuadraticUV(net)
		if e != nil {
			t.Fatal(e)
		}
	}
	lossless, _, e := assembleGraphClosedContact(ctx, base, a, b, parametric.Difference, 1e-9,
		geom.ParameterRectangle{UMax: 1, VMax: 1}, 1, -1, loop, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cutY := big.NewRat(11, 20)
	cutPlane := [4]*big.Rat{nil, big.NewRat(1, 1), nil, new(big.Rat).Neg(cutY)}
	var cutRecipes []AlgebraicSeamRecipe
	for i, edge := range lossless.edges {
		model := edge.Curve.(surfaceTrimCurve)
		q := model.curve.(ExactQuadraticUV)
		patch, err := exactFaceSupportPatch(Face{Surface: model.surface}, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		r, err := q.IntersectModelRawPlane(ctx, patch, cutPlane, geom.SurfaceCurveAgreementOptions{}, geom.ExactCurvePlaneEventOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, hit := range r.Hits {
			recipe, _, err := lossless.PrepareAlgebraicSeamRecipe(ctx, i, cutPlane, hit.Event, AlgebraicSeamOptions{})
			if err != nil {
				t.Fatal(err)
			}
			cutRecipes = append(cutRecipes, recipe)
		}
	}
	subdivided, e := lossless.SubdivideAlgebraicSeams(ctx, cutRecipes, AlgebraicSeamOptions{})
	if e != nil || len(cutRecipes) != 2 {
		t.Fatal(cutRecipes, e)
	}
	fragments, e := subdivided.PartitionFaceByAlgebraicUVLine(ctx, 0, [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat).Neg(cutY)}, AlgebraicSeamOptions{})
	if e != nil || fragments.Vertices[0] == fragments.Vertices[1] || len(fragments.Fragments[0])+len(fragments.Fragments[1]) != len(lossless.edges)+4 {
		t.Fatal(fragments, e)
	}
	for i, r := range fragments.Fragments {
		last := r[len(r)-1]
		if last.BoundaryEdge != -1 || last.Reversed != (i == 0) || math.Abs(fragments.CutUV[i].Point.Y-.55) > 1e-9 {
			t.Fatal(fragments)
		}
	}
	if fragments.FragmentSigns != ([2]int{1, -1}) && fragments.FragmentSigns != ([2]int{-1, 1}) {
		t.Fatal(fragments.FragmentSigns)
	}
	modelCut, sharedDisplay, e := subdivided.TessellateAlgebraicPartitionCutCertified(ctx, 0,
		[3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat).Neg(cutY)}, AlgebraicSeamOptions{},
		geom.ComposedCurvePolylineOptions{Deflection: 1e-5})
	if e != nil || !modelCut.Model.Complete || modelCut.Axis != 0 || len(sharedDisplay.Segments) < 2 || !sharedDisplay.Complete {
		t.Fatal(modelCut, sharedDisplay, e)
	}
	for i, j := range [2]int{0, len(sharedDisplay.Vertices) - 1} {
		if sharedDisplay.Vertices[j].Point != subdivided.vertices[fragments.Vertices[i]].Display {
			t.Fatal("unshared cut display", sharedDisplay.Vertices[j])
		}
	}
	// The upper support is nonlinear along this cut, independently checked
	// at its exact centered parameter. Its elevation is 24+1-64*(1/20)^2.
	cutMid, e := modelCut.Model.Cells[0].EvaluateRational(ctx, big.NewRat(1, 2), geom.ExactCurvePlaneEventOptions{})
	if e != nil || cutMid.Point[1].Cmp(cutY) != 0 || cutMid.Point[2].Cmp(big.NewRat(621, 25)) != 0 {
		t.Fatal(cutMid, e)
	}
	cutLine := [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat).Neg(cutY)}
	arrangement, e := subdivided.ArrangeFacesByAlgebraicUVLines(ctx, []AlgebraicFaceCut{{Face: 0, Line: cutLine}, {Face: 1, Line: cutLine}}, AlgebraicSeamOptions{})
	if e != nil || arrangement.Topology().EulerCharacteristic != 2 || len(arrangement.Faces()) != 4 || len(arrangement.Edges()) != 8 {
		t.Fatal(arrangement.Topology(), e)
	}
	var arrangementFile bytes.Buffer
	if e := WriteAlgebraicFaceArrangementJSON(&arrangementFile, arrangement); e != nil {
		t.Fatal(e)
	}
	restoredArrangement, e := ReadAlgebraicFaceArrangementJSON(ctx, bytes.NewReader(arrangementFile.Bytes()), JSONLimits{}, AlgebraicSeamOptions{})
	if e != nil || restoredArrangement.Topology() != arrangement.Topology() {
		t.Fatal(restoredArrangement.Topology(), e)
	}
	for i, f := range restoredArrangement.Faces() {
		if f.Original != i/2 || f.Fragment != i%2 || f.Outer[len(f.Outer)-1].OriginalUse != -1 {
			t.Fatal(f)
		}
	}
	var editedArrangement nativeAlgebraicFaceArrangement
	if e := json.Unmarshal(arrangementFile.Bytes(), &editedArrangement); e != nil {
		t.Fatal(e)
	}
	editedArrangement.Cuts[0].Line[2] = "-3/5"
	editedData, e := json.Marshal(editedArrangement)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadAlgebraicFaceArrangementJSON(ctx, bytes.NewReader(editedData), JSONLimits{}, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("edited arrangement line lost its source identity", e)
	}
	contacts, e := IntersectAlgebraicArrangementEdges(ctx, arrangement, 6, restoredArrangement, 7, 0, AlgebraicArrangementContactOptions{})
	if e != nil || !contacts.Complete || len(contacts.Coincident) != 0 || len(contacts.Hits) != 2 {
		t.Fatal(contacts, e)
	}
	for i, h := range contacts.Hits {
		cmp, e := h.A.Root().Compare(ctx, h.B.Root(), geom.ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal(cmp, e)
		}
		b, e := h.B.Enclose(ctx, geom.ComposedCurveEndpointOptions{})
		want := .5 + float64(2*i-1)*math.Sqrt(21)/20
		if e != nil || math.Abs(b.Point.X-want) > 1e-9 || math.Abs(b.Point.Z-24) > 1e-9 {
			t.Fatal(b, want, e)
		}
	}
	coincident, e := IntersectAlgebraicArrangementEdges(ctx, arrangement, 6, restoredArrangement, 6, 0, AlgebraicArrangementContactOptions{})
	if e != nil || !coincident.Complete || len(coincident.Coincident) != 1 || len(coincident.Hits) != 0 {
		t.Fatal(coincident, e)
	}
	// Use the lower graph as the authoritative model seam to expose a
	// quadratic conic rather than the upper chart's degree-eight composition.
	linearSeam, _, e := assembleGraphClosedContact(ctx, base, a, b, parametric.Difference, 1e-9,
		geom.ParameterRectangle{UMax: 1, VMax: 1}, 1, -1, loop, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	edges := linearSeam.Edges()
	for i, edge := range edges {
		m := edge.Curve.(surfaceTrimCurve)
		m.surface = b
		edges[i].Curve = m
	}
	linearSeam, e = NewSolidContext(ctx, linearSeam.Vertices(), edges, linearSeam.Faces(), linearSeam.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	linearSub, e := linearSeam.SubdivideAlgebraicSeams(ctx, cutRecipes, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	linearArr, e := linearSub.ArrangeFacesByAlgebraicUVLines(ctx, []AlgebraicFaceCut{{Face: 1, Line: cutLine}}, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	conicOverlap, e := IntersectAlgebraicArrangementEdges(ctx, linearArr, 0, linearArr, 0, -1, AlgebraicArrangementContactOptions{})
	if e != nil || !conicOverlap.Complete || len(conicOverlap.Hits) != 0 || len(conicOverlap.Coincident) != 1 {
		t.Fatal(conicOverlap, e)
	}
	selection := [2][]AlgebraicSewFace{{{Face: 0}, {Face: 1}}, {{Face: 2}, {Face: 3}}}
	sewPairs := make([]AlgebraicSewPair, len(subdivided.edges))
	for i := range sewPairs {
		sewPairs[i] = AlgebraicSewPair{Edges: [2]int{i, i}}
	}
	sewn, e := SewAlgebraicArrangementFragments(ctx, arrangement, restoredArrangement, selection, sewPairs, AlgebraicArrangementContactOptions{})
	if e != nil || sewn.EulerCharacteristic != 2 || len(sewn.Edges) != 8 || len(sewn.Faces) != 4 || len(sewn.VertexSources) != 6 {
		t.Fatal(sewn, e)
	}
	for i, src := range sewn.EdgeSources {
		if i < len(sewPairs) && len(src) != 2 || i >= len(sewPairs) && len(src) != 1 {
			t.Fatal(i, src)
		}
	}
	if _, e := SewAlgebraicArrangementFragments(ctx, arrangement, restoredArrangement, selection, sewPairs[:len(sewPairs)-1], AlgebraicArrangementContactOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("incomplete sewing accepted", e)
	}
	if same, e := proveSewAlgebraicEdge(ctx, arrangement, restoredArrangement, AlgebraicSewPair{Edges: [2]int{6, 6}}, AlgebraicArrangementContactOptions{}); e != nil || !same {
		t.Fatal("exact generated edge did not match", same, e)
	}
	if same, e := proveSewAlgebraicEdge(ctx, arrangement, restoredArrangement, AlgebraicSewPair{Edges: [2]int{6, 7}}, AlgebraicArrangementContactOptions{}); e != nil || same {
		t.Fatal("isolated endpoints promoted to full edge", same, e)
	}
	var sewnFile bytes.Buffer
	if e := WriteAlgebraicSewJSON(&sewnFile, sewn); e != nil {
		t.Fatal(e)
	}
	restoredSew, e := ReadAlgebraicSewJSON(ctx, bytes.NewReader(sewnFile.Bytes()), JSONLimits{}, AlgebraicArrangementContactOptions{})
	if e != nil || restoredSew.EulerCharacteristic != 2 || len(restoredSew.Faces) != 4 {
		t.Fatal(restoredSew, e)
	}
	if c, e := restoredSew.SourceModelCurveCells(ctx, 0, 1, AlgebraicSeamOptions{}); e != nil || !c.Complete {
		t.Fatal(c, e)
	}
	var staleSew nativeAlgebraicSew
	if e := json.Unmarshal(sewnFile.Bytes(), &staleSew); e != nil {
		t.Fatal(e)
	}
	staleSew.Pairs[0].Reverse = true
	staleBytes, e := json.Marshal(staleSew)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadAlgebraicSewJSON(ctx, bytes.NewReader(staleBytes), JSONLimits{}, AlgebraicArrangementContactOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("edited sewing direction accepted", e)
	}
	if _, e := subdivided.PartitionFaceByAlgebraicUVLine(ctx, 0, [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), big.NewRat(-3, 5)}, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("different line accepted persisted split identities", e)
	}
	// A small closed wire strictly below the cut remains a hole of the
	// negative fragment. Its fill face remains unpartitioned.
	holeLoop, e := ellipseGraphContactLoop(geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(7, 16)},
		RadiusSquared: [2]*big.Rat{big.NewRat(1, 1024), big.NewRat(1, 1024)}})
	if e != nil {
		t.Fatal(e)
	}
	var holePath []PCurve
	for _, x := range holeLoop {
		holePath = append(holePath, x.c)
	}
	holed, holedHistory, e := lossless.SplitFaceCurvedLoopContext(ctx, 0, holePath)
	if e != nil {
		t.Fatal(e)
	}
	holedSub, e := holed.SubdivideAlgebraicSeams(ctx, cutRecipes, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	holedPart, e := holedSub.PartitionFaceByAlgebraicUVLine(ctx, holedHistory.Outer, cutLine, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, sign := range holedPart.FragmentSigns {
		if sign < 0 && len(holedPart.HoleFragments[i]) != 1 || sign > 0 && len(holedPart.HoleFragments[i]) != 0 {
			t.Fatal(holedPart)
		}
	}
	holedArr, e := holedSub.ArrangeFacesByAlgebraicUVLines(ctx, []AlgebraicFaceCut{{Face: holedHistory.Outer, Line: cutLine}}, AlgebraicSeamOptions{})
	if e != nil || holedArr.Topology().InnerWires != 1 || holedArr.Topology().EulerCharacteristic != 2 {
		t.Fatal(holedArr.Topology(), e)
	}
	if s, _, e := lossless.SplitExactQuadraticTrimEdge(ctx, 0, big.NewRat(1, 2), edgecheck.Options{}); !errors.Is(e, geom.ErrUnrepresentable) || len(s.faces) != 0 {
		t.Fatal("split rounded an exact vertex", e)
	}
	splitAt := big.NewRat(3, 7)
	split, edit, e := lossless.SplitRationalEndpointQuadraticTrimEdge(ctx, 0, splitAt, edgecheck.Options{})
	if e != nil || split.Topology().EulerCharacteristic != 2 || len(split.edges) != len(lossless.edges)+1 {
		t.Fatal("rational topology split", split.Topology(), edit, e)
	}
	checkRational := func(s Solid, expected [3]*big.Rat) {
		t.Helper()
		points, e := s.RationalVertices(ctx)
		for _, v := range points {
			if v.Vertex == edit.Vertex {
				if e != nil || !sameRationalVertex(v.Point, expected) || v.Display != s.vertices[edit.Vertex] {
					t.Fatal(v, expected, e)
				}
				return
			}
		}
		t.Fatal("missing split vertex", points, e)
	}
	// Independent rational Bernstein evaluation of the first quadrant.
	// The scaled basis weights at 3/7 are 16:8:2.
	expected := [3]*big.Rat{big.NewRat(19, 26), big.NewRat(57, 104), big.NewRat(24, 1)}
	if _, e := geom.RepresentedExactPoint(expected); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("fixture split point must be nonbinary", expected, e)
	}
	checkRational(split, expected)
	var v5 bytes.Buffer
	if e := WriteJSON(&v5, split); e != nil {
		t.Fatal(e)
	}
	newFile, e := ReadJSONContext(ctx, bytes.NewReader(v5.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	checkRational(newFile, expected)
	var splitNative nativeSolid
	if e := json.Unmarshal(v5.Bytes(), &splitNative); e != nil || splitNative.Version != 5 {
		t.Fatal(splitNative.Version, e)
	}
	oldDisplay := splitNative.Vertices[edit.Vertex].X
	splitNative.Vertices[edit.Vertex].X = math.Nextafter(oldDisplay, math.Inf(1))
	badDisplay, e := json.Marshal(splitNative)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJSONContext(ctx, bytes.NewReader(badDisplay), JSONLimits{}); e == nil {
		t.Fatal("a display vertex tampered by one ULP must fail exact reconstruction")
	}
	splitNative.Vertices[edit.Vertex].X, splitNative.Version = oldDisplay, 4
	oldCodec, e := json.Marshal(splitNative)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJSONContext(ctx, bytes.NewReader(oldCodec), JSONLimits{}); e == nil {
		t.Fatal("old version accepted rational endpoint flags")
	}
	translation := geom.Vec3{X: 2, Y: -1, Z: 3}
	movedSplit, e := newFile.Transformed(geom.Translation(translation))
	if e != nil {
		t.Fatal(e)
	}
	checkRational(movedSplit, [3]*big.Rat{new(big.Rat).Add(expected[0], big.NewRat(2, 1)), new(big.Rat).Sub(expected[1], big.NewRat(1, 1)), big.NewRat(27, 1)})
	merged, _, e := newFile.MergeExactQuadraticTrimEdges(ctx, edit.First, edit.Second, splitAt)
	if e != nil || merged.Topology().Edges != lossless.Topology().Edges || merged.Topology().EulerCharacteristic != 2 {
		t.Fatal("rational topology merge", merged.Topology(), e)
	}
	var file bytes.Buffer
	if e := WriteJSON(&file, lossless); e != nil {
		t.Fatal(e)
	}
	q, e := ReadJSONContext(ctx, bytes.NewReader(file.Bytes()), JSONLimits{})
	if e != nil || q.faces[0].Outer[0].Curve.(ExactQuadraticUV).HomogeneousControls()[1][2].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(q.Topology(), e)
	}
	if e := validateExactGraphPartitionEdges(ctx, q, geom.GraphPointContactOptions{}); e != nil {
		t.Fatal(e)
	}
	ray := SplineRayOptions{Line: geom.LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 30}}
	for _, probe := range []struct {
		x, y float64
		hits int
	}{{.5, .5, 2}, {.8, .5, 0}} {
		r, e := q.SplineRay(ctx, geom.Vec3{X: probe.x, Y: probe.y}, geom.Vec3{Z: 1}, ray)
		if e != nil || !r.Complete || len(r.Hits) != probe.hits {
			t.Fatal("nonbinary conic trim ray", r, e)
		}
		if len(r.Hits) == 2 && (r.Hits[0].Direction != -1 || r.Hits[1].Direction != 1) {
			t.Fatal("nonbinary conic trim crossing orientation", r)
		}
	}
	if r, e := q.SplineRay(ctx, geom.Vec3{X: .75, Y: .5}, geom.Vec3{Z: 1}, ray); !errors.Is(e, geom.ErrConvergence) || r.Complete || len(r.Issues) == 0 {
		t.Fatal("shared conic seam must remain unresolved", r, e)
	}
	placed, e := q.Transformed(geom.Translation(geom.Vec3{X: 2, Y: -1, Z: 3}))
	if e != nil {
		t.Fatal(e)
	}
	var placement bytes.Buffer
	if e := WriteJSON(&placement, placed); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadJSONContext(ctx, &placement, JSONLimits{})
	if e != nil || restored.Topology().Faces != 2 {
		t.Fatal(restored.Topology(), e)
	}
	m, e := q.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(m.Volume-lens) > 1e-8 {
		t.Fatal(m, e)
	}
	var bad nativeSolid
	if e := json.Unmarshal(file.Bytes(), &bad); e != nil || bad.Version != 4 {
		t.Fatal(bad.Version, e)
	}
	// The tiny control edit stays below construction tolerance, but
	// loses exact whole-edge agreement and must fail native reload.
	edited := bad.Faces[1].Outer[0].ExactQuadratic
	old := edited[1][0]
	v, _ := new(big.Rat).SetString(old)
	v.Add(v, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 50)))
	edited[1][0] = v.RatString()
	drift, e := json.Marshal(bad)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJSONContext(ctx, bytes.NewReader(drift), JSONLimits{}); e == nil {
		t.Fatal("native reload accepted exact-curve drift below modeling tolerance")
	}
	edited[1][0] = old
	bad.Version = 3
	tampered, e := json.Marshal(bad)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJSONContext(ctx, bytes.NewReader(tampered), JSONLimits{}); e == nil {
		t.Fatal("older native version accepted an exact rational trim")
	}
}

func TestMultispanEllipticalContact(t *testing.T) {
	ctx := context.Background()
	// E=1-16(u-.5)^2-64(v-.5)^2 is represented on two knot spans.
	// Both original supports are independently checked along each shared arc.
	kv := []float64{0, 0, 0, 1, 1, 1}
	makeSurface := func(h float64, upper bool) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i, u := range []float64{0, .25, .5, .75, 1} {
			for j, v := range []float64{0, .5, 1} {
				z := h
				if upper {
					z += 1 - [5]float64{4, 0, 0, 0, 4}[i] - [3]float64{16, -16, 16}[j]
				}
				p = append(p, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 5, 3, p, []float64{0, 0, 0, .5, .5, 1, 1, 1}, kv, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a, b, base := makeSurface(24, true), makeSurface(24, false), makeSurface(0, false)
	hint := geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 64)}}
	s, h, e := CombineEllipticalContactSplineGraphs(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, hint, geom.GraphPointContactOptions{})
	if e != nil || h.Ellipse == nil || len(h.Ellipse.CertifiedCells) != 2 || len(h.ContourEdges) != 4 {
		t.Fatal(h, s.Topology(), e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(p.Volume-math.Pi/64) > 1e-8 {
		t.Fatal(p, e)
	}
}

func TestRotatedEllipticalSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	graph := func(c, f float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		sq := [3]float64{.25, -.25, .25}
		for i := range 3 {
			for j := range 3 {
				u, v := float64(i)/2, float64(j)/2
				// Negative of 16(dx+.5*dy)^2+16*dy²-1.
				z := c + f*(1-16*sq[i]-20*sq[j]-16*(u-.5)*(v-.5))
				p = append(p, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := graph(0, 0), graph(16, 1), graph(16, 0)
	hint := geom.GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{
		big.NewRat(12, 1), big.NewRat(-24, 1), big.NewRat(16, 1),
		big.NewRat(-28, 1), big.NewRat(20, 1), big.NewRat(16, 1),
	}}
	for _, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineQuadraticEllipseContactSplineGraphs(ctx, base, a, b, op, 1e-9, intersect.SplinePlaneOptions{}, hint, geom.GraphPointContactOptions{})
		if e != nil || h.QuadraticEllipse == nil || len(h.ContourEdges) != 4 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(s.Topology(), h, e)
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		want := math.Pi / 32
		switch op {
		case parametric.Union:
			want += 16
		case parametric.Intersection:
			want = 14 - want
		}
		if e != nil || math.Abs(m.Volume-want) > 1e-8 {
			t.Fatal(m, want, e)
		}
	}
	// The rational-point chart also produces a native lens when the
	// normalized rational conjugate controls are exactly representable.
	q, h, e := CombineRationalEllipseContactSplineGraphs(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{},
		hint, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, geom.GraphPointContactOptions{})
	if e != nil || q.Topology().EulerCharacteristic != 2 ||
		h.RationalEllipseArcs == nil || !h.RationalEllipseArcs.Complete {
		t.Fatal(q.Topology(), h, e)
	}

	ap, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bp, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	section, e := SectionRationalEllipseSplineGraphs(ctx, ap, bp, hint, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)},
		geom.GraphPointContactOptions{}, geom.SurfaceCurveAgreementOptions{})
	if e != nil || !section.UV.Complete {
		t.Fatal(section, e)
	}
	for _, owner := range section.Model {
		for i, arc := range owner {
			if !arc.Complete || len(arc.Cells) == 0 {
				t.Fatal(arc)
			}
			q, e := arc.Cells[0].EvaluateRational(ctx, new(big.Rat), geom.ExactCurvePlaneEventOptions{})
			if e != nil || q.Point[0].Cmp(section.UV.Junctions[i][0]) != 0 ||
				q.Point[1].Cmp(section.UV.Junctions[i][1]) != 0 || q.Point[2].Cmp(big.NewRat(16, 1)) != 0 {
				t.Fatal(q, e)
			}
		}
	}

	display, e := section.TessellateCertified(ctx, geom.ComposedCurvePolylineOptions{Deflection: .01})
	if e != nil || len(display.Segments) < 8 || display.MaximumError > .01 {
		t.Fatal(display, e)
	}
	count := make([]int, len(display.Vertices))
	for _, edge := range display.Segments {
		count[edge.Vertices[0]]++
		count[edge.Vertices[1]]++
	}
	for _, n := range count {
		if n != 2 {
			t.Fatal("display knot vertex is not shared by two segments", count)
		}
	}

	forced := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	if _, h, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Union, 1e-9, forced); e != nil || h.QuadraticEllipse == nil {
		t.Fatal("automatic rotated ellipse", h, e)
	}
}

func TestRationalEndpointConicBooleans(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
	graph := func(z, scale float64) *geom.BSplineSurface {
		t.Helper()
		sq := [3]float64{.25, -.25, .25}
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z + scale*(1.0/16-sq[i]-2*sq[j])})
			}
		}
		s, err := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	base, a, b := graph(0, 0), graph(2, 1), graph(2, 0)
	hint := geom.GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{
		big.NewRat(11, 16), big.NewRat(-1, 1), big.NewRat(1, 1), big.NewRat(-2, 1), big.NewRat(2, 1), new(big.Rat),
	}}
	cutY := big.NewRat(11, 20)
	line := [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat).Neg(cutY)}
	lensCut, e := SplitRationalEllipseLensByVerticalPlane(ctx, base, a, b, hint, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, line, 1e-9,
		geom.GraphPointContactOptions{}, AlgebraicSeamOptions{})
	if e != nil || lensCut.Negative.EulerCharacteristic != 2 || lensCut.Positive.EulerCharacteristic != 2 ||
		lensCut.Negative.CapReversed[0] == lensCut.Positive.CapReversed[0] {
		t.Fatal(lensCut.Negative, lensCut.Positive, e)
	}
	for _, tc := range []struct {
		side int
		y    *big.Rat
		want parametric.Location
	}{
		{-1, big.NewRat(1, 2), parametric.Inside}, {1, big.NewRat(1, 2), parametric.Outside},
		{1, big.NewRat(3, 5), parametric.Inside}, {-1, big.NewRat(3, 5), parametric.Outside},
		{-1, cutY, parametric.Boundary}, {1, cutY, parametric.Boundary},
	} {
		q, e := lensCut.LocateRational(ctx, tc.side, [3]*big.Rat{big.NewRat(1, 2), tc.y, big.NewRat(65, 32)}, geom.SurfacePatchOptions{})
		if e != nil || q != tc.want {
			t.Fatal(tc, q, e)
		}
	}
	cap, ok, e := lensCut.CapVerticalSection(ctx, big.NewRat(1, 2), geom.SurfacePatchOptions{})
	if e != nil || !ok || cap[0].Cmp(big.NewRat(2, 1)) != 0 || cap[1].Cmp(big.NewRat(823, 400)) != 0 {
		t.Fatal(cap, ok, e)
	}
	chart, e := lensCut.CapChart(-1)
	if e != nil || chart.Axis != 0 || chart.Orientation != 1 || chart.Parameter[0] == nil || chart.Parameter[1] == nil {
		t.Fatal(chart, e)
	}
	pointOnCap, e := chart.EvaluateRational(big.NewRat(1, 2), big.NewRat(65, 32))
	if e != nil || pointOnCap[1].Cmp(cutY) != 0 {
		t.Fatal(pointOnCap, e)
	}
	chart.Line[2].SetInt64(99)
	lensCut.Source = AlgebraicFaceArrangement{}
	lensCut.Contact.Ellipse.Coefficients[0].SetInt64(99)
	if q, e := lensCut.LocateRational(ctx, -1, pointOnCap, geom.SurfacePatchOptions{}); e != nil || q != parametric.Boundary {
		t.Fatal("caller mutated certified source", q, e)
	}
	if _, ok, e := lensCut.CapVerticalSection(ctx, big.NewRat(1, 5), geom.SurfacePatchOptions{}); e != nil || ok {
		t.Fatal(ok, e)
	}
	var cappedFile bytes.Buffer
	if e := WriteAlgebraicLensPlaneJSON(&cappedFile, lensCut); e != nil {
		t.Fatal(e)
	}
	capReload, e := ReadAlgebraicLensPlaneJSON(ctx, bytes.NewReader(cappedFile.Bytes()), JSONLimits{}, geom.GraphPointContactOptions{}, AlgebraicSeamOptions{})
	if e != nil || capReload.Positive != lensCut.Positive || capReload.Negative != lensCut.Negative {
		t.Fatal(capReload.Positive, capReload.Negative, e)
	}
	var capEdited nativeAlgebraicLensPlaneSplit
	if e := json.Unmarshal(cappedFile.Bytes(), &capEdited); e != nil {
		t.Fatal(e)
	}
	capEdited.Ellipse[0] = "13/16"
	capBad, e := json.Marshal(capEdited)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadAlgebraicLensPlaneJSON(ctx, bytes.NewReader(capBad), JSONLimits{}, geom.GraphPointContactOptions{}, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("incorrect material factor accepted", e)
	}
	if _, e := SplitRationalEllipseLensByVerticalPlane(ctx, base, a, b, hint, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)},
		[3]*big.Rat{big.NewRat(1, 1), new(big.Rat), big.NewRat(-3, 4)}, 1e-9, geom.GraphPointContactOptions{}, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("tangent cap accepted", e)
	}
	coarse, e := lensCut.BoundVolumes(ctx, AlgebraicLensVolumeOptions{MaxDepth: 8})
	if e != nil {
		t.Fatal(e)
	}
	fine, e := lensCut.BoundVolumes(ctx, AlgebraicLensVolumeOptions{MaxDepth: 12})
	if e != nil || !fine.ResolutionReached {
		t.Fatal(fine, e)
	}
	// Independent analytic cap segment integral for a unit disk: at
	// y'=2*sqrt(2)/10, integrate (1-r²) in horizontal strips.
	z := math.Sqrt2 / 5
	primitive := func(t float64) float64 {
		x := math.Asin(t)
		return 3*x/8 + math.Sin(2*x)/4 + math.Sin(4*x)/32
	}
	volumePositive := (4.0 / 3) * (primitive(1) - primitive(z)) / (256 * math.Sqrt2)
	for i, v := range [2]float64{math.Pi/(512*math.Sqrt2) - volumePositive, volumePositive} {
		l, _ := fine.Halves[i].Lower.Float64()
		u, _ := fine.Halves[i].Upper.Float64()
		if l <= 0 || l > v || u < v || fine.Halves[i].Lower.Cmp(coarse.Halves[i].Lower) < 0 || fine.Halves[i].Upper.Cmp(coarse.Halves[i].Upper) > 0 {
			t.Fatal(i, l, v, u, fine, coarse)
		}
	}
	if _, e := lensCut.BoundVolumes(ctx, AlgebraicLensVolumeOptions{MaxCells: 1}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
	interval := func(x *geom.ExactAlgebraicRoot, want float64) {
		t.Helper()
		p := x.Interval()
		l, _ := p.Lower.Float64()
		u, _ := p.Upper.Float64()
		if l > want+1e-12 || u < want-1e-12 {
			t.Fatal(l, want, u)
		}
	}
	lineLimits := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	clipped, e := lensCut.ClipMaterialLine(ctx, -1, geom.Vec3{X: .5, Z: 65.0 / 32}, geom.Vec3{Y: 1}, lineLimits, AlgebraicLensLineOptions{})
	if e != nil || !clipped.Complete || len(clipped.Intervals) != 1 || len(clipped.Isolated) != 0 {
		t.Fatal(clipped, e)
	}
	interval(clipped.Intervals[0].Start.Parameter, .375)
	interval(clipped.Intervals[0].End.Parameter, .55)
	if clipped.Intervals[0].Start.BoundaryMask != 1 || clipped.Intervals[0].End.BoundaryMask != 8 {
		t.Fatal(clipped)
	}
	coinciding, e := lensCut.ClipMaterialLine(ctx, -1, geom.Vec3{X: .5, Z: 2}, geom.Vec3{Y: 1}, lineLimits, AlgebraicLensLineOptions{})
	if e != nil || !coinciding.Complete || len(coinciding.Intervals) != 1 || coinciding.Intervals[0].BoundaryMask != 2 {
		t.Fatal(coinciding, e)
	}
	interval(coinciding.Intervals[0].Start.Parameter, .5-math.Sqrt2/8)
	if coinciding.Intervals[0].Start.BoundaryMask != 7 || coinciding.Intervals[0].End.BoundaryMask != 10 {
		t.Fatal(coinciding)
	}
	tangent, e := lensCut.ClipMaterialLine(ctx, -1, geom.Vec3{X: .5, Z: 33.0 / 16}, geom.Vec3{Y: 1}, lineLimits, AlgebraicLensLineOptions{})
	if e != nil || !tangent.Complete || len(tangent.Intervals) != 0 || len(tangent.Isolated) != 1 || tangent.Isolated[0].BoundaryMask != 1 {
		t.Fatal(tangent, e)
	}
	interval(tangent.Isolated[0].Parameter, .5)
	vertical, e := lensCut.ClipMaterialLine(ctx, -1, geom.Vec3{X: .5, Y: .5, Z: 3}, geom.Vec3{Z: -2}, lineLimits, AlgebraicLensLineOptions{})
	if e != nil || !vertical.Complete || len(vertical.Intervals) != 1 || vertical.Intervals[0].Start.BoundaryMask != 1 || vertical.Intervals[0].End.BoundaryMask != 2 {
		t.Fatal(vertical, e)
	}
	interval(vertical.Intervals[0].Start.Parameter, 15.0/32)
	interval(vertical.Intervals[0].End.Parameter, .5)
	want := math.Pi / (512 * math.Sqrt2)
	for _, tc := range []struct {
		a, b   *geom.BSplineSurface
		op     parametric.BooleanOp
		volume float64
	}{{a, b, parametric.Difference, want}, {b, a, parametric.Difference, 3.0/16 + want},
		{a, b, parametric.Union, 2 + want}, {a, b, parametric.Intersection, 2 - 3.0/16 - want}} {
		s, h, err := CombineRationalEllipseContactSplineGraphs(ctx, base, tc.a, tc.b, tc.op, 1e-9,
			intersect.SplinePlaneOptions{}, hint, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, geom.GraphPointContactOptions{})
		if err != nil || h.RationalEllipseArcs == nil || len(h.ContourEdges) != 4 {
			t.Fatal(tc.op, s.Topology(), h.RationalEllipseArcs, err)
		}
		p, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || math.Abs(p.Volume-tc.volume) > 2e-8 {
			t.Fatal(tc.op, p.Volume, tc.volume, err)
		}
		var file bytes.Buffer
		if err := WriteJSON(&file, s); err != nil {
			t.Fatal(err)
		}
		q, err := ReadJSONContext(ctx, &file, JSONLimits{})
		if err != nil {
			t.Fatal(err)
		}
		vertices, err := q.RationalVertices(ctx)
		found := false
		for _, v := range vertices {
			found = found || sameRationalVertex(v.Point, [3]*big.Rat{big.NewRat(5, 12), big.NewRat(2, 3), big.NewRat(2, 1)})
		}
		if err != nil || !found {
			t.Fatal("missing nonbinary conic junction after reload", vertices, err)
		}
	}
}
