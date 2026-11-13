package extrusion

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"os"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
)

func fixture(t *testing.T) iges.Model {
	t.Helper()
	b, e := os.ReadFile("../iges/testdata/tabulated-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := iges.ReadTabulatedSurfaces(bytes.NewReader(b), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.TabulatedSurfaces[0].Label = "SWEEP"
	m.Entities[0].Label = "PROFILE"
	return m
}
func TestExtrusionBridgeExactDomains(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		source := fixture(t)
		source.LengthUnit = .3048
		source.TabulatedSurfaces[0].End.Y *= sign
		source.TabulatedSurfaces[0].End.Z *= sign
		m, e := FromIGES(source)
		if e != nil {
			t.Fatal(e)
		}
		s := m.Surfaces[0]
		if s.VStart != 0 || s.VEnd != 5 || s.DirectionRatios()[1].Cmp(big.NewRat(int64(sign)*3, 5)) != 0 {
			t.Fatal(s)
		}
		var st bytes.Buffer
		if e = step.WriteRationalLinearExtrusions(&st, m); e != nil {
			t.Fatal(e)
		}
		m, e = step.ReadRationalLinearExtrusions(&st, step.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if m.LengthUnit != source.LengthUnit {
			t.Fatal("units")
		}
		out, e := ToIGES(m)
		if e != nil {
			t.Fatal(e)
		}
		var a, b bytes.Buffer
		if e = iges.WriteTabulatedSurfaces(&a, source); e != nil {
			t.Fatal(e)
		}
		if e = iges.WriteTabulatedSurfaces(&b, out); e != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatal("lost metadata or displacement", e)
		}
		g, _ := source.TabulatedGeometry(0)
		h, _ := m.Surfaces[0].Surface()
		gp, _ := geom.NewSurfacePatch(g, geom.SurfacePatchOptions{})
		hp, _ := geom.NewSurfacePatch(h, geom.SurfacePatchOptions{})
		gj, e := gp.EvaluateRational(context.Background(), big.NewRat(1, 3), big.NewRat(2, 7), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		hj, e := hp.EvaluateRational(context.Background(), big.NewRat(1, 3), big.NewRat(10, 7), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 3; i++ {
			if gj.Point[i].Cmp(hj.Point[i]) != 0 || gj.Du[i].Cmp(hj.Du[i]) != 0 || gj.Dv[i].Cmp(new(big.Rat).Mul(big.NewRat(5, 1), hj.Dv[i])) != 0 {
				t.Fatal("distance/fraction mismatch")
			}
		}
		*out.Entities[0].Spline.Curve = geom.BSpline{}
		if _, e = g.Eval(.5, .5); e != nil {
			t.Fatal("alias", e)
		}
	}
}
func TestExtrusionBridgeUnsupportedAndProof(t *testing.T) {
	for _, tc := range []struct {
		field  string
		change func(*iges.Model)
	}{
		{"surface.subscript", func(m *iges.Model) { m.TabulatedSurfaces[0].Subscript = 1 }},
		{"profile.subscript", func(m *iges.Model) { m.Entities[0].Subscript = 1 }},
		{"profile.form", func(m *iges.Model) { m.Entities[0].Form = 1 }},
		{"profile.plane", func(m *iges.Model) { m.Entities[0].Spline.Normal.Y = 1 }},
		{"distance", func(m *iges.Model) { m.TabulatedSurfaces[0].End = geom.Vec3{X: 2, Y: 1, Z: 1} }},
	} {
		m := fixture(t)
		tc.change(&m)
		got, e := FromIGES(m)
		var u *UnsupportedConversionError
		if !errors.Is(e, ErrUnsupportedConversion) || !errors.As(e, &u) || u.Field != tc.field || len(got.Surfaces) != 0 {
			t.Fatal(tc.field, e)
		}
	}
	m := fixture(t)
	m.TabulatedSurfaces[0].End = geom.Vec3{X: 2, Z: 5}
	got, e := FromIGES(m)
	if !errors.Is(e, ErrInconclusiveProof) || errors.Is(e, ErrUnsupportedConversion) || len(got.Surfaces) != 0 {
		t.Fatal("proof status", e)
	}
}
func TestExtrusionBridgeSTEPGates(t *testing.T) {
	for _, tc := range []struct {
		field  string
		change func(*step.RationalLinearExtrusionModel)
	}{
		{"domain", func(m *step.RationalLinearExtrusionModel) { m.Surfaces[0].VStart = 5; m.Surfaces[0].VEnd = 10 }},
		{"profile.flags", func(m *step.RationalLinearExtrusionModel) { m.Surfaces[0].Profile.SelfIntersect = "U" }},
		{"surface.name", func(m *step.RationalLinearExtrusionModel) { m.Surfaces[0].Name = "too long name" }},
		{"direction.scale", func(m *step.RationalLinearExtrusionModel) {
			s := m.Surfaces[0]
			q, e := step.NewRationalLinearExtrusionSurface(s.Name, s.Profile, [3]*big.Rat{new(big.Rat), big.NewRat(3, 1), big.NewRat(4, 1)}, 0, 5)
			if e != nil {
				t.Fatal(e)
			}
			m.Surfaces[0] = q
		}},
	} {
		m, e := FromIGES(fixture(t))
		if e != nil {
			t.Fatal(e)
		}
		tc.change(&m)
		out, e := ToIGES(m)
		var u *UnsupportedConversionError
		if !errors.As(e, &u) || u.Field != tc.field || len(out.Entities) != 0 {
			t.Fatal(tc.field, e)
		}
	}
}
