package step

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadRoundedRectangleExtrusions(t *testing.T) {
	for _, height := range []float64{-3, 3} {
		for _, blind := range []bool{false, true} {
			m := roundedSTEPFixture(t, height)
			if !blind {
				m.Bodies[0].Blind = nil
			}
			var b bytes.Buffer
			if e := WriteRoundedRectangleExtrusions(&b, m); e != nil {
				t.Fatal(e)
			}
			original := append([]byte(nil), b.Bytes()...)
			got, e := ReadRoundedRectangleExtrusions(&b, Limits{})
			if e != nil || len(got.Bodies) != 1 || got.LengthUnit != m.LengthUnit {
				t.Fatal(e, got)
			}
			wantHeight := 3.
			if blind {
				wantHeight = height
			}
			x, y := m.Bodies[0], got.Bodies[0]
			if x.Name != y.Name || x.Profile != y.Profile || y.Height != wantHeight ||
				x.Holes[0] != y.Holes[0] || len(y.Blind) != len(x.Blind) || blind && x.Blind[0] != y.Blind[0] {
				t.Fatal("roundtrip changes profile or depth", x, y)
			}
			if e := WriteRoundedRectangleExtrusions(&b, got); e != nil {
				t.Fatal(e)
			}
			if _, e := ReadRoundedRectangleExtrusions(&b, Limits{}); e != nil {
				t.Fatal("re-export", e)
			}
			if _, e := ReadCircularHoleExtrusions(bytes.NewReader(original), Limits{}); e == nil {
				t.Fatal("polygon reader accepts circular corner arcs")
			}
		}
	}
}

func TestReadRoundedRectangleExtrusionsRejectsChangedSource(t *testing.T) {
	m := roundedSTEPFixture(t, 3)
	var b bytes.Buffer
	if e := WriteRoundedRectangleExtrusions(&b, m); e != nil {
		t.Fatal(e)
	}
	src := b.String()
	for _, change := range [][2]string{
		{"CYLINDRICAL_SURFACE('',", "CONICAL_SURFACE('',"},
		{",0.5);", ",0.50000000000000000001);"},
		{"FACE_OUTER_BOUND('',", "FACE_BOUND('',"},
	} {
		bad := strings.Replace(src, change[0], change[1], 1)
		if bad == src {
			t.Fatal("no mutation", change)
		}
		if got, e := ReadRoundedRectangleExtrusions(strings.NewReader(bad), Limits{}); e == nil || len(got.Bodies) != 0 {
			t.Fatal("accepted altered graph", change, e)
		}
	}
}
