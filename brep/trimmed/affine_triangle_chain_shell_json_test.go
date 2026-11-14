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

func TestAffineTriangleChainShellJSON(t *testing.T) {
	ctx := context.Background()
	ab, bc, outer := triangleChainFixture(t)
	s, e := NewAffineTriangleChainShell(ctx, ab, bc, outer, AffineTriangleShellOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteAffineTriangleChainShellJSON(ctx, &buf, s, AffineTriangleChainJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), buf.Bytes()...)
	restored, e := ReadAffineTriangleChainShellJSON(ctx, bytes.NewReader(raw), AffineTriangleChainJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	solid := Solid{vertices: restored.Vertices(), edges: restored.Edges(), faces: restored.Faces(), tolerance: 1e-9}
	r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	owners, e := restored.VertexSources(0)
	if e != nil || len(owners) != 3 || len(restored.Boundary()) != 5 {
		t.Fatal("reproved fan and exterior")
	}
	var again bytes.Buffer
	if e = WriteAffineTriangleChainShellJSON(ctx, &again, restored, AffineTriangleChainJSONOptions{}); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("canonical roundtrip", e)
	}
	for _, mutate := range []func(*affineTriangleChainRecipe){
		func(n *affineTriangleChainRecipe) { n.Vertices[0] = 1 },
		func(n *affineTriangleChainRecipe) {
			n.Outer.Triangles[0][0], n.Outer.Triangles[0][1] = n.Outer.Triangles[0][1], n.Outer.Triangles[0][0]
		},
		func(n *affineTriangleChainRecipe) { n.Point[0] = "1/4" },
		func(n *affineTriangleChainRecipe) { n.Witness.Vertices[0].Sources[2][1] = 1 },
		func(n *affineTriangleChainRecipe) { n.Witness.Boundary[0][0] = 2 },
		func(n *affineTriangleChainRecipe) { n.Mates[1].Normal[0] = "2" },
		func(n *affineTriangleChainRecipe) {
			for i, h := range n.Patches[2].Controls {
				z, _ := new(big.Rat).SetString(h[2])
				w, _ := new(big.Rat).SetString(h[3])
				n.Patches[2].Controls[i][2] = z.Add(z, new(big.Rat).Mul(w, big.NewRat(1, 64))).RatString()
			}
		},
	} {
		var n affineTriangleChainRecipe
		if e = json.Unmarshal(raw, &n); e != nil {
			t.Fatal(e)
		}
		mutate(&n)
		// Every mutation retains independently valid source tensor charts: rejection
		// must come from contact/binding/incidence, not malformed support encoding.
		for _, p := range n.Patches {
			if _, e := geom.ImportExactTensorSurfaceRecipe(ctx, p, geom.SurfacePatchOptions{}); e != nil {
				t.Fatal("invalid local fixture", e)
			}
		}
		bad, _ := json.Marshal(n)
		if got, e := ReadAffineTriangleChainShellJSON(ctx, bytes.NewReader(bad), AffineTriangleChainJSONOptions{}); e == nil || got != nil {
			t.Fatal("accepted altered shared provenance")
		}
	}
	for _, o := range []AffineTriangleChainJSONOptions{{MaxWork: 1}, {MaxWork: -1}, {MaxBits: 63}, {MaxBytes: int64(len(raw) - 1)}, {MaxPoles: 1}, {Contact: geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}}, {Shell: AffineTriangleShellOptions{Proof: edgecheck.Options{MaxIntervals: 1}}}} {
		if got, e := ReadAffineTriangleChainShellJSON(ctx, bytes.NewReader(raw), o); e == nil || got != nil {
			t.Fatal("read limit")
		}
		var denied bytes.Buffer
		if e := WriteAffineTriangleChainShellJSON(ctx, &denied, s, o); e == nil || denied.Len() != 0 {
			t.Fatal("atomic write limit")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := ReadAffineTriangleChainShellJSON(canceled, bytes.NewReader(raw), AffineTriangleChainJSONOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
	s.graph.faces[1].Outer[0].UVEnd.X += .125
	var denied bytes.Buffer
	if e := WriteAffineTriangleChainShellJSON(ctx, &denied, s, AffineTriangleChainJSONOptions{}); e == nil || denied.Len() != 0 {
		t.Fatal("stale native use")
	}
}
