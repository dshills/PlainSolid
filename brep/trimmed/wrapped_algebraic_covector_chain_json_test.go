package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestWrappedAlgebraicCovectorChainJSON(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	ab, bc, h, outer := wrappedCovectorChainFixture(t)
	s, e := NewWrappedAlgebraicCovectorChain(ctx, ab, bc, h, outer, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteWrappedAlgebraicCovectorChainJSON(ctx, &buf, s, WrappedCovectorChainJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), buf.Bytes()...)
	restored, e := ReadWrappedAlgebraicCovectorChainJSON(ctx, bytes.NewReader(raw), WrappedCovectorChainJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if restored.OuterSeparation().Margins()[0].Cmp(s.OuterSeparation().Margins()[0]) != 0 || len(restored.Faces()) != 3 {
		t.Fatal("reproved topology/margins")
	}
	solid := Solid{vertices: restored.Vertices(), edges: restored.Edges(), faces: restored.Faces(), tolerance: 1e-9}
	r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	var again bytes.Buffer
	if e = WriteWrappedAlgebraicCovectorChainJSON(ctx, &again, restored, WrappedCovectorChainJSONOptions{}); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("deterministic recipe", e)
	}
	for _, mutate := range []func(*wrappedCovectorChainRecipe){
		func(n *wrappedCovectorChainRecipe) { n.Separators[0].Normal[0] = "1" },
		func(n *wrappedCovectorChainRecipe) { n.Separators[2].Normal[1] = "1/8" },
		func(n *wrappedCovectorChainRecipe) { n.Margins[0] = "1/4" },
		func(n *wrappedCovectorChainRecipe) { n.Mates[0].ReverseB = false },
		func(n *wrappedCovectorChainRecipe) { n.Witness.Edges[0].Sources[0][0] = 2 },
		func(n *wrappedCovectorChainRecipe) { n.Kind = "wrapped-three-support-separated-chain" },
	} {
		var n wrappedCovectorChainRecipe
		if e = json.Unmarshal(raw, &n); e != nil {
			t.Fatal(e)
		}
		mutate(&n)
		bad, e := json.Marshal(n)
		if e != nil {
			t.Fatal(e)
		}
		if got, e := ReadWrappedAlgebraicCovectorChainJSON(ctx, bytes.NewReader(bad), WrappedCovectorChainJSONOptions{}); e == nil || got != nil {
			t.Fatal("accepted mutated source/witness")
		}
	}
	for _, o := range []WrappedCovectorChainJSONOptions{
		{MaxWork: 1}, {MaxBytes: int64(len(raw) - 1)},
		{Face: WrappedQuadraticFaceJSONOptions{Face: WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{MaxOperations: 1}}}},
		{Assembly: edgecheck.Options{MaxWork: 1}},
		{Separation: geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}},
	} {
		if got, e := ReadWrappedAlgebraicCovectorChainJSON(ctx, bytes.NewReader(raw), o); e == nil || got != nil {
			t.Fatal("read budget")
		}
		var denied bytes.Buffer
		if e := WriteWrappedAlgebraicCovectorChainJSON(ctx, &denied, s, o); e == nil || denied.Len() != 0 {
			t.Fatal("atomic write budget")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := ReadWrappedAlgebraicCovectorChainJSON(c, bytes.NewReader(raw), WrappedCovectorChainJSONOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
}
