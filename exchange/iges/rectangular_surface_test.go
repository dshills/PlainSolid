package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func rectangleFixture(t testing.TB) Model {
	t.Helper()
	b, e := os.ReadFile("testdata/rectangular-spline-face.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestRectangularSplineFixture(t *testing.T) {
	m := rectangleFixture(t)
	if len(m.TrimmedSurfaces) != 1 {
		t.Fatal(m)
	}
	d, e := m.RectangularDomain(m.TrimmedSurfaces[0])
	if e != nil {
		t.Fatal(e)
	}
	want := geom.ParameterRectangle{UMin: .3125, UMax: .75, VMin: .25, VMax: .75}
	if d != want {
		t.Fatal(d)
	}
	s := m.Entities[m.TrimmedSurfaces[0].Surface].Surface.Geometry
	got, e := NewRectangularSurface(s, d, m.LengthUnit)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = Write(&b, got); e != nil {
		t.Fatal(e)
	}
	round, e := Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	rd, e := round.RectangularDomain(round.TrimmedSurfaces[0])
	if e != nil || rd != d {
		t.Fatal(rd, e)
	}
	if len(round.Entities[0].Surface.Geometry.Data().Poles) != 4 {
		t.Fatal("support changed")
	}
}
func TestRectangularSplineRejections(t *testing.T) {
	for _, kind := range []string{"nested", "duplicate", "range", "uv", "model", "hole", "wrong-surface", "reverse", "standalone"} {
		t.Run(kind, func(t *testing.T) {
			m := rectangleFixture(t)
			c := m.CurvesOnSurfaces[0]
			u := m.Entities[c.ParameterCurve].Composite
			v := m.Entities[c.ModelCurve].Composite
			switch kind {
			case "standalone":
				m.CurvesOnSurfaces = nil
				m.TrimmedSurfaces = nil
			case "nested":
				u.Curves[0] = c.ModelCurve
			case "duplicate":
				u.Curves[0] = u.Curves[1]
			case "range":
				u.Curves[0] = -1
			case "uv":
				m.Entities[u.Curves[0]].Line.Start.Y = math.Nextafter(.75, 1)
			case "model":
				s := m.Entities[v.Curves[0]].Spline
				p := s.Curve.Poles()
				p[0].Z = math.Nextafter(p[0].Z, 100)
				g, e := geom.NewBSpline(s.Curve.Degree(), p, s.Curve.Knots(), s.Curve.Weights())
				if e != nil {
					t.Fatal(e)
				}
				s.Curve = g
			case "hole":
				m.TrimmedSurfaces[0].Inner = []int{0}
			case "wrong-surface":
				m.TrimmedSurfaces[0].Surface = u.Curves[0]
			case "reverse":
				u.Curves[0], u.Curves[3] = u.Curves[3], u.Curves[0]
			}
			var b bytes.Buffer
			if e := Write(&b, m); e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
		})
	}
	m := rectangleFixture(t)
	s := m.Entities[m.TrimmedSurfaces[0].Surface].Surface.Geometry
	if _, e := NewRectangularSurface(s, geom.ParameterRectangle{UMin: .25, UMax: .75, VMin: .25, VMax: .75}, .001); e == nil {
		t.Fatal("rounded boundary export")
	}
}
func rectangleOracleArea(t *testing.T, path string) float64 {
	t.Helper()
	out := filepath.Join(t.TempDir(), "area.json")
	oracleCommand(t, out, "iges-surface", path)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(b, &r); e != nil || r.Faces != 1 || r.Area <= 0 {
		t.Fatal(r, e)
	}
	return r.Area
}
func TestOCCTRectangularSpline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, path, "write-iges-bounded-spline-face")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	d, e := m.RectangularDomain(m.TrimmedSurfaces[0])
	if e != nil {
		t.Fatal(e)
	}
	made, e := NewRectangularSurface(m.Entities[m.TrimmedSurfaces[0].Surface].Surface.Geometry, d, m.LengthUnit)
	if e != nil {
		t.Fatal(e)
	}
	a := rectangleOracleArea(t, path)
	b := rectangleOracleArea(t, writeModelFile(t, made))
	if math.Abs(a-b) > 1e-8 {
		t.Fatal(a, b)
	}
}
func FuzzRectangularSpline(f *testing.F) {
	b, e := os.ReadFile("testdata/rectangular-spline-face.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	placed, e := os.ReadFile("testdata/placed-spline-face.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(placed)
	holed, e := os.ReadFile("testdata/holed-spline-face.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(holed)
	multiple, e := os.ReadFile("testdata/multiholed-spline-face.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(multiple)
	f.Fuzz(func(t *testing.T, b []byte) {
		m, e := Read(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxRecords: 1000, MaxEntities: 100, MaxValues: 3000, MaxControlPoints: 100})
		if e == nil {
			for _, tr := range m.TrimmedSurfaces {
				if e := m.ValidateTrimmedSurface(tr); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}

func TestRectangularSplineParserBounds(t *testing.T) {
	raw, e := os.ReadFile("testdata/rectangular-spline-face.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, mutation := range [][2]string{{"102,4,9,11,13,15;", "102,4,7,11,13,15;"}, {"102,4,9,11,13,15;", "102,4,9,9,13,15;"}, {"102,4,9,11,13,15;", "102,4,99,11,13,15;"}, {"102,4,9,11,13,15;", "102,5,9,11,13,15;"}, {"142,0,3,7,17,3;", "142,0,3,7,19,3;"}} {
		// Preserve the fixed-width P record and its directory ownership.
		lines := bytes.Split(bytes.Clone(raw), []byte{'\n'})
		changed := false
		for i, line := range lines {
			if len(line) == 80 && line[72] == 'P' && bytes.Contains(line[:64], []byte(mutation[0])) {
				text := bytes.Replace(bytes.TrimRight(line[:64], " "), []byte(mutation[0]), []byte(mutation[1]), 1)
				if len(text) > 64 {
					t.Fatal("mutation size")
				}
				copy(line[:64], bytes.Repeat([]byte{' '}, 64))
				copy(line, text)
				lines[i] = line
				changed = true
			}
		}
		if !changed {
			t.Fatal("missing mutation")
		}
		if _, e := Read(bytes.NewReader(bytes.Join(lines, []byte{'\n'})), Limits{}); e == nil {
			t.Fatal(mutation)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 12}, {MaxControlPoints: 11}, {MaxValues: 10}, {MaxRecords: 20}} {
		if _, e := Read(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestRectangularSplineFold(t *testing.T) {
	s, e := geom.NewBSplineSurface(2, 1, 3, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}, {}, {Y: 1}}, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NewRectangularSurface(s, geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}, .001); e == nil {
		t.Fatal("folded patch accepted")
	}
}
