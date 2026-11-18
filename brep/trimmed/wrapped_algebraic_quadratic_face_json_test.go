//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func algebraicFaceJSONFixture(t *testing.T) (*WrappedAlgebraicQuadraticFace, []byte) {
	t.Helper()
	ctx := context.Background()
	face, e := NewWrappedAlgebraicQuadraticFace(ctx, wrappedOverlapSupport(t), wrappedAlgebraicLoop(t), nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: true})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicQuadraticFaceJSON(ctx, &out, face, WrappedQuadraticFaceJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	return face, out.Bytes()
}
func TestWrappedAlgebraicQuadraticFaceJSON(t *testing.T) {
	ctx := context.Background()
	source, data := algebraicFaceJSONFixture(t)
	restored, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(data), WrappedQuadraticFaceJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	native := restored.NativeFace()
	if !native.Face.Reversed || len(native.Edges) != 4 || len(native.Vertices) != 4 {
		t.Fatal("native topology")
	}
	open := Solid{vertices: native.Vertices, edges: native.Edges, faces: []Face{native.Face}, tolerance: 1e-9}
	proof, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal("continuous reload", e)
	}
	a, _ := source.Composition(0, 2)
	b, _ := restored.Composition(0, 2)
	same, e := a.CompareModelBounded(ctx, b, false, 64, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !same.Proven || !same.Equivalent {
		t.Fatal(same, e)
	}
	piece, _ := b.Piece(0)
	root, _ := piece.Model.EndpointRoot(1)
	kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || kind.Kind != geom.ExactRootKindIrrational {
		t.Fatal("interior root rounded", kind, e)
	}
	var encoded bytes.Buffer
	if e := WriteWrappedAlgebraicQuadraticFaceJSON(ctx, &encoded, restored, WrappedQuadraticFaceJSONOptions{}); e != nil || !bytes.Equal(data, encoded.Bytes()) {
		t.Fatal("noncanonical reload", e)
	}
	if q, e := ReadWrappedQuadraticFaceJSON(ctx, bytes.NewReader(data), WrappedQuadraticFaceJSONOptions{}); e == nil || q != nil {
		t.Fatal("legacy reader widened")
	}
	for _, o := range []WrappedQuadraticFaceJSONOptions{{MaxBytes: int64(len(data) - 1)}, {MaxControlPoints: 4}, {Face: WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{MaxOperations: 1}}}} {
		if q, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(data), o); e == nil || q != nil {
			t.Fatal("read limit")
		}
		var sink bytes.Buffer
		if e := WriteWrappedAlgebraicQuadraticFaceJSON(ctx, &sink, source, o); e == nil || sink.Len() != 0 {
			t.Fatal("non atomic write")
		}
	}
}
func TestWrappedAlgebraicQuadraticFaceJSONRejectsWitnesses(t *testing.T) {
	ctx := context.Background()
	source, data := algebraicFaceJSONFixture(t)
	mutations := []func(*wrappedAlgebraicFaceRecipe){
		func(n *wrappedAlgebraicFaceRecipe) { n.ModelVertices[0][0] = "2" },
		func(n *wrappedAlgebraicFaceRecipe) { n.ModelVertices[0] = append(n.ModelVertices[0], "0") },
		func(n *wrappedAlgebraicFaceRecipe) { n.ModelVertices[0][0] = "1e999999999" },
		func(n *wrappedAlgebraicFaceRecipe) { n.SourceOwners[0][1] = 2 },
		func(n *wrappedAlgebraicFaceRecipe) { n.SourceOwners[0][3] = 3 },
		func(n *wrappedAlgebraicFaceRecipe) { n.SourceOwners = n.SourceOwners[:len(n.SourceOwners)-1] },
		func(n *wrappedAlgebraicFaceRecipe) { n.Kind = "wrapped-quadratic-open-face" },
		func(n *wrappedAlgebraicFaceRecipe) { n.Surface.Poles[0].X += .125 },
		func(n *wrappedAlgebraicFaceRecipe) { n.Loops[0][2][1][2] = "0" },
	}
	for i, change := range mutations {
		var n wrappedAlgebraicFaceRecipe
		if e := json.Unmarshal(data, &n); e != nil {
			t.Fatal(e)
		}
		change(&n)
		raw, e := json.Marshal(n)
		if e != nil {
			t.Fatal(e)
		}
		if q, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(raw), WrappedQuadraticFaceJSONOptions{}); e == nil || q != nil {
			t.Fatal("accepted mutation", i)
		}
	}
	for _, raw := range [][]byte{append(append([]byte{}, data...), []byte(" {}")...), bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"Version":1`), 1), bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"complete":true`), 1)} {
		if q, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(raw), WrappedQuadraticFaceJSONOptions{}); e == nil || q != nil {
			t.Fatal("asserted/ambiguous JSON")
		}
	}
	// A corrupted in-memory model curve has the same endpoints but a different
	// interior. Writer must compare actual model association, not just vertices.
	copyFace := *source.face
	copyFace.native = source.NativeFace()
	copyFace.native.Edges = append([]parametric.Edge(nil), copyFace.native.Edges...)
	marker := copyFace.native.Edges[2].Curve.(wrappedAlgebraicTrimCurve)
	q := marker.curve.(ExactQuadraticUV)
	h := q.HomogeneousControls()
	h[1][1].Add(h[1][1], big.NewRat(1, 16))
	altered, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	marker.curve = altered
	copyFace.native.Edges[2].Curve = marker
	var sink bytes.Buffer
	if e := WriteWrappedAlgebraicQuadraticFaceJSON(ctx, &sink, &WrappedAlgebraicQuadraticFace{face: &copyFace}, WrappedQuadraticFaceJSONOptions{}); e == nil || sink.Len() != 0 {
		t.Fatal("changed model interior published")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := ReadWrappedAlgebraicQuadraticFaceJSON(canceled, bytes.NewReader(data), WrappedQuadraticFaceJSONOptions{}); e == nil || q != nil {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicQuadraticFaceJSONHoleOwners(t *testing.T) {
	ctx := context.Background()
	points := [][2]*big.Rat{{big.NewRat(-1, 16), big.NewRat(-3, 4)}, {big.NewRat(-1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-3, 4)}}
	var hole []ExactQuadraticUV
	for i, p := range points {
		q, e := quadraticLine(p, points[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	face, e := NewWrappedAlgebraicQuadraticFace(ctx, wrappedOverlapSupport(t), wrappedAlgebraicLoop(t), [][]ExactQuadraticUV{hole}, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicQuadraticFaceJSON(ctx, &out, face, WrappedQuadraticFaceJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(out.Bytes()), WrappedQuadraticFaceJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	native := got.NativeFace()
	if len(native.Face.Holes) != 1 || len(native.Edges) != 8 {
		t.Fatal("lost hole")
	}
	var recipe wrappedAlgebraicFaceRecipe
	if e := json.Unmarshal(out.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	if recipe.SourceOwners[7][0] != 1 || recipe.SourceOwners[7][1] != 3 || recipe.SourceOwners[7][3] != 4 {
		t.Fatal("lost original hole owner")
	}
	// Every source loop is rebuilt; the final hole edge closes on its own base.
	recipe.SourceOwners[7][3] = 0
	raw, _ := json.Marshal(recipe)
	if r, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(raw), WrappedQuadraticFaceJSONOptions{}); e == nil || r != nil {
		t.Fatal("cross-loop mate admitted")
	}
}
