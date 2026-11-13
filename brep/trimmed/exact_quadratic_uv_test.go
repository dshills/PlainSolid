package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExactQuadraticUVModelComposition(t *testing.T) {
	ctx := context.Background()
	r := func(x int64) *big.Rat { return big.NewRat(x, 1) }
	h := [3][3]*big.Rat{
		{r(0), r(0), r(1)},
		{big.NewRat(1, 7), big.NewRat(1, 3), big.NewRat(2, 3)},
		{r(1), r(1), r(1)},
	}
	q, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	h[1][0].SetInt64(99)
	copied := q.HomogeneousControls()
	copied[1][1].SetInt64(99)
	u, derivative, e := q.EvaluateRational(big.NewRat(1, 2))
	if e != nil || u[0].Cmp(big.NewRat(27, 70)) != 0 || u[1].Cmp(big.NewRat(1, 2)) != 0 ||
		derivative[0].Cmp(big.NewRat(6, 5)) != 0 || derivative[1].Cmp(big.NewRat(6, 5)) != 0 {
		t.Fatal(u, derivative, e)
	}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}},
		[]float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	o := geom.SurfacePatchOptions{}
	p, e := geom.NewSurfacePatch(s, o)
	if e != nil {
		t.Fatal(e)
	}
	model, e := q.ModelCurve(ctx, p, o)
	if e != nil {
		t.Fatal(e)
	}
	at, e := model.Eval(.5)
	if e != nil || math.Abs(at.X-27.0/70) > 1e-15 || at.Y != .5 || math.Abs(at.Z-27.0/140) > 1e-15 {
		t.Fatal(at, e)
	}
	lo, hi, e := outwardTrimRational(big.NewRat(1, 3))
	if e != nil || ratInset(lo).Cmp(big.NewRat(1, 3)) >= 0 || ratInset(hi).Cmp(big.NewRat(1, 3)) <= 0 {
		t.Fatal(lo, hi, e)
	}
	uv, box, ok, e := splineDistanceTrimWitness(ctx, p, Use{Curve: q}, .5)
	if e != nil || !ok || ratInset(uv.Min.X).Cmp(u[0]) > 0 || ratInset(uv.Max.X).Cmp(u[0]) < 0 ||
		ratInset(box.Min.Z).Cmp(big.NewRat(27, 140)) > 0 || ratInset(box.Max.Z).Cmp(big.NewRat(27, 140)) < 0 {
		t.Fatal(uv, box, e)
	}
	section, e := q.IntersectExactQuadraticTrimPlane(ctx, p,
		[3]geom.Vec3{{Y: .5}, {X: 1, Y: .5}, {Y: .5, Z: 1}}, geom.SplinePlaneOptions{}, geom.SurfaceCurveCompositionOptions{})
	if e != nil || !section.Complete || len(section.Hits) != 1 || !section.Hits[0].Crossing ||
		section.Hits[0].Parameter.Min.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(section, e)
	}
	p, e = p.InsertUKnot(.5, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	// The exact composition reports a crossing of u=.5 at an
	// irrational parameter without rounding it into a rational cut.
	pieces, e := q.ModelCurvePieces(ctx, p, geom.SurfaceCurveCompositionOptions{})
	if !errors.Is(e, geom.ErrConvergence) || pieces.Complete || len(pieces.Unresolved) != 1 {
		t.Fatal(pieces, e)
	}
	cells, e := q.ModelCurveCells(ctx, p, geom.SurfaceCurveAgreementOptions{})
	if e != nil || !cells.Complete || len(cells.Cells) != 2 {
		t.Fatal(cells, e)
	}
	atExact, e := cells.Cells[0].EvaluateRational(ctx, big.NewRat(1, 2), geom.ExactCurvePlaneEventOptions{})
	if e != nil || atExact.Point[0].Cmp(big.NewRat(27, 70)) != 0 || atExact.Point[2].Cmp(big.NewRat(27, 140)) != 0 {
		t.Fatal(atExact, e)
	}
	junctions, e := q.EncloseModelKnotJunctions(ctx, p, geom.SurfaceCurveAgreementOptions{},
		geom.ComposedCurveEndpointOptions{Tolerance: 1e-12})
	if e != nil || len(junctions) != 1 || junctions[0].ErrorUpper > 1e-12 ||
		math.Abs(junctions[0].Point.X-.5) > 1e-12 || junctions[0].Left.Parameter.Lower.Cmp(junctions[0].Left.Parameter.Upper) == 0 {
		t.Fatal(junctions, e)
	}

	planeCut, e := q.IntersectModelRawPlane(ctx, p, [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)},
		geom.SurfaceCurveAgreementOptions{}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(planeCut.Hits) != 1 || len(planeCut.Hits[0].Cells) != 2 ||
		planeCut.Hits[0].Event.Interval().Lower.Cmp(planeCut.Hits[0].Event.Interval().Upper) == 0 {
		t.Fatal(planeCut, e)
	}
	lineCut, e := q.IntersectModelFiniteLine(ctx, p, geom.Vec3{X: .5}, geom.Vec3{Y: 1, Z: .5},
		[2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 1)}, geom.SurfaceCurveAgreementOptions{}, geom.ComposedCurveLineOptions{})
	if e != nil || len(lineCut.Hits) != 1 || len(lineCut.Hits[0].Cells) != 2 || lineCut.Hits[0].BoundaryMask != 0 {
		t.Fatal(lineCut, e)
	}
	point, e := lineCut.Hits[0].Event.Enclose(ctx, geom.ComposedCurveEndpointOptions{})
	if e != nil || math.Abs(point.Point.X-.5) > 1e-9 || math.Abs(point.Point.Z-point.Point.Y/2) > 1e-9 {
		t.Fatal(point, e)
	}

	mesh, e := q.TessellateModelCertified(ctx, p, geom.SurfaceCurveAgreementOptions{}, geom.ComposedCurvePolylineOptions{Deflection: .002})
	if e != nil || mesh.MaximumError > .002 || len(mesh.Segments)+1 != len(mesh.Vertices) {
		t.Fatal(mesh, e)
	}
	count := 0
	for i, v := range mesh.Vertices {
		if v.Algebraic {
			count++
			if i == 0 || i == len(mesh.Vertices)-1 || math.Abs(v.Point.X-.5) > .002 ||
				mesh.Segments[i-1].Cell != 0 || mesh.Segments[i].Cell != 1 {
				t.Fatal(i, v)
			}
		}
	}
	if count != 1 {
		t.Fatal("irrational knot must use exactly one shared display vertex", count)
	}

}

