package step

import (
	"bytes"
	"context"
	"os"
	"plainsolid/brep/trimmed"
	"testing"
)

func TestExactEllipticalPrismCollection(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/occt-exact-ellipse-collection.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadExactEllipticalPrismCollection(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 3 {
		t.Fatal("member count")
	}
	if m.Members[0].Variant != EllipticalTilted || m.Members[1].Variant != EllipticalOrthogonalAnnular || m.LengthUnit != .001 {
		t.Fatal("classification")
	}
	if _, e = ReadExactEllipticalPrismCompound(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("pair reader widened")
	}
	sources := make([]*File, len(m.Members))
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
	single, e := mappedPlanarSourceBytes(sources[0])
	if e != nil {
		t.Fatal(e)
	}
	one, e := ReadExactEllipticalPrismCollection(ctx, bytes.NewReader(single), Limits{}, 1e-9)
	if e != nil || len(one.Members) != 1 {
		t.Fatal("single root", e)
	}
	var out bytes.Buffer
	for _, invalid := range [][]*File{nil, make([]*File, 17)} {
		if e = WriteExactEllipticalPrismCollection(ctx, &out, invalid, Limits{}, 1e-9); e == nil || out.Len() != 0 {
			t.Fatal("root bound")
		}
	}

	if e = WriteExactEllipticalPrismCollection(ctx, &out, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	again, e := ReadExactEllipticalPrismCollection(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for i := range again.Members {
		if again.Members[i].Variant != m.Members[i].Variant || again.Members[i].Name != m.Members[i].Name {
			t.Fatal("lost member")
		}
	}
	if path := os.Getenv("PLAINSOLID_ELLIPSE_COLLECTION_OUTPUT"); path != "" {
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, pair := range [][2]string{{"(#11,#87)", "(#11,#15)"}, {"#186,", "#72,"}, {"#88)", "#16)"}} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("missing mutation", pair[0])
		}
		x, e := ReadExactEllipticalPrismCollection(ctx, bytes.NewReader(bad), Limits{}, 1e-9)
		if e == nil || x.Source != nil {
			t.Fatal("cross-owner accepted")
		}
	}
	if _, e = ReadExactEllipticalPrismCollection(ctx, bytes.NewReader(raw), Limits{MaxFaces: 21}, 1e-9); e == nil {
		t.Fatal("cumulative face budget")
	}
	out.Reset()
	if e = WriteExactEllipticalPrismCollection(ctx, &out, []*File{sources[0], nil}, Limits{}, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("non-atomic invalid member")
	}
	if _, e = ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("single material widened")
	}
}

func FuzzExactEllipticalPrismCollection(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-exact-ellipse-collection.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1048576 {
			t.Skip()
		}
		m, e := ReadExactEllipticalPrismCollection(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 1048576, MaxEntities: 1888, MaxValues: 65536, MaxDepth: 16}, 1e-9)
		if e != nil && m.Source != nil {
			t.Fatal("partial compound")
		}
	})
}
