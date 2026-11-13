package step

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func curveOracleCheck(t *testing.T, path string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "curve.json")
	stepOracle(t, out, "step-curves", path)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Edges  int
		Length float64
	}
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	if got.Edges != 1 || math.Abs(got.Length-math.Pi/2) > 1e-6 {
		t.Fatal(got)
	}
}
func curveFile(t *testing.T, m CurveModel) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "curve.step")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	e = WriteCurves(f, m)
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
	return p
}
func TestOCCTSTEPCurveExport(t *testing.T) {
	m, e := ReadCurves(strings.NewReader(independentCurve), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	curveOracleCheck(t, curveFile(t, m))
}
func TestOCCTSTEPCurveImport(t *testing.T) {
	p := filepath.Join(t.TempDir(), "occt.step")
	stepOracle(t, p, "write-step-curves")
	f, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCurves(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Curves) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	for _, u := range []float64{0, .25, .5, 1} {
		p, e := m.Curves[0].Geometry.Eval(u)
		if e != nil || math.Abs(math.Hypot(p.X, p.Y)-1) > 1e-9 || math.Abs(p.Z) > 1e-9 {
			t.Fatal(p, e)
		}
	}
	curveOracleCheck(t, curveFile(t, m))
}
