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

func multiholedFixture(t testing.TB) Model {
	t.Helper()
	raw, e := os.ReadFile("testdata/multiholed-spline-face.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestMultipleRectangularHolesRoundtrip(t *testing.T) {
	m := multiholedFixture(t)
	tr := m.TrimmedSurfaces[0]
	outer, e := m.RectangularDomain(tr)
	if e != nil {
		t.Fatal(e)
	}
	holes, e := m.RectangularHoles(tr)
	if e != nil || len(holes) != 2 {
		t.Fatal(holes, e)
	}
	expected := []geom.ParameterRectangle{{UMin: .3125, UMax: .75, VMin: .125, VMax: .375}, {UMin: .3125, UMax: .75, VMin: .625, VMax: .875}}
	if !reflect.DeepEqual(holes, expected) {
		t.Fatal(holes)
	}
	s := m.Entities[tr.Surface].Surface.Geometry
	made, e := NewRectangularSurfaceWithHoles(s, outer, holes, m.LengthUnit)
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
	actual, e := round.RectangularHoles(round.TrimmedSurfaces[0])
	if e != nil || !reflect.DeepEqual(actual, holes) || !reflect.DeepEqual(round.Entities[0].Surface.Geometry.Data(), s.Data()) {
		t.Fatal(actual, e)
	}
	if _, e := round.RectangularHole(round.TrimmedSurfaces[0]); e == nil {
		t.Fatal("one-hole inspector accepted multiple")
	}
	empty, e := NewRectangularSurfaceWithHoles(s, outer, nil, m.LengthUnit)
	if e != nil {
		t.Fatal(e)
	}
	none, e := empty.RectangularHoles(empty.TrimmedSurfaces[0])
	if e != nil || len(none) != 0 {
		t.Fatal(none, e)
	}
}
func TestMultipleRectangularHolesReject(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	outer := geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}
	first := geom.ParameterRectangle{UMin: .125, UMax: .5, VMin: .125, VMax: .5}
	cases := map[string]geom.ParameterRectangle{"touch-edge": {UMin: .5, UMax: .75, VMin: .125, VMax: .5}, "touch-corner": {UMin: .5, UMax: .75, VMin: .5, VMax: .75}, "overlap": {UMin: .375, UMax: .75, VMin: .375, VMax: .75}, "nested": {UMin: .25, UMax: .375, VMin: .25, VMax: .375}, "outer-touch": {UMin: .625, UMax: 1, VMin: .625, VMax: .875}}
	for name, second := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := NewRectangularSurfaceWithHoles(s, outer, []geom.ParameterRectangle{first, second}, .001); e == nil {
				t.Fatal("constructor accepted")
			}
			// Construct exact independent contours without the public constructor's
			// containment checks, then test the validator used by the parser and writer.
			m, e := NewRectangularSurfaceWithHole(s, outer, first, .001)
			if e != nil {
				t.Fatal(e)
			}
			inner, e := newRectangularSurface(s, second, .001, true)
			if e != nil {
				t.Fatal(e)
			}
			shift := len(m.Entities) - 1
			for _, entity := range inner.Entities[1:] {
				if entity.Composite != nil {
					for i := range entity.Composite.Curves {
						entity.Composite.Curves[i] += shift
					}
				}
				m.Entities = append(m.Entities, entity)
			}
			c := inner.CurvesOnSurfaces[0]
			c.ParameterCurve += shift
			c.ModelCurve += shift
			m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, c)
			m.TrimmedSurfaces[0].Inner = append(m.TrimmedSurfaces[0].Inner, 2)
			if e := m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); e == nil {
				t.Fatal("invalid graph accepted")
			}
			var b bytes.Buffer
			if e := Write(&b, m); e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
		})
	}
	if _, e := NewRectangularSurfaceWithHoles(s, outer, make([]geom.ParameterRectangle, MaxRectangularHoles+1), .001); e == nil {
		t.Fatal("hole budget")
	}
	m := multiholedFixture(t)
	m.TrimmedSurfaces[0].Inner[1] = m.TrimmedSurfaces[0].Inner[0]
	if e := m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); e == nil {
		t.Fatal("duplicate contour")
	}
}
func TestOCCTMultipleRectangularHoles(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, source, "write-iges-multiholed-spline-face")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	tr := m.TrimmedSurfaces[0]
	outer, e := m.RectangularDomain(tr)
	if e != nil {
		t.Fatal(e)
	}
	holes, e := m.RectangularHoles(tr)
	if e != nil || len(holes) != 2 {
		t.Fatal(holes, e)
	}
	made, e := NewRectangularSurfaceWithHoles(m.Entities[tr.Surface].Surface.Geometry, outer, holes, m.LengthUnit)
	if e != nil {
		t.Fatal(e)
	}
	reference, center := placedMeasure(t, source)
	area, moment := rationalGraphMeasure(0, 42, 0, 2)
	for _, y := range [][2]float64{{.25, .75}, {1.25, 1.75}} {
		a, c := rationalGraphMeasure(20, 36, y[0], y[1])
		area -= a
		moment = moment.Sub(c)
	}
	if math.Abs(reference-area) > .003 || center.Sub(moment.Scale(1/area)).Length() > .001 {
		t.Fatal(reference, area, center, moment.Scale(1/area))
	}
	for _, path := range []string{writeModelFile(t, m), writeModelFile(t, made)} {
		a, c := placedMeasure(t, path)
		if math.Abs(a-reference) > 1e-8 || c.Sub(center).Length() > 1e-8 {
			t.Fatal(a, reference, c, center)
		}
	}
}

func BenchmarkMultipleRectangularHoles(b *testing.B) {
	raw, e := os.ReadFile("testdata/multiholed-spline-face.igs")
	if e != nil {
		b.Fatal(e)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := Read(bytes.NewReader(raw), Limits{}); e != nil {
			b.Fatal(e)
		}
	}
}
