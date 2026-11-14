package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestAlgebraicGraphStockRationalOccurrence(t *testing.T) {
	ctx := context.Background()
	s := makeWarpedSquareRationalGraphTube(t, 1./8)
	const z = .5 - 11./64
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}},
	}
	selection := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, e := s.ClipAlgebraicGraphCapPair(ctx, planes, selection, RationalGraphShellRegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q := big.NewRat
	matrix := [3][4]*big.Rat{
		{q(3, 5), q(0, 1), q(4, 5), q(1, 7)},
		{q(0, 1), q(1, 1), q(0, 1), q(-1, 3)},
		{q(-4, 5), q(0, 1), q(3, 5), q(1, 11)},
	}
	pose, e := placement.NewRationalRigid(matrix, 4096)
	if e != nil {
		t.Fatal(e)
	}
	root := geom.ExactCurvePlaneEventOptions{}
	w, e := pair.PlaceVerticalMaterialRigid(ctx, pose, selection, root)
	if e != nil {
		t.Fatal(e)
	}
	if p, e := w.Pose(); e != nil || p[2][3].Cmp(q(1, 11)) != 0 {
		t.Fatal("exact placed translation", p, e)
	}
	var payload bytes.Buffer
	if e := WriteAlgebraicGraphStockOccurrenceJSON(&payload, w); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), payload.Bytes()...)
	w, e = ReadAlgebraicGraphStockOccurrenceJSON(ctx, bytes.NewReader(raw), JSONLimits{}, selection,
		RationalGraphShellRegionOptions{}, root)
	if e != nil {
		t.Fatal("reload original geometry and exact pose", e)
	}
	var corrupt nativeAlgebraicGraphStockOccurrence
	if e := json.Unmarshal(raw, &corrupt); e != nil {
		t.Fatal(e)
	}
	corrupt.Pose[0][0] = "4/5"
	damaged, e := json.Marshal(corrupt)
	if e != nil {
		t.Fatal(e)
	}
	if partial, e := ReadAlgebraicGraphStockOccurrenceJSON(ctx, bytes.NewReader(damaged), JSONLimits{}, selection,
		RationalGraphShellRegionOptions{}, root); e == nil || partial.pair.caps[0].recipe != nil {
		t.Fatal("non-rigid native pose must fail atomically", e)
	}
	for _, factor := range []int64{-1, 0, 1} {
		eps := new(big.Rat).SetFrac(big.NewInt(factor), new(big.Int).Lsh(big.NewInt(1), 120))
		local := [3]*big.Rat{q(1, 2), q(7, 4), new(big.Rat).Add(q(35, 64), eps)}
		p, e := pose.TransformPoint(ctx, local, false, 4096)
		if e != nil {
			t.Fatal(e)
		}
		got, e := w.ClassifyRational(ctx, p)
		want := map[int64]parametric.Location{-1: parametric.Inside, 0: parametric.Boundary, 1: parametric.Outside}[factor]
		if e != nil || got != want {
			t.Fatal("world sub-ULP classification", factor, got, want, e)
		}
	}
	origin, e := pose.TransformPoint(ctx, [3]*big.Rat{q(0, 1), q(89, 64), q(1, 2)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	direction, e := pose.TransformVector(ctx, [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	hits, e := w.IntersectCapLineRational(ctx, 0, origin, direction, [2]*big.Rat{q(-1, 1), q(4, 1)}, root)
	if e != nil || len(hits.Contacts) != 4 || len(hits.Material) != 2 {
		t.Fatal("world exact cap line keeps its parameter and original owners", hits.Contacts, e)
	}
	proof, e := hits.Contacts[2].Parameter.SignPolynomial(ctx, []*big.Rat{q(-25, 8), q(0, 1), q(1, 1)}, root)
	if e != nil || proof.Sign != 0 {
		t.Fatal("world transport changed an irrational line parameter", proof, e)
	}
	throughWall, e := pose.TransformPoint(ctx, [3]*big.Rat{q(0, 1), q(89, 64), q(513, 1024)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	solidHits, e := w.IntersectLineRational(ctx, throughWall, direction, [2]*big.Rat{q(-1, 1), q(4, 1)}, root)
	if e != nil || len(solidHits.Contacts) != 4 || len(solidHits.Material) != 2 ||
		solidHits.Contacts[2].StockFace != w.pair.caps[0].wires[hits.Contacts[2].Wire][hits.Contacts[2].Use].Source.Face {
		t.Fatal("world line retains native stock-wall owner and both intervals", solidHits, e)
	}
	same, e := solidHits.Contacts[2].Parameter.Compare(ctx, hits.Contacts[2].Parameter, root)
	if e != nil || !same.Proven || same.Order != 0 {
		t.Fatal("world line changed its exact algebraic parameter", same, e)
	}
	options := AlgebraicGraphStockFillOptions{Deflection: .12}
	options.Boundary.Section = selection
	options.Boundary.Display.Deflection = .003
	options.Boundary.EndpointTolerance = 1e-12
	mesh, e := w.TessellateCertified(ctx, options)
	if e != nil || mesh.SignedVolume == nil || mesh.MaximumFillError > options.Deflection || len(mesh.Mesh.Triangles) != len(mesh.Source.IndexedFaces) {
		t.Fatal("closed world display and original ancestry", mesh.MaximumFillError, e)
	}
	// Bounding the exact volume is invariant under the world pose.
	width := q(1, 1<<20)
	a, e := pair.BoundVerticalPolynomialVolume(ctx, width, selection, root)
	if e != nil {
		t.Fatal(e)
	}
	b, e := w.BoundVolume(ctx, width, selection, root)
	if e != nil || a.Lower.Cmp(b.Lower) != 0 || a.Upper.Cmp(b.Upper) != 0 {
		t.Fatal("proper rational rigid placement must preserve volume bounds", a, b, e)
	}
}
