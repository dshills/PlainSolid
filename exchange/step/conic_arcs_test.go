package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
)

func conicArcModel(t *testing.T, kind string) ConicArcModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	var c geom.Curve
	start, end := -.75/4, 1.25/4
	if kind == "parabola" {
		c, e = geom.NewParabola(f, 2)
	} else {
		c, e = geom.NewHyperbola(f, 3, 2)
		start, end = math.Tanh(-.75/2), math.Tanh(1.25/2)
	}
	if e != nil {
		t.Fatal(e)
	}
	return ConicArcModel{LengthUnit: .001, Curves: []ConicArc{{Name: kind, Geometry: c, Start: start, End: end}}}
}
func checkConic(t *testing.T, m ConicArcModel, kind string) {
	t.Helper()
	if len(m.Curves) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	c := m.Curves[0]
	for i, u := range []float64{-.75, 1.25} {
		v := c.Start
		if i == 1 {
			v = c.End
		}
		got, e := c.Geometry.Eval(v)
		if e != nil {
			t.Fatal(e)
		}
		want := geom.Vec3{X: 1 + u*u/8, Y: 2 + u, Z: 3}
		if kind == "hyperbola" {
			want = geom.Vec3{X: 1 + 3*math.Cosh(u), Y: 2 + 2*math.Sinh(u), Z: 3}
		}
		if got.Distance(want) > 1e-10 {
			t.Fatal(got, want)
		}
	}
}
func TestConicArcRoundtrip(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"parabola", "hyperbola"} {
		var b bytes.Buffer
		if e := WriteConicArcs(&b, conicArcModel(t, kind)); e != nil {
			t.Fatal(e)
		}
		m, e := ReadConicArcs(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		checkConic(t, m, kind)
		for _, change := range [][2]string{{".T.,.PARAMETER.", ".F.,.PARAMETER."}, {"PARAMETER_VALUE(", "LENGTH_MEASURE("}, {".PARAMETER.", ".CARTESIAN."}} {
			bad := strings.Replace(b.String(), change[0], change[1], 1)
			if _, e := ReadConicArcs(strings.NewReader(bad), Limits{}); e == nil {
				t.Fatal(change)
			}
		}
		if _, e := ReadConicArcs(bytes.NewReader(b.Bytes()), Limits{MaxVertices: 1}); e == nil {
			t.Fatal("resource limit")
		}
		if e := WriteConicArcs(failWriter{}, conicArcModel(t, kind)); e == nil {
			t.Fatal("writer failure")
		}
	}
}
func TestOCCTConicArcs(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"parabola", "hyperbola"} {
		t.Run(kind, func(t *testing.T) {
			fixture := filepath.Join(t.TempDir(), "fixture.step")
			stepOracle(t, fixture, "write-step-"+kind)
			raw, e := os.ReadFile(fixture)
			if e != nil {
				t.Fatal(e)
			}
			m, e := ReadConicArcs(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			checkConic(t, m, kind)
			var b bytes.Buffer
			if e = WriteConicArcs(&b, conicArcModel(t, kind)); e != nil {
				t.Fatal(e)
			}
			ours := filepath.Join(t.TempDir(), "ours.step")
			if e = os.WriteFile(ours, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			lengths := []float64{}
			for _, path := range []string{fixture, ours} {
				out := filepath.Join(t.TempDir(), "measure.json")
				stepOracle(t, out, "step-curves", path)
				raw, e = os.ReadFile(out)
				if e != nil {
					t.Fatal(e)
				}
				var r struct {
					Edges  int
					Length float64
				}
				if e = json.Unmarshal(raw, &r); e != nil || r.Edges != 1 {
					t.Fatal(e, r)
				}
				lengths = append(lengths, r.Length)
			}
			if math.Abs(lengths[0]-lengths[1]) > 1e-8 {
				t.Fatal(lengths)
			}
		})
	}
}

func TestIndependentConicFixtures(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"parabola", "hyperbola"} {
		raw, e := os.ReadFile("testdata/" + kind + ".step")
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadConicArcs(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		checkConic(t, m, kind)
	}
}
func FuzzConicArcSTEP(f *testing.F) {
	for _, kind := range []string{"parabola", "hyperbola"} {
		raw, e := os.ReadFile("testdata/" + kind + ".step")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(string(raw))
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ReadConicArcs(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 512})
	})
}

func TestConicArcInvalidParameters(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"parabola", "hyperbola"} {
		for _, span := range [][2]float64{{0, 0}, {1, -1}, {math.NaN(), 1}, {0, math.Inf(1)}} {
			m := conicArcModel(t, kind)
			m.Curves[0].Start, m.Curves[0].End = span[0], span[1]
			var b bytes.Buffer
			if e := WriteConicArcs(&b, m); e == nil || b.Len() != 0 {
				t.Fatal(kind, span, e)
			}
		}
	}
	m := conicArcModel(t, "hyperbola")
	m.Curves[0].Start = -1
	var b bytes.Buffer
	if e := WriteConicArcs(&b, m); e == nil {
		t.Fatal("infinite hyperbola endpoint")
	}
	h := conicArcModel(t, "hyperbola").Curves[0].Geometry.(geom.Hyperbola)
	frame, a, c, _ := h.Parameters()
	negative, _ := geom.NewHyperbolaBranch(frame, a, c, -1)
	m = conicArcModel(t, "hyperbola")
	m.Curves[0].Geometry = negative
	if e := WriteConicArcs(&b, m); e == nil {
		t.Fatal("negative branch")
	}
	if _, e := ReadConicArcs(nil, Limits{}); e == nil {
		t.Fatal("nil input")
	}
}
