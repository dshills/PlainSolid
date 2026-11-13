package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func placedFixture(t testing.TB) Model {
	t.Helper()
	b, e := os.ReadFile("testdata/placed-spline-face.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestPlacedRectangularSurface(t *testing.T) {
	m := placedFixture(t)
	tr := m.TrimmedSurfaces[0]
	se := m.Entities[tr.Surface]
	d, e := m.RectangularDomain(tr)
	if e != nil {
		t.Fatal(e)
	}
	a := geom.Affine{{0, -1, 0, 5}, {1, 0, 0, 7}, {0, 0, 1, 11}}
	if se.Transform == nil || *se.Transform != a {
		t.Fatal(se.Transform)
	}
	made, e := NewPlacedRectangularSurface(se.Surface.Geometry, d, m.LengthUnit, a)
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
	if !reflect.DeepEqual(round.Entities[0].Surface.Geometry.Data(), se.Surface.Geometry.Data()) || *round.Entities[0].Transform != a {
		t.Fatal("baked or changed original support")
	}
	for _, idx := range round.Entities[round.CurvesOnSurfaces[0].ParameterCurve].Composite.Curves {
		if round.Entities[idx].Transform != nil {
			t.Fatal("placed UV coordinates")
		}
	}
	// Exact homogeneous comparison also permits separately baked model edges.
	for _, idx := range m.Entities[m.CurvesOnSurfaces[0].ModelCurve].Composite.Curves {
		leaf := &m.Entities[idx]
		exact, e := geom.NewExactSplineCurve(leaf.Spline.Curve, rectangleOptions)
		if e != nil {
			t.Fatal(e)
		}
		exact, e = exact.Transform(a, rectangleOptions)
		if e != nil {
			t.Fatal(e)
		}
		leaf.Spline.Curve, e = exact.ToBSpline()
		if e != nil {
			t.Fatal(e)
		}
		leaf.Transform = nil
		leaf.Spline.Planar = false
	}
	if e = m.ValidateTrimmedSurface(tr); e != nil {
		t.Fatal(e)
	}
}
func TestPlacedRectangularRejections(t *testing.T) {
	for _, kind := range []string{"model-missing", "support-missing", "uv", "mismatch", "scale", "shear", "reflection", "rounded", "singular"} {
		t.Run(kind, func(t *testing.T) {
			m := placedFixture(t)
			c := m.CurvesOnSurfaces[0]
			si := c.Surface
			mi := m.Entities[c.ModelCurve].Composite.Curves[0]
			ui := m.Entities[c.ParameterCurve].Composite.Curves[0]
			switch kind {
			case "model-missing":
				m.Entities[mi].Transform = nil
			case "support-missing":
				m.Entities[si].Transform = nil
			case "uv":
				a := *m.Entities[si].Transform
				m.Entities[ui].Transform = &a
			case "mismatch":
				m.Entities[mi].Transform[0][3] = math.Nextafter(5, 6)
			case "scale":
				m.Entities[si].Transform[1][0] = 2
			case "shear":
				m.Entities[si].Transform[0][0] = .5
			case "reflection":
				m.Entities[si].Transform[1][0] = -1
			case "rounded":
				m.Entities[si].Transform[1][0] = math.Nextafter(1, 2)
			case "singular":
				m.Entities[si].Transform[1][0] = 0
			}
			var b bytes.Buffer
			if e := Write(&b, m); e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
		})
	}
}
func placedMeasure(t *testing.T, path string) (float64, geom.Vec3) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "measure.json")
	oracleCommand(t, out, "iges-surface", path)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Faces    int
		Area     float64
		Centroid [3]float64
	}
	if e = json.Unmarshal(b, &r); e != nil || r.Faces != 1 {
		t.Fatal(r, e)
	}
	return r.Area, geom.Vec3{X: r.Centroid[0], Y: r.Centroid[1], Z: r.Centroid[2]}
}
func TestOCCTPlacedRectangularSurface(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, path, "write-iges-placed-spline-face")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	tr := m.TrimmedSurfaces[0]
	d, e := m.RectangularDomain(tr)
	if e != nil {
		t.Fatal(e)
	}
	se := m.Entities[tr.Surface]
	made, e := NewPlacedRectangularSurface(se.Surface.Geometry, d, m.LengthUnit, *se.Transform)
	if e != nil {
		t.Fatal(e)
	}
	base, e := NewRectangularSurface(se.Surface.Geometry, d, m.LengthUnit)
	if e != nil {
		t.Fatal(e)
	}
	area, centroid := placedMeasure(t, writeModelFile(t, base))
	expected := se.Transform.Apply(centroid)
	for _, file := range []string{path, writeModelFile(t, made), writeModelFile(t, m)} {
		gotArea, gotCenter := placedMeasure(t, file)
		if math.Abs(gotArea-area) > 1e-8 || gotCenter.Sub(expected).Length() > 1e-8 {
			t.Fatal(gotArea, area, gotCenter, expected)
		}
	}
}
