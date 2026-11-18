package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func coloredModel(t *testing.T) ColoredModel {
	return ColoredModel{LengthUnit: .001, Bodies: []ColoredBody{{Body: Body{Name: "part", Solid: advancedSolids(t)[0]}, SurfaceColor: &RGB{.2, .4, .6}}}}
}
func TestColoredRoundtrip(t *testing.T) {
	t.Parallel()
	m := coloredModel(t)
	var b bytes.Buffer
	if e := WriteColoredPlanar(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadColoredPlanar(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 1 || got.Bodies[0].SurfaceColor == nil || *got.Bodies[0].SurfaceColor != *m.Bodies[0].SurfaceColor {
		t.Fatal(got)
	}
	if _, e := Read(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal("geometry-only reader changed", e)
	}
	for _, change := range [][2]string{{".BOTH.", ".POSITIVE."}, {"COLOUR_RGB(''," + real(.2), "COLOUR_RGB('',1.2"}, {"SURFACE_STYLE_FILL_AREA", "SURFACE_STYLE_TRANSPARENT"}} {
		bad := strings.Replace(b.String(), change[0], change[1], 1)
		if bad == b.String() {
			t.Fatal("stale mutation", change)
		}
		if _, e := ReadColoredPlanar(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("accepted", change)
		}
	}
	if e := WriteColoredPlanar(failWriter{}, m); e == nil {
		t.Fatal("writer error")
	}
	m.Bodies[0].SurfaceColor.R = math.NaN()
	if e := WriteColoredPlanar(&bytes.Buffer{}, m); e == nil {
		t.Fatal("NaN color")
	}
}
func TestOCCTColorBothDirections(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "occt.step")
	stepOracle(t, path, "write-step-color")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	got, e := ReadColoredPlanar(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 1 || got.Bodies[0].SurfaceColor == nil {
		t.Fatal(got)
	}
	c := got.Bodies[0].SurfaceColor
	if math.Abs(c.R-.2) > 1e-6 || math.Abs(c.G-.4) > 1e-6 || math.Abs(c.B-.6) > 1e-6 {
		t.Fatal(c)
	}
	var b bytes.Buffer
	if e = WriteColoredPlanar(&b, coloredModel(t)); e != nil {
		t.Fatal(e)
	}
	ours := filepath.Join(t.TempDir(), "ours.step")
	if e = os.WriteFile(ours, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "color.json")
	stepOracle(t, out, "step-color", ours)
	raw, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var colors [][3]float64
	if e = json.Unmarshal(raw, &colors); e != nil {
		t.Fatal(e)
	}
	if len(colors) != 1 {
		t.Fatal(colors)
	}
	for i, want := range []float64{.2, .4, .6} {
		if math.Abs(colors[0][i]-want) > 1e-6 {
			t.Fatal(colors)
		}
	}
}

func TestColoredOptionalAndBounds(t *testing.T) {
	t.Parallel()
	m := coloredModel(t)
	uncolored := m.Bodies[0]
	uncolored.SurfaceColor = nil
	m.Bodies = append(m.Bodies, uncolored)
	var b bytes.Buffer
	if e := WriteColoredPlanar(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadColoredPlanar(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 2 || got.Bodies[0].SurfaceColor == nil || got.Bodies[1].SurfaceColor != nil {
		t.Fatal(got)
	}
	if _, e := ReadColoredPlanar(bytes.NewReader(b.Bytes()), Limits{MaxBytes: 20}); e == nil {
		t.Fatal("byte limit")
	}
	if _, e := ReadColoredPlanar(nil, Limits{}); e == nil {
		t.Fatal("nil reader")
	}
	if _, e := ReadColoredPlanar(failReader{}, Limits{}); e == nil {
		t.Fatal("reader error")
	}
	bad := strings.Replace(b.String(), "ENDSEC;\nEND-ISO-10303-21;", "#999999=COLOUR_RGB('',0.,0.,0.);\nENDSEC;\nEND-ISO-10303-21;", 1)
	if _, e := ReadColoredPlanar(strings.NewReader(bad), Limits{}); e == nil {
		t.Fatal("unowned style ignored")
	}
}
func FuzzColoredSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/colored-box.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(string(raw))
	f.Add("ISO-10303-21;HEADER;ENDSEC;DATA;ENDSEC;END-ISO-10303-21;")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ReadColoredPlanar(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 512})
	})
}

func TestIndependentColorFixture(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/colored-box.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadColoredPlanar(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || m.Bodies[0].SurfaceColor == nil {
		t.Fatal(m)
	}
	c := m.Bodies[0].SurfaceColor
	if math.Abs(c.R-.2) > 1e-6 || math.Abs(c.G-.4) > 1e-6 || math.Abs(c.B-.6) > 1e-6 {
		t.Fatal(c)
	}
}