func TestSplitExactQuadraticTrimEdgeRepresentedPoint(t *testing.T) {
	ctx := context.Background()
	// Two graph supports have the same curved boundary z=1+uv where
	// v=u², and differ by a strictly positive gap inside that curve.
	k := []float64{0, 0, 0, 1, 1, 1}
	makeGraph := func(top bool) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				u, v := float64(i)/2, float64(j)/2
				z := 1 + u*v
				if top {
					z += v
					if i == 2 {
						z--
					}
				}
				p = append(p, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}

	a := makeGraph(true)
	b := makeGraph(false)
	dyadic, e := NewExactQuadraticUV([3][3]*big.Rat{
		{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)},
		{big.NewRat(1, 2), big.NewRat(0, 1), big.NewRat(1, 1)},
		{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)},
	})
	if e != nil {
		t.Fatal(e)
	}
	// Only the first edge is split; assemble a narrow graph shell with a
	// curved lower boundary and vertical walls along the original chart.
	loop := []openSectionPiece{
		{a: geom.Vec2{}, b: geom.Vec2{X: 1, Y: 1}, c: dyadic, side: -1},
		{a: geom.Vec2{X: 1, Y: 1}, b: geom.Vec2{Y: 1}, side: 2},
		{a: geom.Vec2{Y: 1}, b: geom.Vec2{}, side: 3},
	}
	s, _, e := assembleOpenSplineGraphPair(a, b, 0, true, 1e-9, loop)
	if e != nil {
		t.Fatal(e)
	}
	seam, e := s.IntersectExactSplineSeamLine(ctx, 0, geom.Vec3{X: .5, Z: 1}, geom.Vec3{Y: 1, Z: .5},
		[2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.SurfaceCurveAgreementOptions{}, geom.ComposedCurveLineOptions{})
	if e != nil || seam.Faces != ([2]int{0, 1}) || len(seam.Section.Hits) != 1 || seam.HitVertex[0] != -1 ||
		seam.Section.Hits[0].LineParameter.Lower.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal("shared seam contact", seam, e)
	}
	seam, e = s.IntersectExactSplineSeamLine(ctx, 0, geom.Vec3{}, geom.Vec3{Z: 1},
		[2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.SurfaceCurveAgreementOptions{}, geom.ComposedCurveLineOptions{})
	if e != nil || len(seam.Section.Hits) != 1 || seam.HitVertex[0] != s.edges[0].Start || seam.Section.Hits[0].BoundaryMask != 2 {
		t.Fatal("shared seam endpoint", seam, e)
	}
	patch, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane := [4]*big.Rat{nil, big.NewRat(1, 1), nil, big.NewRat(-1, 2)}
	section, e := dyadic.IntersectModelRawPlane(ctx, patch, plane, geom.SurfaceCurveAgreementOptions{}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(section.Hits) != 1 {
		t.Fatal(section, e)
	}
	recipe, proof, e := s.PrepareAlgebraicSeamRecipe(ctx, 0, plane, section.Hits[0].Event, AlgebraicSeamOptions{})
	if e != nil || proof.Faces != ([2]int{0, 1}) || !proof.Model.Converged || math.Abs(proof.Model.Point.X-1/math.Sqrt2) > 1e-9 {
		t.Fatal(recipe, proof, e)
	}
	lineEnds := [2][3]*big.Rat{{big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(3, 2)},
		{new(big.Rat), big.NewRat(1, 2), big.NewRat(1, 1)}}
	lineControls := make([][4]*big.Rat, 2)
	for i, row := range lineEnds {
		lineControls[i] = [4]*big.Rat{row[0], row[1], row[2], big.NewRat(1, 1)}
	}
	affine, e := geom.NewExactHomogeneousBezier(ctx, lineControls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	composed, e := dyadic.ModelCurveCells(ctx, patch, geom.SurfaceCurveAgreementOptions{})
	if e != nil || len(composed.Cells) != 1 {
		t.Fatal(composed, e)
	}
	pair, e := geom.IntersectExactComposedCurveAffineEdge(ctx, composed.Cells[0], affine, geom.ComposedCurveLineOptions{})
	if e != nil || !pair.Complete || len(pair.Hits) != 1 {
		t.Fatal(pair, e)
	}
	affineRecipe, paired, e := s.PrepareAlgebraicAffineSeamRecipe(ctx, 0, lineEnds, pair.Hits[0], AlgebraicSeamOptions{})
	if e != nil || affineRecipe.Version != 3 || paired.PairedParameter == nil {
		t.Fatal(affineRecipe, paired, e)
	}
	pairedInterval := paired.PairedParameter.Interval()
	v, _ := pairedInterval.Lower.Float64()
	w, _ := pairedInterval.Upper.Float64()
	if v > 1-1/math.Sqrt2 || w < 1-1/math.Sqrt2 {
		t.Fatal(pairedInterval)
	}
	badPaired := affineRecipe
	badPaired.Paired = &affineRecipe.Parameter
	if _, e := s.VerifyAlgebraicSeamRecipe(ctx, badPaired, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("mismatched second edge root accepted", e)
	}
	data, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	var decoded AlgebraicSeamRecipe
	if e := json.Unmarshal(data, &decoded); e != nil {
		t.Fatal(e)
	}
	reload, e := s.VerifyAlgebraicSeamRecipe(ctx, decoded, AlgebraicSeamOptions{})
	if e != nil || reload.Model.Point != proof.Model.Point {
		t.Fatal(reload, e)
	}
	decoded.Plane[3] = new(big.Rat).Add(big.NewRat(-1, 2), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))).RatString()
	if _, e := s.VerifyAlgebraicSeamRecipe(ctx, decoded, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("distinct nearby plane event accepted", e)
	}
	closePlane := plane
	closePlane[3], _ = new(big.Rat).SetString(decoded.Plane[3])
	closeSection, e := dyadic.IntersectModelRawPlane(ctx, patch, closePlane, geom.SurfaceCurveAgreementOptions{}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(closeSection.Hits) != 1 {
		t.Fatal(closeSection, e)
	}
	near, _, e := s.PrepareAlgebraicSeamRecipe(ctx, 0, closePlane, closeSection.Hits[0].Event, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	subdivision, e := s.SubdivideAlgebraicSeams(ctx, []AlgebraicSeamRecipe{recipe, near}, AlgebraicSeamOptions{})
	if e != nil || len(subdivision.Edges()) != len(s.edges)+2 || len(subdivision.Vertices()) != len(s.vertices)+2 {
		t.Fatal(subdivision.Edges(), subdivision.Vertices(), e)
	}
	verifySubdivision := func(q AlgebraicSubdivision) {
		t.Helper()
		edges, vertices := q.Edges(), q.Vertices()
		middle := edges[1]
		if middle.Original != 0 || middle.Start == middle.End || vertices[middle.Start].Recipe != 1 || vertices[middle.End].Recipe != 0 ||
			edges[0].End != middle.Start || edges[2].Start != middle.End {
			t.Fatal("root order lost or distinct vertices welded", edges[:3], vertices)
		}
		cmp, err := middle.Lower.Compare(ctx, middle.Upper, geom.ExactCurvePlaneEventOptions{})
		if err != nil || !cmp.Proven || cmp.Order != -1 || cmp.Intervals[0][1].Cmp(cmp.Intervals[1][0]) >= 0 {
			t.Fatal(cmp, err)
		}
		m, err := q.ModelCurveCells(ctx, 1, AlgebraicSeamOptions{})
		if err != nil || !m.Complete || len(m.Cells) != 1 {
			t.Fatal(m, err)
		}
		display, err := q.TessellateModelEdgeCertified(ctx, 1, AlgebraicSeamOptions{}, geom.ComposedCollectionPolylineOptions{
			ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: 1e-7},
		})
		if err != nil || !display.Complete || !display.DisplayDegenerate || len(display.CollapsedSegments) != 1 ||
			len(display.Junctions) != 2 || display.Junctions[0].Vertex == display.Junctions[1].Vertex {
			t.Fatal(display, err)
		}
		for face := 0; face < 2; face++ {
			chart, err := q.CoedgeChart(ctx, 1, face, AlgebraicSeamOptions{})
			if err != nil || chart.UV == nil || !chart.Model.Complete {
				t.Fatal(chart, err)
			}
			at, err := chart.UV.EncloseEndpoint(ctx, 1, geom.ComposedCurveEndpointOptions{})
			if err != nil || !at.Converged || math.Abs(at.Point.X-1/math.Sqrt2) > 1e-9 || math.Abs(at.Point.Y-.5) > 1e-9 {
				t.Fatal(at, err)
			}
		}
	}
	verifySubdivision(subdivision)
	var file bytes.Buffer
	if e := WriteAlgebraicSubdivisionJSON(&file, subdivision); e != nil {
		t.Fatal(e)
	}
	restoredSubdivision, e := ReadAlgebraicSubdivisionJSON(ctx, bytes.NewReader(file.Bytes()), JSONLimits{}, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	verifySubdivision(restoredSubdivision)
	if _, e := s.SubdivideAlgebraicSeams(ctx, []AlgebraicSeamRecipe{recipe, recipe}, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("duplicate algebraic split accepted", e)
	}
	// The shared model seam is (t,t²,1+t³). A second solid's upper
	// support z=9/8 meets it once at t=.5. Cutting a circular hole in that
	// face removes the root by exact trim membership, without losing the
	// supporting-surface evidence or either original source owner.
	makeLevel := func(z float64) *geom.BSplineSurface {
		knots := []float64{0, 0, 1, 1}
		p, e := geom.NewBSplineSurface(1, 1, 2, 2,
			[]geom.Vec3{{Z: z}, {Y: 1, Z: z}, {X: 1, Z: z}, {X: 1, Y: 1, Z: z}}, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	other, _, e := assembleOpenSplineGraphPair(makeLevel(9.0/8), makeLevel(0), 0, true, 1e-9,
		partitionGraphRectangleLoop(geom.ParameterRectangle{UMax: 1, VMax: 1}))
	if e != nil {
		t.Fatal(e)
	}
	contact, e := IntersectSplineSeamFace(ctx, s, 0, other, 0, SplineSeamFaceOptions{})
	if e != nil || !contact.Complete || contact.SourceFaces != ([2]int{0, 1}) || len(contact.Hits) != 1 ||
		contact.Hits[0].CurveParameter.Min > .5 || contact.Hits[0].CurveParameter.Max < .5 {
		t.Fatal(contact, e)
	}
	graphContact, e := IntersectSplineSeamFace(ctx, s, 0, other, 0, SplineSeamFaceOptions{EnableIdentityGraph: true})
	if e != nil || !graphContact.Complete || graphContact.Graph == nil || len(graphContact.Graph.Hits) != 1 ||
		len(graphContact.Graph.Hits[0].SourceCells) != 1 || graphContact.Graph.Hits[0].SourceCells[0] != 0 ||
		graphContact.Graph.Hits[0].EdgeParameter.Lower.Cmp(big.NewRat(1, 2)) != 0 || graphContact.Graph.Hits[0].Signs != ([2]int{-1, 1}) {
		t.Fatal(graphContact, e)
	}
	graphRecipe, graphProof, e := s.PrepareAlgebraicGraphSeamRecipe(ctx, 0, other.faces[0].Surface,
		graphContact.Graph.Hits[0].Event, AlgebraicSeamOptions{})
	if e != nil || graphRecipe.Version != 2 || graphProof.Faces != ([2]int{0, 1}) || graphProof.Model.Point != (geom.Vec3{X: .5, Y: .25, Z: 1.125}) {
		t.Fatal(graphRecipe.Version, graphProof, e)
	}
	combined, e := s.SubdivideAlgebraicSeams(ctx, []AlgebraicSeamRecipe{recipe, graphRecipe}, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var combinedFile bytes.Buffer
	if e := WriteAlgebraicSubdivisionJSON(&combinedFile, combined); e != nil {
		t.Fatal(e)
	}
	graphReload, e := ReadAlgebraicSubdivisionJSON(ctx, &combinedFile, JSONLimits{}, AlgebraicSeamOptions{})
	if e != nil || len(graphReload.Edges()) != len(s.edges)+2 || graphReload.Vertices()[len(s.vertices)].Recipe != 1 {
		t.Fatal(graphReload.Vertices(), e)
	}
	graphTamper := graphReload.Recipes()[1]
	graphTamper.Graph, e = parametric.EncodeSurface(makeLevel(9.0/8 + 1.0/1024))
	if e != nil {
		t.Fatal(e)
	}
	if _, e := s.VerifyAlgebraicSeamRecipe(ctx, graphTamper, AlgebraicSeamOptions{}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("graph drift accepted", e)
	}
	hole, e := ellipseGraphContactLoop(geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 4)},
		RadiusSquared: [2]*big.Rat{big.NewRat(1, 256), big.NewRat(1, 256)}})
	if e != nil {
		t.Fatal(e)
	}
	var holeCurves []PCurve
	for _, x := range hole {
		holeCurves = append(holeCurves, x.c)
	}
	other, faceSplit, e := other.SplitFaceCurvedLoopContext(ctx, 0, holeCurves)
	if e != nil {
		t.Fatal(e)
	}
	contact, e = IntersectSplineSeamFace(ctx, s, 0, other, faceSplit.Outer, SplineSeamFaceOptions{})
	if e != nil || !contact.Complete || len(contact.Hits) != 0 || len(contact.Excluded) != 1 {
		t.Fatal(contact, e)
	}
	q, h, e := s.SplitExactQuadraticTrimEdge(ctx, 0, big.NewRat(1, 2), edgecheck.Options{})
	if e != nil || q.Topology().Edges != s.Topology().Edges+1 || q.Topology().EulerCharacteristic != s.Topology().EulerCharacteristic ||
		q.vertices[h.Vertex] != (geom.Vec3{X: .5, Y: .25, Z: 1.125}) {
		t.Fatal(q.Topology(), h, e)
	}
	if _, _, e := q.MergeExactQuadraticTrimEdges(ctx, h.First, h.Second, big.NewRat(1, 3)); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("wrong split ratio did not reject", e)
	}
	merged, m, e := q.MergeExactQuadraticTrimEdges(ctx, h.First, h.Second, big.NewRat(1, 2))
	if e != nil || merged.Topology().Edges != s.Topology().Edges || m.Vertices[h.Vertex] != -1 ||
		m.Edges[h.First] != m.Edge || m.Edges[h.Second] != m.Edge {
		t.Fatal(merged.Topology(), m, e)
	}
	r := merged.edges[m.Edge].Curve.(surfaceTrimCurve).curve.(ExactQuadraticUV)
	x, y := dyadic.HomogeneousControls(), r.HomogeneousControls()
	for i := range x {
		for j := range x[i] {
			if x[i][j].Cmp(y[i][j]) != 0 {
				t.Fatal("homogeneous controls changed", x, y)
			}
		}
	}

}

func TestExactQuadraticEdgeAgreementAcrossIrrationalKnot(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e := p.InsertVKnot(.5, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	other, e := refined.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	q, e := NewExactQuadraticUV([3][3]*big.Rat{
		{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)},
		{big.NewRat(1, 2), big.NewRat(0, 1), big.NewRat(1, 1)},
		{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)},
	})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := q.ModelCurvePieces(ctx, refined, geom.SurfaceCurveCompositionOptions{}); !errors.Is(e, geom.ErrConvergence) || len(r.Unresolved) != 1 {
		t.Fatal(r, e)
	}
	model := parametric.Edge{Curve: surfaceTrimCurve{s, q.ends[0], q.ends[1], q}, Range: parametric.Interval{Max: 1}}
	use := Use{Curve: q, UVStart: q.ends[0], UVEnd: q.ends[1]}
	if ok, e := compareSplineEdgeUse(ctx, model, other, use, edgecheck.Options{}); e != nil || !ok {
		t.Fatal("continuous identity does not require constructing rational pieces", ok, e)
	}
	use.Curve = q.reversed()
	use.UVStart, use.UVEnd, use.Reversed = use.UVEnd, use.UVStart, true
	if ok, e := compareSplineEdgeUse(ctx, model, other, use, edgecheck.Options{}); e != nil || !ok {
		t.Fatal("opposite incidence lost an algebraic cut", ok, e)
	}
}
