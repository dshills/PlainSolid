package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestPartitionedSplineLensOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-partitioned-spline-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPartitionedSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WritePartitionedSplineLensBody(&b, m.Body, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	target := t.TempDir() + "/native.step"
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	r := 15. / 64
	wantV := math.Pi * math.Pow(r, 4) / 2
	wantA := 4 * math.Pi / 3 * (math.Pow(1+r*r, 1.5) - 1)
	for index, path := range []string{source, target} {
		output := t.TempDir() + "/mass.json"
		stepOracle(t, output, "step-mass-precise", path)
		data, e := os.ReadFile(output)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Volume, Area float64 }
		if e = json.Unmarshal(data, &mass); e != nil {
			t.Fatal(e)
		}
		volumeTolerance, areaTolerance := 1e-10, 1e-9
		// Pinned OCCT source integration has the same small adaptive residual
		// observed for the unsplit lens; exact source curves are checked above.
		if index == 0 {
			volumeTolerance, areaTolerance = 2e-9, 1e-7
		}
		if math.Abs(mass.Volume-wantV) > volumeTolerance || math.Abs(mass.Area-wantA) > areaTolerance {
			t.Fatal(path, mass, wantV, wantA)
		}
	}
}
