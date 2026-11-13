package step

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPreflightSweptFixtureRouting(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		standalone bool
	}{
		{"linear-extrusion.step", "linear", true}, {"linear-extrusion-occt.step", "linear", false}, {"rational-linear-extrusion-occt.step", "rational", false},
		{"revolution-surface.step", "revolution", true}, {"revolution-surface-occt.step", "revolution", false}, {"revolution-curved-occt.step", "revolution", false}, {"revolution-oblique-occt.step", "revolution", false}, {"revolution-rational-occt.step", "revolution", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, e := os.ReadFile(filepath.Join("testdata", tc.name))
			if e != nil {
				t.Fatal(e)
			}
			r, e := Preflight(bytes.NewReader(raw), Limits{})
			if e != nil || len(r.Unclassified) != 0 {
				t.Fatal(r, e)
			}
			reader, decoder := "ReadLinearExtrusions", "DecodeLinearExtrusionSurface"
			if tc.kind == "rational" {
				reader, decoder = "ReadRationalLinearExtrusions", "DecodeRationalLinearExtrusionSurface"
			}
			if tc.kind == "revolution" {
				reader, decoder = "ReadRevolutionSurfaces", "DecodeRevolutionSurface"
			}
			if !slices.Contains(r.CandidateReaders, decoder) || slices.Contains(r.CandidateReaders, reader) != tc.standalone {
				t.Fatal(r.CandidateReaders)
			}
			if _, e = Read(bytes.NewReader(raw), Limits{}); e == nil {
				t.Fatal("generic solid reader accepted support")
			}
			if _, e = ReadAnalyticBodies(bytes.NewReader(raw), Limits{}); e == nil {
				t.Fatal("analytic solid reader accepted support")
			}
			if tc.standalone {
				if tc.kind == "linear" {
					_, e = ReadLinearExtrusions(bytes.NewReader(raw), Limits{})
				} else {
					_, e = ReadRevolutionSurfaces(bytes.NewReader(raw), Limits{})
				}
				if e != nil {
					t.Fatal("exact standalone codec", e)
				}
				return
			}
			if tc.kind == "linear" {
				_, e = ReadLinearExtrusions(bytes.NewReader(raw), Limits{})
			} else if tc.kind == "rational" {
				_, e = ReadRationalLinearExtrusions(bytes.NewReader(raw), Limits{})
			} else {
				_, e = ReadRevolutionSurfaces(bytes.NewReader(raw), Limits{})
			}
			if e == nil {
				t.Fatal("whole-model codec accepted independent face topology")
			}
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			count := 0
			for _, id := range f.Order {
				if _, ok := component(f.Entities[id], "RECTANGULAR_TRIMMED_SURFACE"); !ok {
					continue
				}
				if tc.kind == "linear" {
					_, e = DecodeLinearExtrusionSurface(f, id, 64)
				} else if tc.kind == "rational" {
					_, e = DecodeRationalLinearExtrusionSurface(f, id, 64)
				} else {
					_, e = DecodeRevolutionSurface(f, id, 64)
				}
				if e != nil {
					t.Fatal("exact entity codec", e)
				}
				count++
			}
			if count != 1 {
				t.Fatal("fixture support count", count)
			}
		})
	}
}
func TestPreflightSweptHintsDoNotAcceptGraphs(t *testing.T) {
	base := "#1=GEOMETRIC_SET();#2=GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION();#3=RECTANGULAR_TRIMMED_SURFACE();#4=SURFACE_OF_REVOLUTION();"
	for _, tc := range []struct {
		suffix string
		whole  bool
	}{{"", true}, {"#5=ADVANCED_FACE();", false}, {"#5=MANIFOLD_SOLID_BREP();", false}, {"#5=SURFACE_OF_LINEAR_EXTRUSION();", false}} {
		raw := preflightFile(base + tc.suffix)
		r, e := Preflight(strings.NewReader(raw), Limits{})
		if e != nil || !slices.Contains(r.CandidateReaders, "DecodeRevolutionSurface") || slices.Contains(r.CandidateReaders, "ReadRevolutionSurfaces") != tc.whole {
			t.Fatal(r, e)
		}
		if _, e = Read(strings.NewReader(raw), Limits{}); e == nil {
			t.Fatal("invalid solid accepted")
		}
		if _, e = ReadRevolutionSurfaces(strings.NewReader(raw), Limits{}); e == nil {
			t.Fatal("hint was acceptance")
		}
	}
}
