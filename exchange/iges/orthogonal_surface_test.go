package iges

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func orthogonalModel(t testing.TB) Model {
	t.Helper()
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 4}, {X: 4}, {X: 4, Y: 4, Z: 1}}, []float64{0, 0, 4, 4}, []float64{0, 0, 4, 4}, nil)
	if e != nil {
		t.Fatal(e)
	}
	outer := []geom.Vec2{{}, {X: 4}, {X: 4, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 4}, {Y: 4}}
	holes := [][]geom.Vec2{{{X: .5, Y: .5}, {X: .5, Y: 1.5}, {X: 1, Y: 1.5}, {X: 1, Y: 1}, {X: 1.5, Y: 1}, {X: 1.5, Y: .5}}, {{X: 2.5, Y: .5}, {X: 2.5, Y: 1.5}, {X: 3.5, Y: 1.5}, {X: 3.5, Y: .5}}}
	m, e := NewOrthogonalSurface(s, outer, holes, .001)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestIndependentOrthogonalSurface(t *testing.T) {
	raw, e := os.ReadFile("testdata/orthogonal-holed-spline-face.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	outer, holes, e := m.OrthogonalBoundaries(m.TrimmedSurfaces[0])
	if e != nil || len(outer) != 6 || len(holes) != 2 || len(holes[0]) != 6 || len(holes[1]) != 4 {
		t.Fatal(outer, holes, e)
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 10}, {MaxControlPoints: 3}} {
		if _, e := Read(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}

func TestOrthogonalSurfaceRoundtrip(t *testing.T) {
	m := orthogonalModel(t)
	outer, holes, e := m.OrthogonalBoundaries(m.TrimmedSurfaces[0])
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = Write(&b, m); e != nil {
		t.Fatal(e)
	}
	round, e := Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	a, h, e := round.OrthogonalBoundaries(round.TrimmedSurfaces[0])
	if e != nil || !reflect.DeepEqual(a, outer) || !reflect.DeepEqual(h, holes) {
		t.Fatal(a, h, e)
	}
	if !reflect.DeepEqual(round.Entities[0].Surface.Geometry.Data(), m.Entities[0].Surface.Geometry.Data()) {
		t.Fatal("support altered")
	}
	if _, e = round.RectangularDomain(round.TrimmedSurfaces[0]); e == nil {
		t.Fatal("rectangle API broadened")
	}
	if _, e = round.RectangularHoles(round.TrimmedSurfaces[0]); e == nil {
		t.Fatal("rectangular holes API broadened")
	}
}
func TestOrthogonalSurfaceReject(t *testing.T) {
	for _, kind := range []string{"reversal", "model", "UV", "count", "source-weights", "nested", "notch", "outer-touch", "hole-touch"} {
		t.Run(kind, func(t *testing.T) {
			m := orthogonalModel(t)
			c := m.CurvesOnSurfaces[1]
			uv := m.Entities[c.ParameterCurve].Composite.Curves
			model := m.Entities[c.ModelCurve].Composite.Curves
			switch kind {
			case "reversal":
				for _, id := range uv {
					line := m.Entities[id].Line
					line.Start, line.End = line.End, line.Start
				}
			case "model":
				sp := m.Entities[model[0]].Spline
				p := sp.Curve.Poles()
				p[0].X = math.Nextafter(p[0].X, math.Inf(1))
				var e error
				sp.Curve, e = geom.NewBSpline(sp.Curve.Degree(), p, sp.Curve.Knots(), sp.Curve.Weights())
				if e != nil {
					t.Fatal(e)
				}
			case "UV":
				m.Entities[uv[0]].Line.Start.X = math.Nextafter(m.Entities[uv[0]].Line.Start.X, math.Inf(1))
			case "count":
				m.Entities[c.ParameterCurve].Composite.Curves = uv[:4]
			case "source-weights":
				m.Entities[model[0]].sourceWeightsRounded = true
			default:
				outer, holes, e := m.OrthogonalBoundaries(m.TrimmedSurfaces[0])
				if e != nil {
					t.Fatal(e)
				}
				sq := func(x, y, size float64) []geom.Vec2 {
					return []geom.Vec2{{X: x, Y: y}, {X: x, Y: y + size}, {X: x + size, Y: y + size}, {X: x + size, Y: y}}
				}
				switch kind {
				case "nested":
					holes = [][]geom.Vec2{sq(.5, .5, 1), sq(.75, .75, .25)}
				case "notch":
					holes = [][]geom.Vec2{sq(2.5, 2.5, .5)}
				case "outer-touch":
					holes = [][]geom.Vec2{sq(0, .5, .5)}
				case "hole-touch":
					holes = [][]geom.Vec2{sq(.5, .5, .5), sq(1, 1, .5)}
				}
				if _, e = NewOrthogonalSurface(m.Entities[0].Surface.Geometry, outer, holes, .001); e == nil {
					t.Fatal("bad polygon accepted")
				}
				return
			}
			if e := m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); e == nil {
				t.Fatal("invalid source correspondence accepted")
			}
		})
	}
}
func TestOCCTOrthogonalSurface(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, source, "write-iges-orthogonal-holed-spline-face")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	outer, holes, e := m.OrthogonalBoundaries(m.TrimmedSurfaces[0])
	if e != nil || len(outer) != 6 || len(holes) != 2 {
		t.Fatal(outer, holes, e)
	}
	var b bytes.Buffer
	if e = Write(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "ours.igs")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	a, c := rectangleOracleArea(t, source), rectangleOracleArea(t, path)
	if math.Abs(a-c) > 1e-8 {
		t.Fatal(a, c)
	}
}
func FuzzOrthogonalSurface(f *testing.F) {
	m := orthogonalModel(f)
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = Read(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 256, MaxControlPoints: 128})
	})
}
