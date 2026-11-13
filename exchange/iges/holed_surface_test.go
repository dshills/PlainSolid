package iges

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func holedFixture(t testing.TB) Model {
	t.Helper()
	b, e := os.ReadFile("testdata/holed-spline-face.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestRectangularSplineHole(t *testing.T) {
	m := holedFixture(t)
	tr := m.TrimmedSurfaces[0]
	outer, e := m.RectangularDomain(tr)
	if e != nil {
		t.Fatal(e)
	}
	hole, e := m.RectangularHole(tr)
	if e != nil {
		t.Fatal(e)
	}
	if outer != (geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}) || hole != (geom.ParameterRectangle{UMin: .3125, UMax: .75, VMin: .25, VMax: .75}) {
		t.Fatal(outer, hole)
	}
	made, e := NewRectangularSurfaceWithHole(m.Entities[tr.Surface].Surface.Geometry, outer, hole, m.LengthUnit)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = Write(&b, made); e != nil {
		t.Fatal(e)
	}
	round, e := Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	got, e := round.RectangularHole(round.TrimmedSurfaces[0])
	if e != nil || got != hole {
		t.Fatal(got, e)
	}
}
func TestRectangularSplineHoleRejections(t *testing.T) {
	for _, kind := range []string{"outer-as-hole", "duplicate", "wrong-support", "missing", "wrong-winding", "one-ulp-model"} {
		t.Run(kind, func(t *testing.T) {
			m := holedFixture(t)
			tr := &m.TrimmedSurfaces[0]
			inner := &m.CurvesOnSurfaces[tr.Inner[0]]
			switch kind {
			case "outer-as-hole":
				tr.Inner[0] = tr.Outer
			case "duplicate":
				tr.Inner = append(tr.Inner, tr.Inner[0])
			case "wrong-support":
				inner.Surface = inner.ParameterCurve
			case "missing":
				tr.Inner[0] = len(m.CurvesOnSurfaces)
			case "wrong-winding":
				tr.Outer, tr.Inner[0] = tr.Inner[0], tr.Outer
			case "one-ulp-model":
				idx := m.Entities[inner.ModelCurve].Composite.Curves[0]
				s := m.Entities[idx].Spline
				p := s.Curve.Poles()
				p[0].X = math.Nextafter(p[0].X, 100)
				g, e := geom.NewBSpline(s.Curve.Degree(), p, s.Curve.Knots(), s.Curve.Weights())
				if e != nil {
					t.Fatal(e)
				}
				s.Curve = g
			}
			var b bytes.Buffer
			if e := Write(&b, m); e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
		})
	}
	m := holedFixture(t)
	s := m.Entities[m.TrimmedSurfaces[0].Surface].Surface.Geometry
	outer := geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}
	for _, hole := range []geom.ParameterRectangle{{UMin: 0, UMax: .75, VMin: .25, VMax: .75}, {UMin: -.1, UMax: .75, VMin: .25, VMax: .75}, {UMin: .3125, UMax: 1, VMin: .25, VMax: .75}, {UMin: .75, UMax: .3125, VMin: .25, VMax: .75}, {UMin: .25, UMax: .75, VMin: .25, VMax: .75}} {
		if _, e := NewRectangularSurfaceWithHole(s, outer, hole, .001); e == nil {
			t.Fatal(hole)
		}
	}
}
func TestOCCTRectangularSplineHole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, path, "write-iges-holed-spline-face")
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	tr := m.TrimmedSurfaces[0]
	outer, e := m.RectangularDomain(tr)
	if e != nil {
		t.Fatal(e)
	}
	hole, e := m.RectangularHole(tr)
	if e != nil {
		t.Fatal(e)
	}
	s := m.Entities[tr.Surface].Surface.Geometry
	made, e := NewRectangularSurfaceWithHole(s, outer, hole, m.LengthUnit)
	if e != nil {
		t.Fatal(e)
	}

	referenceArea, referenceCenter := placedMeasure(t, path)
	area0, moment0 := rationalGraphMeasure(0, 42, 0, 2)
	area1, moment1 := rationalGraphMeasure(20, 36, .5, 1.5)
	expectedArea := area0 - area1
	expectedCenter := moment0.Sub(moment1).Scale(1 / expectedArea)
	// OCCT default surface quadrature has ~0.00194 area error on this graph.
	// Keep analytic accuracy separate from strict source/output agreement.
	if math.Abs(referenceArea-expectedArea) > 0.003 || referenceCenter.Sub(expectedCenter).Length() > 0.001 {
		t.Fatal(referenceArea, expectedArea, referenceCenter, expectedCenter)
	}
	for _, file := range []string{writeModelFile(t, made), writeModelFile(t, m)} {
		area, center := placedMeasure(t, file)
		if math.Abs(area-referenceArea) > 1e-8 || center.Sub(referenceCenter).Length() > 1e-8 {
			t.Fatal(area, referenceArea, center, referenceCenter)
		}
	}
}

// Independent graph z=x*y/4. Integrate x analytically and y by composite Simpson.
func rationalGraphMeasure(x0, x1, y0, y1 float64) (float64, geom.Vec3) {
	n := 1024
	h := (y1 - y0) / float64(n)
	area := 0.
	moment := geom.Vec3{}
	for i := 0; i <= n; i++ {
		y := y0 + float64(i)*h
		a := 1 + y*y/16
		f := func(x float64) float64 { return .5 * (x*math.Sqrt(a+x*x/16) + 4*a*math.Asinh(x/(4*math.Sqrt(a)))) }
		g := func(x float64) float64 { return 16.0 / 3 * math.Pow(a+x*x/16, 1.5) }
		slice, mx := f(x1)-f(x0), g(x1)-g(x0)
		weight := 2.
		if i%2 == 1 {
			weight = 4
		}
		if i == 0 || i == n {
			weight = 1
		}
		area += weight * slice
		moment = moment.Add(geom.Vec3{X: mx, Y: y * slice, Z: y * mx / 4}.Scale(weight))
	}
	return area * h / 3, moment.Scale(h / 3)
}
