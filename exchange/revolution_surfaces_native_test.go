package exchange

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
)

func revolutionNativeFixture(t *testing.T) step.RevolutionSurfaceModel {
	t.Helper()
	b, e := os.ReadFile("step/testdata/revolution-surface.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadRevolutionSurfaces(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestRevolutionSurfacesNativeChain(t *testing.T) {
	m := revolutionNativeFixture(t)
	m.Surfaces[0].Name = "curved sweep α"
	m.Surfaces[0].Profile.Name = "original meridian"
	m.LengthUnit = .3048
	second := m.Surfaces[0]
	second.Name = "negative axis"
	second.Axis = geom.Vec3{Z: -1}
	second.AngleStart = -1.25
	second.AngleEnd = -.25
	m.Surfaces = append(m.Surfaces, second)
	var b bytes.Buffer
	if e := WriteRevolutionSurfacesJSON(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	got, e := ReadRevolutionSurfacesJSON(&b, RevolutionSurfaceJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Surfaces) != 2 || got.LengthUnit != m.LengthUnit || got.Surfaces[0].Name != m.Surfaces[0].Name || got.Surfaces[0].Profile.Name != m.Surfaces[0].Profile.Name {
		t.Fatal("metadata changed")
	}
	var again bytes.Buffer
	if e = WriteRevolutionSurfacesJSON(&again, got); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("canonical bytes", e)
	}
	var st bytes.Buffer
	if e = step.WriteRevolutionSurfaces(&st, got); e != nil {
		t.Fatal(e)
	}
	back, e := step.ReadRevolutionSurfaces(&st, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, s := range back.Surfaces {
		a, _ := m.Surfaces[i].Surface()
		c, e := s.Surface()
		if e != nil {
			t.Fatal(e)
		}
		u := (s.AngleStart + s.AngleEnd) / 2
		for _, v := range []float64{0, .125, .5, 1} {
			p, _ := a.Eval(u, v)
			q, _ := c.Eval(u, v)
			du, dv, _ := a.Partials(u, v)
			eu, ev, _ := c.Partials(u, v)
			if p != q || du != eu || dv != ev {
				t.Fatal("parameter geometry changed")
			}
		}
	}
	// Returned profiles are independent of input geometry and reconstructed surfaces.
	s, _ := got.Surfaces[0].Surface()
	*got.Surfaces[0].Profile.Geometry = geom.BSpline{}
	if _, e = s.Eval(.5, .5); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Surfaces[0].Profile.Geometry.Eval(.5); e != nil {
		t.Fatal(e)
	}
}
func TestRevolutionSurfacesNativeRejects(t *testing.T) {
	m := revolutionNativeFixture(t)
	m.Surfaces = append(m.Surfaces, m.Surfaces[0])
	var b bytes.Buffer
	if e := WriteRevolutionSurfacesJSON(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	for _, change := range []func(*revolutionSurfacesJSON){
		func(d *revolutionSurfacesJSON) { d.Version = 2 }, func(d *revolutionSurfacesJSON) { d.Kind = "solid" }, func(d *revolutionSurfacesJSON) { d.AngularUnit = "degree" }, func(d *revolutionSurfacesJSON) { d.LengthUnit = 0 },
		func(d *revolutionSurfacesJSON) { d.Surfaces[1].Axis = geom.Vec3{Z: 2} }, func(d *revolutionSurfacesJSON) { d.Surfaces[1].AngleEnd = 6.28 }, func(d *revolutionSurfacesJSON) { d.Surfaces[1].Profile.Poles[1].Z = 0 }, func(d *revolutionSurfacesJSON) { d.Surfaces[1].Profile.Poles[1].Y = 1 }, func(d *revolutionSurfacesJSON) { d.Surfaces[1].Profile.Weights = []float64{2, 2, 2} }, func(d *revolutionSurfacesJSON) { d.Surfaces[1].Profile.Closed = "T" }, func(d *revolutionSurfacesJSON) { d.Surfaces[1].Profile.SelfIntersect = "A" },
	} {
		var d revolutionSurfacesJSON
		if e := json.Unmarshal(raw, &d); e != nil {
			t.Fatal(e)
		}
		change(&d)
		bad, _ := json.Marshal(d)
		got, e := ReadRevolutionSurfacesJSON(bytes.NewReader(bad), RevolutionSurfaceJSONLimits{})
		if e == nil || len(got.Surfaces) != 0 || got.LengthUnit != 0 {
			t.Fatal("non-atomic invalid read", e)
		}
	}
	for _, bad := range [][]byte{append(append([]byte(nil), raw...), []byte("{}")...), bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Version":1`), 1), bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Extra":0,"Version":1`), 1)} {
		if _, e := ReadRevolutionSurfacesJSON(bytes.NewReader(bad), RevolutionSurfaceJSONLimits{}); e == nil {
			t.Fatal("noncanonical accepted")
		}
	}
	for _, l := range []RevolutionSurfaceJSONLimits{{MaxBytes: 32}, {MaxSurfaces: 1}, {MaxPoles: 3}, {MaxBytes: -1}, {MaxPoles: 4097}} {
		if _, e := ReadRevolutionSurfacesJSON(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit", l)
		}
	}
	m.Surfaces[1].Axis = geom.Vec3{Z: 2}
	var out bytes.Buffer
	if e := WriteRevolutionSurfacesJSON(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("non-atomic writer")
	}
	m = revolutionNativeFixture(t)
	m.LengthUnit = math.NaN()
	if e := WriteRevolutionSurfacesJSON(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("nonfinite unit")
	}
}
func FuzzRevolutionSurfacesNative(f *testing.F) {
	f.Add([]byte(`{"Version":1}`))
	c, _ := geom.NewBSpline(1, []geom.Vec3{{X: 1}, {X: 2, Z: 1}}, []float64{0, 0, 1, 1}, nil)
	var valid bytes.Buffer
	if e := WriteRevolutionSurfacesJSON(&valid, step.RevolutionSurfaceModel{LengthUnit: 1, Surfaces: []step.RevolutionSurface{{Profile: step.SplineCurve{Geometry: c}, Axis: geom.Vec3{Z: 1}, AngleStart: 0, AngleEnd: 1}}}); e != nil {
		f.Fatal(e)
	}
	f.Add(valid.Bytes())
	weighted, _ := geom.NewBSpline(1, c.Poles(), c.Knots(), []float64{1, .5})
	var rational bytes.Buffer
	if e := WriteRationalRevolutionSurfacesJSON(&rational, step.RevolutionSurfaceModel{LengthUnit: 1, Surfaces: []step.RevolutionSurface{{Profile: step.SplineCurve{Geometry: weighted}, Axis: geom.Vec3{Z: 1}, AngleStart: 0, AngleEnd: 1}}}); e != nil {
		f.Fatal(e)
	}
	f.Add(rational.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRevolutionSurfacesJSON(bytes.NewReader(b), RevolutionSurfaceJSONLimits{MaxBytes: 65536, MaxSurfaces: 4, MaxPoles: 64})
		_, _ = ReadRationalRevolutionSurfacesJSON(bytes.NewReader(b), RevolutionSurfaceJSONLimits{MaxBytes: 65536, MaxSurfaces: 4, MaxPoles: 64})
	})
}

func TestRevolutionSurfacesNativeV1RemainsPolynomial(t *testing.T) {
	m := revolutionNativeFixture(t)
	c := m.Surfaces[0].Profile.Geometry
	c, e := geom.NewBSpline(c.Degree(), c.Poles(), c.Knots(), []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	m.Surfaces[0].Profile.Geometry = c
	if _, e = m.Surfaces[0].Surface(); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteRevolutionSurfacesJSON(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("v1 silently broadened")
	}
	m = revolutionNativeFixture(t)
	if e = WriteRevolutionSurfacesJSON(&b, m); e != nil {
		t.Fatal(e)
	}
	var d revolutionSurfacesJSON
	if e = json.Unmarshal(b.Bytes(), &d); e != nil {
		t.Fatal(e)
	}
	d.Surfaces[0].Profile.Weights = []float64{1, .5, 1}
	raw, _ := json.Marshal(d)
	if got, e := ReadRevolutionSurfacesJSON(bytes.NewReader(raw), RevolutionSurfaceJSONLimits{}); e == nil || len(got.Surfaces) != 0 {
		t.Fatal("forged v1 weighted accepted")
	}
}
