package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func TestOCCTCompositeCurves(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_STEP_COMPOSITE")
	if oracle == "" {
		t.Skip("optional independent STEP composite oracle")
	}
	type result struct {
		Edges, Wires, Closed int
		Length               float64
		Start, End           [3]float64
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
		mode := "write"
		if closed {
			mode = "write-closed"
		}
		source := filepath.Join(t.TempDir(), "source.step")
		run(mode, source)
		want := run("read", source)
		raw, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadCompositeCurves(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteCompositeCurves(&b, m); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "native.step")
		if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		got := run("read", path)
		n, c := 2, 0
		end := [3]float64{2, 1, 0}
		if closed {
			n, c = 3, 1
			end = [3]float64{}
		}
		if got.Edges != n || got.Wires != 1 || got.Closed != c || got.Start != [3]float64{} || got.End != end || math.Abs(got.Length-want.Length) > 1e-9 {
			t.Fatal(got, want)
		}
		lower, upper := 0., 0.
		for _, seg := range m.Curves[0].Segments {
			r, e := geom.BoundCurveLength(seg.Curve.Geometry, geom.CurveLengthBoundOptions{Tolerance: .01, MaxSpans: 256, MaxOperations: 500000})
			if e != nil || !r.Complete {
				t.Fatal(e, r)
			}
			lower += r.Lower
			upper += r.Upper
		}
		if got.Length < lower-1e-9 || got.Length > upper+1e-9 {
			t.Fatal("independent length outside exact bound", got.Length, lower, upper)
		}
		t.Log(closed, got, lower, upper)
	}
}
