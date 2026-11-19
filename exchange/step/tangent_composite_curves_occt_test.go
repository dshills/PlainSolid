package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOCCTTangentCompositeCurves(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_STEP_COMPOSITE")
	if oracle == "" {
		t.Skip("optional tangent composite helper")
	}
	type result struct {
		Edges, Wires, Closed           int
		Length                         float64
		DerivativeEnd, DerivativeStart [3]float64
	}
	run := func(mode, path string) result {
		raw, e := exec.Command(oracle, mode, path).CombinedOutput()
		if e != nil {
			t.Fatalf("%s %v %s", mode, e, raw)
		}
		if mode != "read" {
			return result{}
		}
		i := bytes.LastIndex(raw, []byte("{\"Edges\""))
		if i < 0 {
			t.Fatal(string(raw))
		}
		var out result
		if e = json.Unmarshal(bytes.TrimSpace(raw[i:]), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	for _, closed := range []bool{false, true} {
		mode := "write-tangent"
		if closed {
			mode += "-closed"
		}
		source := filepath.Join(t.TempDir(), "source.step")
		run(mode, source)
		want := run("read", source)
		raw, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadTangentCompositeCurves(bytes.NewReader(raw), Limits{})
		if e != nil || !m.TangentTransitions[0][0] {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteTangentCompositeCurves(&b, m); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "native.step")
		if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		got := run("read", path)
		edges, c := 2, 0
		if closed {
			edges, c = 3, 1
		}
		d := [3]float64{1.5, 0, 0}
		if got.Edges != edges || got.Wires != 1 || got.Closed != c || math.Abs(got.Length-want.Length) > 1e-9 || got.DerivativeEnd != d || got.DerivativeStart != d || want.DerivativeEnd != d || want.DerivativeStart != d {
			t.Fatal(got, want)
		}
		t.Log(closed, got)
	}
}
