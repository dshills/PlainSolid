package step

import (
	"bytes"
	"context"
	"io"
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/internal/slowtest"
	"testing"
)

type ellipseFamilyCounter struct {
	io.Reader
	n int
}

func (r *ellipseFamilyCounter) Read(b []byte) (int, error) {
	n, e := r.Reader.Read(b)
	r.n += n
	return n, e
}
func familyFixtures(t *testing.T) map[ExactEllipticalPrismVariant][]byte {
	t.Helper()
	out := map[ExactEllipticalPrismVariant][]byte{}
	f, e := rationalPrismTemplate()
	if e != nil {
		t.Fatal(e)
	}
	out[EllipticalCardinal], e = mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	for kind, name := range map[ExactEllipticalPrismVariant]string{EllipticalPythagorean: "pythagorean-elliptical-prism", EllipticalExactControl: "exact-control-elliptical-prism", EllipticalTilted: "tilted-elliptical-prism", EllipticalHomotheticAnnular: "elliptical-annular-prism", EllipticalNestedAnnular: "nested-elliptical-annular-prism", EllipticalOrthogonalAnnular: "orthogonal-elliptical-annular-prism"} {
		out[kind], e = os.ReadFile("testdata/occt-" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
	}
	return out
}
func TestExactEllipticalPrismFamily(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for kind, raw := range familyFixtures(t) {
		t.Run(string(kind), func(t *testing.T) {
			r := &ellipseFamilyCounter{Reader: bytes.NewReader(raw)}
			m, e := ReadExactEllipticalPrismFamily(ctx, r, Limits{}, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			if m.Variant != kind || m.LengthUnit != .001 || r.n != len(raw) {
				t.Fatal(m.Variant, r.n)
			}
			n := 0
			for _, yes := range []bool{m.Cardinal != nil, m.Pythagorean != nil, m.ExactControl != nil, m.Tilted != nil, m.HomotheticAnnular != nil, m.NestedAnnular != nil, m.OrthogonalAnnular != nil} {
				if yes {
					n++
				}
			}
			if n != 1 {
				t.Fatal("ambiguous union")
			}
			var native bytes.Buffer
			if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
				t.Fatal(e)
			}
			s, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if len(s.Faces()) != len(m.Solid.Faces()) {
				t.Fatal("native faces")
			}
			var out bytes.Buffer
			if e = WriteExactEllipticalPrismFamilySource(ctx, &out, m.Source, Limits{}, 1e-9); e != nil {
				t.Fatal(e)
			}
			again, e := ReadExactEllipticalPrismFamily(ctx, &out, Limits{}, 1e-9)
			if e != nil || again.Variant != kind || again.Name != m.Name {
				t.Fatal(again.Variant, e)
			}
		})
	}
}
func TestExactEllipticalPrismFamilyReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := familyFixtures(t)[EllipticalTilted]
	for _, b := range [][]byte{[]byte("ISO-10303-21;"), bytes.Replace(raw, []byte("CLOSED_SHELL"), []byte("OPEN_SHELL"), 1), bytes.Replace(raw, []byte("5.,2."), []byte("6.,2."), 1), bytes.Replace(raw, []byte("END-ISO-10303-21;"), []byte("END-ISO-10303-21;garbage"), 1)} {
		m, e := ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(b), Limits{}, 1e-9)
		if e == nil || m.Source != nil || m.Variant != "" {
			t.Fatal("partial acceptance")
		}
	}
	if _, e := ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(raw), Limits{MaxFaces: 5}, 1e-9); e == nil {
		t.Fatal("budget")
	}
	vendor, e := os.ReadFile("testdata/occt-elliptical-prism.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(vendor), Limits{}, 1e-6); e == nil {
		t.Fatal("rounded repair admitted")
	}
	m, e := ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m.Source.Entities[999] = m.Source.Entities[15]
	var out bytes.Buffer
	if e = WriteExactEllipticalPrismFamilySource(ctx, &out, m.Source, Limits{}, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("orphan output")
	}
}

func FuzzExactEllipticalPrismFamily(f *testing.F) {
	for _, name := range []string{"pythagorean-elliptical-prism", "exact-control-elliptical-prism", "tilted-elliptical-prism", "elliptical-annular-prism", "nested-elliptical-annular-prism", "orthogonal-elliptical-annular-prism"} {
		b, e := os.ReadFile("testdata/occt-" + name + ".step")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	template, e := rationalPrismTemplate()
	if e != nil {
		f.Fatal(e)
	}
	cardinal, e := mappedPlanarSourceBytes(template)
	if e != nil {
		f.Fatal(e)
	}
	f.Add(cardinal)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadExactEllipticalPrismFamily(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 118, MaxValues: 4096, MaxDepth: 16}, 1e-9)
		if e != nil && (m.Source != nil || m.Variant != "") {
			t.Fatal("partial result")
		}
	})
}
