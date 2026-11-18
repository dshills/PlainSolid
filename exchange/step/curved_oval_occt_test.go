package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestCurvedOvalFaceOCCT(t *testing.T) {
	t.Parallel()
	checkCurvedOvalOCCT(t, "write-step-curved-oval-face")
}
func TestRationalCurvedOvalFaceOCCT(t *testing.T) {
	t.Parallel()
	checkCurvedOvalOCCT(t, "write-step-rational-curved-oval-face")
}
func checkCurvedOvalOCCT(t *testing.T, mode string) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, mode)
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCurvedOvalFace(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if mode == "write-step-curved-oval-hole" && (len(m.Holes) != 1 || m.Holes[0].Radii.X != 3./32) {
		t.Fatal(m.Holes)
	}
	if m.Outer.QuarterWeight != .5 || m.Outer.Radii.X != 3./16 {
		t.Fatal(m.Outer)
	}
	var b bytes.Buffer
	if e = WriteCurvedOvalFace(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/native.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	areas := []float64{}
	for _, p := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Area float64 }
		if e = json.Unmarshal(data, &mass); e != nil || mass.Area <= 0 {
			t.Fatal(mass, e)
		}
		areas = append(areas, mass.Area)
	}
	if math.Abs(areas[0]-areas[1]) > 1e-10*math.Max(1, math.Abs(areas[0])) {
		t.Fatal(areas)
	}
}
