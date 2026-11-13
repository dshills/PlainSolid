package step

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadDiskHoleExtrusionsCanonical(t *testing.T) {
	for _, height := range []float64{-3, 3} {
		for _, kind := range []string{"mixed", "through", "solid"} {
			t.Run(kind+real(height), func(t *testing.T) {
				in := diskExtrusionFixture(t, height)
				switch kind {
				case "through":
					in.Bodies[0].Blind = nil
				case "solid":
					in.Bodies[0].Holes, in.Bodies[0].Blind = nil, nil
				}
				var out bytes.Buffer
				if e := WriteDiskHoleExtrusions(&out, in); e != nil {
					t.Fatal(e)
				}
				source := append([]byte(nil), out.Bytes()...)
				got, e := ReadDiskHoleExtrusions(&out, Limits{})
				if e != nil {
					t.Fatal(e)
				}
				a, b := in.Bodies[0], got.Bodies[0]
				wantHeight := height
				if kind != "mixed" {
					wantHeight = 3
				}
				if got.LengthUnit != in.LengthUnit || len(got.Bodies) != 1 || a.Name != b.Name || b.Height != wantHeight ||
					a.Outer != b.Outer || len(a.Holes) != len(b.Holes) || len(a.Blind) != len(b.Blind) {
					t.Fatal(got)
				}
				if len(a.Holes) > 0 && a.Holes[0] != b.Holes[0] || len(a.Blind) > 0 && a.Blind[0] != b.Blind[0] {
					t.Fatal("incorrect cavity recovery", b)
				}
				if kind == "mixed" && a.Frame != b.Frame {
					t.Fatal("blind entry frame changed")
				}
				if _, e := ReadCircularHoleExtrusions(bytes.NewReader(source), Limits{}); e == nil {
					t.Fatal("polygon reader accepted a circular outer wire")
				}
			})
		}
	}
}

func TestReadDiskHoleExtrusionsRejectsNoncanonicalOuter(t *testing.T) {
	var out bytes.Buffer
	if e := WriteDiskHoleExtrusions(&out, diskExtrusionFixture(t, -3)); e != nil {
		t.Fatal(e)
	}
	source := out.String()
	for _, mutation := range [][2]string{
		{"CYLINDRICAL_SURFACE('',", "CONICAL_SURFACE('',"},
		{"CIRCLE('',", "ELLIPSE('',"},
		{"SEAM_CURVE('',", "SURFACE_CURVE('',"},
		{"4.0000000000000000", "4.00000000000000000001"},
	} {
		changed := strings.Replace(source, mutation[0], mutation[1], 1)
		if changed == source {
			if mutation[0] == "4.0000000000000000" {
				// Real formatting may use its shortest finite decimal.
				changed = strings.Replace(source, ",4.)", ",4.00000000000000000001)", 1)
			}
			if changed == source {
				t.Fatal("mutation fixture missing", mutation[0])
			}
		}
		if r, e := ReadDiskHoleExtrusions(strings.NewReader(changed), Limits{}); e == nil || len(r.Bodies) != 0 {
			t.Fatal("accepted invalid source", mutation, e)
		}
	}
}
