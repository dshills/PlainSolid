package revolution

import (
	"bytes"
	"errors"
	"os"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
)

func bridgeFixture(t *testing.T) iges.Model {
	t.Helper()
	b, e := os.ReadFile("../iges/testdata/revolution-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := iges.ReadRevolutionSurfaces(bytes.NewReader(b), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Revolutions[0].Label = "SURFACE"
	m.Entities[m.Revolutions[0].Generatrix].Label = "PROFILE"
	return m
}
func TestIGESBridgeNativeSTEPChain(t *testing.T) {
	source := bridgeFixture(t)
	m, e := FromIGES(source)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteRationalRevolutionSurfacesJSON(&native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadRationalRevolutionSurfacesJSON(&native, RevolutionSurfaceJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var st bytes.Buffer
	if e = step.WriteRevolutionSurfaces(&st, m); e != nil {
		t.Fatal(e)
	}
	m, e = step.ReadRevolutionSurfaces(&st, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	out, e := ToIGES(m)
	if e != nil {
		t.Fatal(e)
	}
	var a, b bytes.Buffer
	if e = iges.WriteRevolutionSurfaces(&a, source); e != nil {
		t.Fatal(e)
	}
	if e = iges.WriteRevolutionSurfaces(&b, out); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("metadata/geometry changed")
	}
	g, _ := source.RevolutionGeometry(0)
	h, _ := out.RevolutionGeometry(0)
	for _, v := range []float64{0, .25, .5, 1} {
		p, _ := g.Eval(.5, v)
		q, _ := h.Eval(.5, v)
		du, dv, _ := g.Partials(.5, v)
		eu, ev, _ := h.Partials(.5, v)
		if p != q || du != eu || dv != ev {
			t.Fatal("parameter mismatch")
		}
	}
	*out.Entities[1].Spline.Curve = geom.BSpline{}
	if _, e = g.Eval(.5, .5); e != nil {
		t.Fatal("geometry alias", e)
	}
}
func TestIGESBridgeMetadataRejects(t *testing.T) {
	for _, tc := range []struct {
		field  string
		mutate func(*iges.Model)
	}{
		{"axis.label", func(m *iges.Model) { m.Entities[0].Label = "AXIS" }},
		{"axis.subscript", func(m *iges.Model) { m.Entities[0].Subscript = 1 }},
		{"profile.subscript", func(m *iges.Model) { m.Entities[1].Subscript = 1 }},
		{"surface.subscript", func(m *iges.Model) { m.Revolutions[0].Subscript = 1 }},
		{"directory.form", func(m *iges.Model) { m.Entities[1].Form = 1 }},
		{"profile.planar", func(m *iges.Model) { m.Entities[1].Spline.Planar = false }},
		{"profile.normal", func(m *iges.Model) { m.Entities[1].Spline.Normal.Y = 1 }},
		{"dependency.order", func(m *iges.Model) {
			m.Entities[0], m.Entities[1] = m.Entities[1], m.Entities[0]
			m.Revolutions[0].Axis = 1
			m.Revolutions[0].Generatrix = 0
		}},
	} {
		m := bridgeFixture(t)
		tc.mutate(&m)
		got, e := FromIGES(m)
		var unsupportedError *UnsupportedConversionError
		if !errors.Is(e, ErrUnsupportedConversion) || !errors.As(e, &unsupportedError) || unsupportedError.Field != tc.field || len(got.Surfaces) != 0 {
			t.Fatal(tc.field, e)
		}
	}
}
func TestIGESBridgeSTEPRejects(t *testing.T) {
	for _, tc := range []struct {
		field  string
		mutate func(*step.RevolutionSurfaceModel)
	}{
		{"profile.closed", func(m *step.RevolutionSurfaceModel) { m.Surfaces[0].Profile.Closed = "U" }},
		{"profile.self_intersect", func(m *step.RevolutionSurfaceModel) { m.Surfaces[0].Profile.SelfIntersect = "" }},
		{"surface.name", func(m *step.RevolutionSurfaceModel) { m.Surfaces[0].Name = "long surface" }},
		{"surface.name", func(m *step.RevolutionSurfaceModel) { m.Surfaces[0].Name = " NAME " }},
		{"profile.name", func(m *step.RevolutionSurfaceModel) { m.Surfaces[0].Profile.Name = "α" }},
		{"iges.representation", func(m *step.RevolutionSurfaceModel) { m.LengthUnit = .123 }},
		{"profile.normal", func(m *step.RevolutionSurfaceModel) {
			s := &m.Surfaces[0]
			p := s.Profile.Geometry.Poles()
			for i := range p {
				p[i].Y = 4 * p[i].X
				p[i].X *= 3
			}
			c, _ := geom.NewBSpline(2, p, s.Profile.Geometry.Knots(), s.Profile.Geometry.Weights())
			s.Profile.Geometry = c
		}},
	} {
		m, e := FromIGES(bridgeFixture(t))
		if e != nil {
			t.Fatal(e)
		}
		tc.mutate(&m)
		out, e := ToIGES(m)
		var u *UnsupportedConversionError
		if !errors.As(e, &u) || u.Field != tc.field || len(out.Entities) != 0 {
			t.Fatal(tc.field, e)
		}
	}
}

func TestIGESBridgeSignedUnits(t *testing.T) {
	source := bridgeFixture(t)
	source.LengthUnit = .3048
	source.Entities[0].Line.End.Z = -1
	source.Revolutions[0].AngleStart = -1.25
	source.Revolutions[0].AngleEnd = -.25
	m, e := FromIGES(source)
	if e != nil {
		t.Fatal(e)
	}
	out, e := ToIGES(m)
	if e != nil {
		t.Fatal(e)
	}
	var a, b bytes.Buffer
	if e = iges.WriteRevolutionSurfaces(&a, source); e != nil {
		t.Fatal(e)
	}
	if e = iges.WriteRevolutionSurfaces(&b, out); e != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("signed/unit metadata", e)
	}
}
func TestIGESBridgeUnrepresentableAxisEndpoint(t *testing.T) {
	m, e := FromIGES(bridgeFixture(t))
	if e != nil {
		t.Fatal(e)
	}
	s := &m.Surfaces[0]
	s.Origin.Z = 1e16
	p := s.Profile.Geometry.Poles()
	for i := range p {
		p[i].Z = 1e16 + 2*float64(i)
	}
	c, e := geom.NewBSpline(2, p, s.Profile.Geometry.Knots(), s.Profile.Geometry.Weights())
	if e != nil {
		t.Fatal(e)
	}
	s.Profile.Geometry = c
	out, e := ToIGES(m)
	var u *UnsupportedConversionError
	if !errors.As(e, &u) || u.Field != "axis.end" || len(out.Entities) != 0 {
		t.Fatal(e)
	}
}
