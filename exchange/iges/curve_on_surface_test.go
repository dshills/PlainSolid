package iges

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

func planarMappingModel(t testing.TB) Model {
	t.Helper()
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3}, {X: 2}, {X: 2, Y: 3}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	uv := []geom.Vec3{{X: .25, Y: .25}, {X: .75, Y: .25}, {X: .75, Y: .75}}
	model := []geom.Vec3{{X: .5, Y: .75}, {X: 1.5, Y: .75}, {X: 1.5, Y: 2.25}}
	k := []float64{0, 0, 0, 1, 1, 1}
	w := []float64{1, math.Sqrt(.5), 1}
	u, e := geom.NewBSpline(2, uv, k, w)
	if e != nil {
		t.Fatal(e)
	}
	c, e := geom.NewBSpline(2, model, k, w)
	if e != nil {
		t.Fatal(e)
	}
	return Model{LengthUnit: .001, Entities: []Entity{{Surface: &Surface{Geometry: s, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1, Polynomial: true}}, {Spline: &Spline{Curve: u, Start: 0, End: 1}}, {Spline: &Spline{Curve: c, Start: 0, End: 1}}}, CurvesOnSurfaces: []CurveOnSurface{{Surface: 0, ParameterCurve: 1, ModelCurve: 2, Preference: 3}}}
}
func TestExactCurveOnSurface(t *testing.T) {
	m := planarMappingModel(t)
	if e := m.ValidateCurveOnSurface(m.CurvesOnSurfaces[0]); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := Read(&b, Limits{})
	if e != nil || len(got.CurvesOnSurfaces) != 1 {
		t.Fatal(got, e)
	}
	if e = got.ValidateCurveOnSurface(got.CurvesOnSurfaces[0]); e != nil {
		t.Fatal(e)
	}
}
func TestCurveOnSurfaceRejectsMismatch(t *testing.T) {
	for _, kind := range []string{"point", "weight", "surface", "reference", "transform"} {
		m := planarMappingModel(t)
		switch kind {
		case "point":
			s := m.Entities[2].Spline
			d := s.Curve.Poles()
			d[1].X = math.Nextafter(d[1].X, 2)
			g, e := geom.NewBSpline(s.Curve.Degree(), d, s.Curve.Knots(), s.Curve.Weights())
			if e != nil {
				t.Fatal(e)
			}
			s.Curve = g
		case "weight":
			s := m.Entities[2].Spline
			w := s.Curve.Weights()
			w[1] = math.Nextafter(w[1], 1)
			g, e := geom.NewBSpline(s.Curve.Degree(), s.Curve.Poles(), s.Curve.Knots(), w)
			if e != nil {
				t.Fatal(e)
			}
			s.Curve = g
		case "surface":
			s := m.Entities[0].Surface
			d := s.Geometry.Data()
			d.Poles[3].Z = .01
			g, e := geom.NewBSplineSurface(1, 1, 2, 2, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				t.Fatal(e)
			}
			s.Geometry = g
		case "reference":
			m.CurvesOnSurfaces[0].ParameterCurve = 2
		case "transform":
			x := geom.Identity()
			m.Entities[0].Transform = &x
		}
		if e := m.ValidateCurveOnSurface(m.CurvesOnSurfaces[0]); e == nil {
			t.Fatal(kind)
		}
	}
}
func TestOCCTCurveOnSurfaceImport(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pcurve.igs")
	oracleCommand(t, p, "write-iges-pcurve")
	f, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if len(m.CurvesOnSurfaces) != 1 {
		t.Fatal(m)
	}
	if e = m.ValidateCurveOnSurface(m.CurvesOnSurfaces[0]); e != nil {
		t.Fatal(e)
	}
	c := m.CurvesOnSurfaces[0]
	uv, e := m.Entities[c.ParameterCurve].Eval(.5)
	if e != nil {
		t.Fatal(e)
	}
	p3, e := m.Entities[c.ModelCurve].Eval(.5)
	if e != nil || p3.Distance(geom.Vec3{X: 2 * uv.X, Y: 3 * uv.Y}) > 1e-12 {
		t.Fatal(uv, p3, e)
	}
}

func TestOCCTCurveOnSurfaceExport(t *testing.T) {
	path := writeModelFile(t, planarMappingModel(t))
	out := filepath.Join(t.TempDir(), "length.json")
	oracleCommand(t, out, "iges", path)
	data, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Edges  int
		Length float64
	}
	if e = json.Unmarshal(data, &got); e != nil {
		t.Fatal(e)
	}
	const n = 10000
	h := math.Pi / (2 * n)
	sum := 0.0
	for i := 0; i <= n; i++ {
		theta := float64(i) * h
		value := math.Sqrt(math.Cos(theta)*math.Cos(theta) + 2.25*math.Sin(theta)*math.Sin(theta))
		weight := 2.0
		if i == 0 || i == n {
			weight = 1
		} else if i%2 == 1 {
			weight = 4
		}
		sum += weight * value
	}
	want := sum * h / 3
	reference := filepath.Join(t.TempDir(), "reference.igs")
	oracleCommand(t, reference, "write-iges-pcurve")
	referenceOut := filepath.Join(t.TempDir(), "reference.json")
	oracleCommand(t, referenceOut, "iges", reference)
	raw, e := os.ReadFile(referenceOut)
	if e != nil {
		t.Fatal(e)
	}
	var baseline struct {
		Edges  int
		Length float64
	}
	if e = json.Unmarshal(raw, &baseline); e != nil {
		t.Fatal(e)
	}
	// OCCT's default quadrature shows ~2.11e-6 error even on its own fixture.
	if got.Edges != 1 || baseline.Edges != 1 || math.Abs(got.Length-baseline.Length) > 1e-9 || math.Abs(got.Length-want) > 5e-6 {
		t.Fatal(got, want)
	}
}
func FuzzCurveOnSurface(f *testing.F) {
	var b bytes.Buffer
	_ = Write(&b, planarMappingModel(f))
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		m, e := Read(bytes.NewReader(data), Limits{MaxBytes: 65536, MaxRecords: 1000, MaxEntities: 100, MaxValues: 3000, MaxControlPoints: 100})
		if e == nil {
			for _, c := range m.CurvesOnSurfaces {
				if e = m.ValidateCurveOnSurface(c); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}

func TestCurveOnSurfacePointerAndLimits(t *testing.T) {
	var b bytes.Buffer
	if e := Write(&b, planarMappingModel(t)); e != nil {
		t.Fatal(e)
	}
	raw := b.String()
	for _, replacement := range []string{"142,0,1,3,3,3", "142,0,1,3,9,3", "142,0,3,1,5,3", "142,9,1,3,5,3"} {
		bad := strings.Replace(raw, "142,0,1,3,5,3", replacement, 1)
		if bad == raw {
			t.Fatal("mutation missed")
		}
		if _, e := Read(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(replacement)
		}
	}
	if _, e := Read(strings.NewReader(raw), Limits{MaxControlPoints: 9}); e == nil {
		t.Fatal("control net budget")
	}
}
