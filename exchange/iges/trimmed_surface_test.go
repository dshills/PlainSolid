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

func trimmedPlaneModel(t testing.TB) Model {
	t.Helper()
	m := planarMappingModel(t)
	m.Entities = m.Entities[:1]
	m.CurvesOnSurfaces = nil
	for _, loop := range [][]geom.Vec3{{{X: .125, Y: .125}, {X: .875, Y: .125}, {X: .875, Y: .875}, {X: .125, Y: .875}, {X: .125, Y: .125}}, {{X: .375, Y: .375}, {X: .375, Y: .625}, {X: .625, Y: .625}, {X: .625, Y: .375}, {X: .375, Y: .375}}} {
		model := make([]geom.Vec3, len(loop))
		for i, p := range loop {
			model[i] = geom.Vec3{X: 2 * p.X, Y: 3 * p.Y}
		}
		k := []float64{0, 0, 1, 2, 3, 4, 4}
		u, e := geom.NewBSpline(1, loop, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		c, e := geom.NewBSpline(1, model, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		a := len(m.Entities)
		m.Entities = append(m.Entities, Entity{Spline: &Spline{Curve: u, Start: 0, End: 4}}, Entity{Spline: &Spline{Curve: c, Start: 0, End: 4}})
		m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, CurveOnSurface{Surface: 0, ParameterCurve: a, ModelCurve: a + 1, Preference: 3})
	}
	m.TrimmedSurfaces = []TrimmedSurface{{Surface: 0, Outer: 0, Inner: []int{1}}}
	return m
}
func TestTrimmedPlaneRoundtrip(t *testing.T) {
	m := trimmedPlaneModel(t)
	if e := m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := Read(&b, Limits{})
	if e != nil || len(got.TrimmedSurfaces) != 1 || len(got.TrimmedSurfaces[0].Inner) != 1 {
		t.Fatal(got, e)
	}
}
func TestTrimmedPlaneMalformed(t *testing.T) {
	for _, kind := range []string{"reverse", "cross", "touch", "outside", "duplicate", "support"} {
		m := trimmedPlaneModel(t)
		trim := m.TrimmedSurfaces[0]
		if kind == "duplicate" {
			trim.Inner = append(trim.Inner, 1)
		} else if kind == "support" {
			trim.Surface = 1
		} else {
			c := m.CurvesOnSurfaces[1]
			p := m.Entities[c.ParameterCurve].Spline.Curve.Poles()
			switch kind {
			case "reverse":
				p[1], p[3] = p[3], p[1]
			case "cross":
				p[1], p[2] = p[2], p[1]
			case "touch":
				p[0].X = .125
				p[4] = p[0]
			case "outside":
				for i := range p {
					p[i].X += .5
				}
			}
			mp := make([]geom.Vec3, len(p))
			for i, v := range p {
				mp[i] = geom.Vec3{X: 2 * v.X, Y: 3 * v.Y}
			}
			k := m.Entities[c.ParameterCurve].Spline.Curve.Knots()
			uv, e := geom.NewBSpline(1, p, k, nil)
			if e != nil {
				t.Fatal(e)
			}
			model, e := geom.NewBSpline(1, mp, k, nil)
			if e != nil {
				t.Fatal(e)
			}
			m.Entities[c.ParameterCurve].Spline.Curve = uv
			m.Entities[c.ModelCurve].Spline.Curve = model
		}
		if e := m.ValidateTrimmedSurface(trim); e == nil {
			t.Fatal(kind)
		}
	}
}
func checkTrimmedOracle(t *testing.T, path string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "area.json")
	oracleCommand(t, out, "iges-surface", path)
	data, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(data, &got); e != nil {
		t.Fatal(e)
	}
	if got.Faces != 1 || math.Abs(got.Area-3) > 1e-8 {
		t.Fatal(got)
	}
}
func TestOCCTTrimmedPlaneImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trimmed.igs")
	oracleCommand(t, path, "write-iges-trimmed")
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(f, Limits{})
	_ = f.Close()
	if e != nil || len(m.TrimmedSurfaces) != 1 {
		t.Fatal(m, e)
	}
	checkTrimmedOracle(t, writeModelFile(t, m))
}
func TestOCCTTrimmedPlaneExport(t *testing.T) {
	checkTrimmedOracle(t, writeModelFile(t, trimmedPlaneModel(t)))
}
func FuzzTrimmedSurface(f *testing.F) {
	var b bytes.Buffer
	_ = Write(&b, trimmedPlaneModel(f))
	f.Add(b.Bytes())
	if raw, e := os.ReadFile("testdata/quadratic-trim.igs"); e == nil {
		f.Add(raw)
	}
	if raw, e := os.ReadFile("testdata/quadratic-annulus.igs"); e == nil {
		f.Add(raw)
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		m, e := Read(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxRecords: 1000, MaxEntities: 100, MaxValues: 3000, MaxControlPoints: 100})
		if e == nil {
			for _, s := range m.TrimmedSurfaces {
				if e = m.ValidateTrimmedSurface(s); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}

func TestTrimmedPlanePointersAndNoHoles(t *testing.T) {
	m := trimmedPlaneModel(t)
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := b.String()
	for _, value := range []string{"144,1,1,1,11,11", "144,1,1,1,11,99", "144,1,0,1,11,13"} {
		bad := strings.Replace(raw, "144,1,1,1,11,13", value, 1)
		if bad == raw {
			t.Fatal("missed mutation")
		}
		if _, e := Read(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(value)
		}
	}
	m.TrimmedSurfaces[0].Inner = nil
	b.Reset()
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := Read(&b, Limits{})
	if e != nil || len(got.TrimmedSurfaces[0].Inner) != 0 {
		t.Fatal(got, e)
	}
}
