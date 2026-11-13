package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestRefinedLensCurvesOCCT(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-refined-curve-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadRefinedSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, curves := range m.Curves {
		for role, k := range []float64{.75, .5, .25} {
			if len(curves[role].Poles()) != 4 || curves[role].Knots()[3] != k {
				t.Fatal("source owner partition", role)
			}
		}
	}
	var b bytes.Buffer
	if e = WriteRefinedSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	target := t.TempDir() + "/native.step"
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	r := 105. / 512
	wantV := math.Pi * math.Pow(r, 4) / 2
	wantA := 4 * math.Pi / 3 * (math.Pow(1+r*r, 1.5) - 1)
	var canonical bytes.Buffer
	if e = WritePartitionedSplineLensBody(&canonical, m.Body, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	canonicalPath := t.TempDir() + "/canonical.step"
	if e = os.WriteFile(canonicalPath, canonical.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for i, path := range []string{source, target, canonicalPath} {
		output := t.TempDir() + "/mass.json"
		stepOracle(t, output, "step-mass-precise", path)
		raw, e = os.ReadFile(output)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Volume, Area float64 }
		if e = json.Unmarshal(raw, &mass); e != nil {
			t.Fatal(e)
		}
		vt, at := 1e-10, 1e-9
		if i == 0 {
			vt, at = 5e-9, 5e-7
		}
		if math.Abs(mass.Volume-wantV) > vt || math.Abs(mass.Area-wantA) > at {
			t.Fatal(i, mass, wantV, wantA)
		}
	}
}
func FuzzRefinedLensCurves(f *testing.F) {
	raw, e := os.ReadFile("testdata/refined-curve-lens.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRefinedSplineLensBody(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 512, MaxValues: 12000, MaxFaces: 2, MaxVertices: 256, MaxEdgeUses: 32, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}
