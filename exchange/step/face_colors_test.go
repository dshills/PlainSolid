package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func faceColorModel(t *testing.T) FaceColoredModel {
	return FaceColoredModel{LengthUnit: .001, Bodies: []FaceColoredBody{{Body: coloredModel(t).Bodies[0].Body, FaceColors: map[int]RGB{0: {.8, .1, .3}}}}}
}
func TestFaceColorsRoundtrip(t *testing.T) {
	t.Parallel()
	m := faceColorModel(t)
	m.Bodies[0].SurfaceColor = &RGB{.2, .4, .6}
	m.Bodies[0].FaceColors[3] = RGB{1, 0, 0}
	var b, again bytes.Buffer
	if e := WriteFaceColoredPlanar(&b, m); e != nil {
		t.Fatal(e)
	}
	if e := WriteFaceColoredPlanar(&again, m); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("not deterministic", e)
	}
	got, e := ReadFaceColoredPlanar(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 1 || got.Bodies[0].SurfaceColor == nil || len(got.Bodies[0].FaceColors) != 2 {
		t.Fatal(got)
	}
	for i, c := range m.Bodies[0].FaceColors {
		if got.Bodies[0].FaceColors[i] != c {
			t.Fatal(got)
		}
	}
	if _, e := ReadColoredPlanar(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("whole-body API accepted face override")
	}
	for _, index := range []int{-1, 6} {
		bad := faceColorModel(t)
		bad.Bodies[0].FaceColors[index] = RGB{1, 0, 0}
		var out bytes.Buffer
		if e := WriteFaceColoredPlanar(&out, bad); e == nil || out.Len() != 0 {
			t.Fatal(index, e)
		}
	}
}
func TestOCCTFaceColors(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(t.TempDir(), "occt.step")
	stepOracle(t, fixture, "write-step-face-color")
	raw, e := os.ReadFile(fixture)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadFaceColoredPlanar(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || len(m.Bodies[0].FaceColors) != 1 {
		t.Fatal(m)
	}
	c, ok := m.Bodies[0].FaceColors[0]
	if !ok || math.Abs(c.R-.8) > 1e-6 || math.Abs(c.G-.1) > 1e-6 || math.Abs(c.B-.3) > 1e-6 {
		t.Fatal(m)
	}
	var b bytes.Buffer
	if e = WriteFaceColoredPlanar(&b, faceColorModel(t)); e != nil {
		t.Fatal(e)
	}
	ours := filepath.Join(t.TempDir(), "ours.step")
	if e = os.WriteFile(ours, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "colors.json")
	stepOracle(t, out, "step-face-color", ours)
	raw, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var colors [][3]float64
	if e = json.Unmarshal(raw, &colors); e != nil || len(colors) != 1 {
		t.Fatal(e, string(raw))
	}
	for i, want := range []float64{.8, .1, .3} {
		if math.Abs(colors[0][i]-want) > 1e-6 {
			t.Fatal(colors)
		}
	}
}

func TestIndependentFaceColorFixture(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/face-colored-box.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadFaceColoredPlanar(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || len(m.Bodies[0].FaceColors) != 1 {
		t.Fatal(m)
	}
	c, ok := m.Bodies[0].FaceColors[0]
	if !ok || math.Abs(c.R-.8) > 1e-6 {
		t.Fatal(m)
	}
}
func FuzzFaceColorSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/face-colored-box.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadFaceColoredPlanar(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 512})
	})
}
