//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedShellJSONFixture(t *testing.T) (*WrappedAlgebraicOpenShell, []byte) {
	t.Helper()
	a, b := wrappedAlgebraicMateFaces(t, false, false)
	s, e := NewWrappedAlgebraicOpenShell(context.Background(), a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicOpenShellJSON(context.Background(), &out, s, WrappedAlgebraicOpenShellJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	return s, out.Bytes()
}
func TestWrappedAlgebraicOpenShellJSON(t *testing.T) {
	ctx := context.Background()
	source, data := wrappedShellJSONFixture(t)
	restored, e := ReadWrappedAlgebraicOpenShellJSON(ctx, bytes.NewReader(data), WrappedAlgebraicOpenShellJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(restored.Vertices()) != 6 || len(restored.Edges()) != 7 || len(restored.Boundary()) != 6 {
		t.Fatal("incidence")
	}
	native := Solid{vertices: restored.Vertices(), edges: restored.Edges(), faces: restored.Faces(), tolerance: 1e-9}
	agreement, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete {
		t.Fatal(agreement, e)
	}
	owner, _ := restored.Pair().Owner(1)
	if owner.UV[0][0].Cmp(big.NewRat(15, 4)) != 0 {
		t.Fatal("lost authored lift")
	}
	proof, _ := restored.Pair().Composition(0)
	piece, _ := proof.Piece(0)
	root, _ := piece.Model.EndpointRoot(1)
	kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || kind.Kind != geom.ExactRootKindIrrational {
		t.Fatal("rounded root", kind, e)
	}
	var again bytes.Buffer
	if e := WriteWrappedAlgebraicOpenShellJSON(ctx, &again, restored, WrappedAlgebraicOpenShellJSONOptions{}); e != nil || !bytes.Equal(data, again.Bytes()) {
		t.Fatal("deterministic reload", e)
	}
	for _, o := range []WrappedAlgebraicOpenShellJSONOptions{{MaxBytes: int64(len(data) - 1)}, {Face: WrappedQuadraticFaceJSONOptions{MaxBytes: 1}}, {Assembly: edgecheck.Options{MaxWork: 1}}, {Assembly: edgecheck.Options{MaxIntervals: 1}}} {
		if s, e := ReadWrappedAlgebraicOpenShellJSON(ctx, bytes.NewReader(data), o); e == nil || s != nil {
			t.Fatal("read bound")
		}
		var dst bytes.Buffer
		if e := WriteWrappedAlgebraicOpenShellJSON(ctx, &dst, source, o); e == nil || dst.Len() != 0 {
			t.Fatal("write atomicity")
		}
	}
}
func TestWrappedAlgebraicOpenShellJSONRejects(t *testing.T) {
	ctx := context.Background()
	source, data := wrappedShellJSONFixture(t)
	changes := []func(*wrappedAlgebraicShellRecipe){
		func(n *wrappedAlgebraicShellRecipe) { n.ReverseB = false },
		func(n *wrappedAlgebraicShellRecipe) { n.Mate[0][1] = 1 },
		func(n *wrappedAlgebraicShellRecipe) { n.Witness.Vertices[0].Point[0] = "2" },
		func(n *wrappedAlgebraicShellRecipe) { n.Witness.Vertices[2].Sources[1][1] = 0 },
		func(n *wrappedAlgebraicShellRecipe) { n.Witness.Edges[2].Sources[1][4] = 0 },
		func(n *wrappedAlgebraicShellRecipe) { n.Witness.Edges[2].Vertices[1] = 0 },
		func(n *wrappedAlgebraicShellRecipe) { n.Witness.FaceUses[1][0][1] = 0 },
		func(n *wrappedAlgebraicShellRecipe) { n.Witness.Boundary[0][0] = 1 },
		func(n *wrappedAlgebraicShellRecipe) { n.Witness.Boundary = n.Witness.Boundary[:5] },
		func(n *wrappedAlgebraicShellRecipe) {
			var f wrappedAlgebraicFaceRecipe
			_ = json.Unmarshal(n.Faces[1], &f)
			f.Reversed = !f.Reversed
			n.Faces[1], _ = json.Marshal(f)
		},
		// The altered source face still has the same vertices, but its shared model
		// curve differs. Full reconstruction must reject the stored mate.
		func(n *wrappedAlgebraicShellRecipe) {
			var f wrappedAlgebraicFaceRecipe
			_ = json.Unmarshal(n.Faces[1], &f)
			f.Loops[0][0][1][1] = "17/16"
			n.Faces[1], _ = json.Marshal(f)
		},
	}
	for i, change := range changes {
		var n wrappedAlgebraicShellRecipe
		if e := json.Unmarshal(data, &n); e != nil {
			t.Fatal(e)
		}
		change(&n)
		raw, _ := json.Marshal(n)
		if s, e := ReadWrappedAlgebraicOpenShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicOpenShellJSONOptions{}); e == nil || s != nil {
			t.Fatal("accepted mutation", i)
		}
	}
	for _, raw := range [][]byte{append(append([]byte{}, data...), []byte(" {}")...), bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"Version":1`), 1), bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"complete":true`), 1)} {
		if s, e := ReadWrappedAlgebraicOpenShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicOpenShellJSONOptions{}); e == nil || s != nil {
			t.Fatal("ambiguous or asserted proof")
		}
	}
	corrupted := *source
	corrupted.edges = source.Edges()
	marker := corrupted.edges[2].Curve.(wrappedAlgebraicTrimCurve)
	q := marker.curve.(ExactQuadraticUV)
	h := q.HomogeneousControls()
	h[1][1].Add(h[1][1], big.NewRat(1, 16))
	altered, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	marker.curve = altered
	corrupted.edges[2].Curve = marker
	var dst bytes.Buffer
	if e := WriteWrappedAlgebraicOpenShellJSON(ctx, &dst, &corrupted, WrappedAlgebraicOpenShellJSONOptions{}); e == nil || dst.Len() != 0 {
		t.Fatal("stale source published")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if s, e := ReadWrappedAlgebraicOpenShellJSON(canceled, bytes.NewReader(data), WrappedAlgebraicOpenShellJSONOptions{}); e == nil || s != nil {
		t.Fatal("cancel")
	}
}
