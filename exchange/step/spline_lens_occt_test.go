package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestSplineLensOCCT(t *testing.T) { t.Parallel(); checkSplineLensOCCT(t, "write-step-spline-lens") }
func TestRefinedSplineLensOCCT(t *testing.T) {
	t.Parallel()
	checkSplineLensOCCT(t, "write-step-refined-spline-lens")
}
func checkSplineLensOCCT(t *testing.T, mode string) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, mode)
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if mode == "write-step-refined-spline-lens" && (m.Lower.Data().CountU != 4 || m.Lower.Data().CountV != 5 || m.Upper.Data().CountU != 3) {
		t.Fatal("source basis flattened")
	}
	var b bytes.Buffer
	if e = WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	target := t.TempDir() + "/native.step"
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	wantV := math.Pi / 512
	wantA := 4 * math.Pi / 3 * (math.Pow(1+1./16, 1.5) - 1)
	for index, path := range []string{source, target} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", path)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var m struct{ Volume, Area float64 }
		if e = json.Unmarshal(data, &m); e != nil {
			t.Fatal(e)
		}
		// Source and regenerated graphs produce different adaptive quadrature
		// errors in pinned OCCT. Keep the exported geometry's analytic check
		// tight; independently bound the measured source integration residual.
		volumeTolerance, areaTolerance := 1e-10, 1e-10
		if index == 0 {
			volumeTolerance, areaTolerance = 2e-9, 1e-7
		}
		if math.Abs(m.Volume-wantV) > volumeTolerance || math.Abs(m.Area-wantA) > areaTolerance {
			t.Fatal(index, m, wantV, wantA)
		}

	}
}
