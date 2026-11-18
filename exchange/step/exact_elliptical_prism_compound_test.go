package step

import (
	"bytes"
	"context"
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestExactEllipticalPrismCompound(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/occt-exact-ellipse-compound.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadExactEllipticalPrismCompound(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if m.Members[0].Variant != EllipticalTilted || m.Members[1].Variant != EllipticalOrthogonalAnnular || m.LengthUnit != .001 {
		t.Fatal("classification")
	}
	sources := [2]*File{}
	for i, b := range m.Members {
		sources[i] = b.Source
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, b.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
		if e != nil || len(s.Faces()) != len(b.Solid.Faces()) {
			t.Fatal(e)
		}
	}
	var out bytes.Buffer
	if e = WriteExactEllipticalPrismCompound(ctx, &out, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	again, e := ReadExactEllipticalPrismCompound(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for i := range again.Members {
		if again.Members[i].Variant != m.Members[i].Variant || again.Members[i].Name != m.Members[i].Name {
			t.Fatal("lost member")
		}
	}
	if path := os.Getenv("PLAINSOLID_ELLIPSE_COMPOUND_OUTPUT"); path != "" {
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, pair := range [][2]string{{"(#11,#87)", "(#11,#15)"}, {"#186,", "#72,"}, {"#88)", "#16)"}} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("missing mutation", pair[0])
		}
		x, e := ReadExactEllipticalPrismCompound(ctx, bytes.NewReader(bad), Limits{}, 1e-9)
		if e == nil || x.Source != nil {
			t.Fatal("cross-owner accepted")
		}
	}
	if _, e = ReadExactEllipticalPrismCompound(ctx, bytes.NewReader(raw), Limits{MaxFaces: 15}, 1e-9); e == nil {
		t.Fatal("cumulative face budget")
	}
	out.Reset()
	if e = WriteExactEllipticalPrismCompound(ctx, &out, [2]*File{sources[0], nil}, Limits{}, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("non-atomic invalid member")
	}
	if _, e = ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("single material widened")
	}
}

func FuzzExactEllipticalPrismCompound(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-exact-ellipse-compound.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		m, e := ReadExactEllipticalPrismCompound(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 236, MaxValues: 8192, MaxDepth: 16}, 1e-9)
		if e != nil && m.Source != nil {
			t.Fatal("partial compound")
		}
	})
}
