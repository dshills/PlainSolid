package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestIndependentConcurrentRationalLens(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/concurrent-rational-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadRationalSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Curves) != 7 || m.Body.Upper.Data().CountV != 4 || m.Body.Lower.Data().CountU != 4 {
		t.Fatal("source partition")
	}
	var b bytes.Buffer
	if e = WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	copy, e := ReadRationalSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Curves, copy.Curves) || !reflect.DeepEqual(m.Body.Upper.Data(), copy.Body.Upper.Data()) || !reflect.DeepEqual(m.Body.Lower.Data(), copy.Body.Lower.Data()) {
		t.Fatal("raw bases changed")
	}
}
func TestRationalLensOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-concurrent-rational-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadRationalSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	target := t.TempDir() + "/native.step"
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	r := 263340. / (1 << 22)
	wantV := math.Pi * math.Pow(r, 4) / 2
	wantA := 4 * math.Pi / 3 * (math.Pow(1+r*r, 1.5) - 1)
	for i, path := range []string{source, target} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", path)
		raw, e = os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Volume, Area float64 }
		if e = json.Unmarshal(raw, &mass); e != nil {
			t.Fatal(e)
		}
		if math.Abs(mass.Volume-wantV) > 1e-10 || math.Abs(mass.Area-wantA) > 1e-9 {
			t.Fatal(i, mass, wantV, wantA)
		}
	}
}
func FuzzRationalSplineLens(f *testing.F) {
	raw, e := os.ReadFile("testdata/concurrent-rational-lens.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRationalSplineLensBody(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 512, MaxValues: 15000, MaxFaces: 2, MaxVertices: 256, MaxEdgeUses: 32, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}
