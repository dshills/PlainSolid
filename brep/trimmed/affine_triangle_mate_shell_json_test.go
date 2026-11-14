package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestAffineTriangleMateShellJSON(t *testing.T) {
	ctx := context.Background()
	contact := triangleShellContactConfigured(t, false, false, true)
	s, e := NewAffineTriangleMateShell(ctx, contact, AffineTriangleShellOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteAffineTriangleMateShellJSON(ctx, &buf, s, AffineTriangleShellJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), buf.Bytes()...)
	restored, e := ReadAffineTriangleMateShellJSON(ctx, bytes.NewReader(raw), AffineTriangleShellJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	solid := Solid{vertices: restored.Vertices(), edges: restored.Edges(), faces: restored.Faces(), tolerance: 1e-9}
	r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	p, _, _ := restored.Contact().Source(1)
	old, _, _ := contact.Source(1)
	same, e := geom.CompareExactSurfaces(ctx, p, old, geom.ExactSurfaceComparisonOptions{})
	if e != nil || !same.Equal {
		t.Fatal("exact nonbinary source reload", same, e)
	}
	var again bytes.Buffer
	if e = WriteAffineTriangleMateShellJSON(ctx, &again, restored, AffineTriangleShellJSONOptions{}); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("deterministic recipe", e)
	}
	for _, mutate := range []func(*affineTriangleShellRecipe){
		func(n *affineTriangleShellRecipe) { n.Normal[0] = "2" }, func(n *affineTriangleShellRecipe) { n.Triangles[1][0][0] = "1/4" }, func(n *affineTriangleShellRecipe) { n.Patches[1].Controls[1][0] = "99" }, func(n *affineTriangleShellRecipe) { n.Patches[1].KnotsU[0] = .125 }, func(n *affineTriangleShellRecipe) { n.Witness.Edges[0].Sources[0][0] = 1 }, func(n *affineTriangleShellRecipe) { n.Projection[0] = [2]int{1, 1} },
	} {
		var n affineTriangleShellRecipe
		if e = json.Unmarshal(raw, &n); e != nil {
			t.Fatal(e)
		}
		mutate(&n)
		bad, _ := json.Marshal(n)
		if got, e := ReadAffineTriangleMateShellJSON(ctx, bytes.NewReader(bad), AffineTriangleShellJSONOptions{}); e == nil || got != nil {
			t.Fatal("accepted mutation")
		}
	}
	for _, o := range []AffineTriangleShellJSONOptions{{MaxBytes: int64(len(raw) - 1)}, {MaxWork: 1}, {MaxPoles: 1}, {Contact: geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}}, {Shell: AffineTriangleShellOptions{Proof: edgecheck.Options{MaxWork: 1}}}} {
		if got, e := ReadAffineTriangleMateShellJSON(ctx, bytes.NewReader(raw), o); e == nil || got != nil {
			t.Fatal("read limits")
		}
		var denied bytes.Buffer
		if e := WriteAffineTriangleMateShellJSON(ctx, &denied, s, o); e == nil || denied.Len() != 0 {
			t.Fatal("atomic writer limits")
		}
	}
	s.graph.faces[0].Outer[0].UVStart.X += .125
	var denied bytes.Buffer
	if e := WriteAffineTriangleMateShellJSON(ctx, &denied, s, AffineTriangleShellJSONOptions{}); e == nil || denied.Len() != 0 {
		t.Fatal("stale native pcurve")
	}
}
