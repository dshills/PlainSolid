package sphere

import (
	"bytes"
	"errors"
	"os"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func fixture() iges.SphericalSupportModel {
	return iges.SphericalSupportModel{LengthUnit: .3048, Surfaces: []iges.SphericalSupport{{Center: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: -1}, Reference: geom.Vec3{Z: -1}, Radius: 2.5, Metadata: iges.SphereDatumMetadata{Label: "SPHERE"}, CenterMetadata: iges.SphereDatumMetadata{Label: "CENTER"}, AxisMetadata: iges.SphereDatumMetadata{Label: "AXIS"}, ReferenceMetadata: iges.SphereDatumMetadata{Label: "RADIAL"}}}}
}
func TestSphereBridgeRoundtrip(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		m := fixture()
		m.Surfaces[0].Axis.X = sign
		s, e := FromIGES(m)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = step.WriteSphereSupports(&b, s); e != nil {
			t.Fatal(e)
		}
		s, e = step.ReadSphereSupports(&b, step.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		got, e := ToIGES(s)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(m, got) {
			t.Fatal(got, m)
		}
		if e = iges.WriteSphericalSupports(&b, got); e != nil {
			t.Fatal(e)
		}
		got, e = iges.ReadSphericalSupports(&b, iges.Limits{})
		if e != nil || !reflect.DeepEqual(m, got) {
			t.Fatal(e, got)
		}
		got.Surfaces[0].Center.X = 9
		if m.Surfaces[0].Center.X != 2 {
			t.Fatal("alias")
		}
	}
}
func TestSphereBridgeGates(t *testing.T) {
	m := fixture()
	for i := 0; i < 4; i++ {
		bad := fixture()
		s := &bad.Surfaces[0]
		meta := []*iges.SphereDatumMetadata{&s.Metadata, &s.CenterMetadata, &s.AxisMetadata, &s.ReferenceMetadata}
		meta[i].Subscript = 1
		out, e := FromIGES(bad)
		if !errors.Is(e, ErrUnsupportedConversion) || len(out.Surfaces) != 0 {
			t.Fatal(e)
		}
	}
	s, e := FromIGES(m)
	if e != nil {
		t.Fatal(e)
	}
	for _, mut := range []func(*step.SphereSupportModel){func(s *step.SphereSupportModel) { s.Name = "model" }, func(s *step.SphereSupportModel) { s.SetName = "set" }, func(s *step.SphereSupportModel) { s.Surfaces[0].PlacementName = "frame" }, func(s *step.SphereSupportModel) { s.Surfaces[0].Name = "overlong-label" }, func(s *step.SphereSupportModel) { s.Surfaces[0].Axis.X = 2 }} {
		bad := s
		bad.Surfaces = append([]step.SphereSupport(nil), s.Surfaces...)
		mut(&bad)
		out, e := ToIGES(bad)
		if !errors.Is(e, ErrUnsupportedConversion) || len(out.Surfaces) != 0 {
			t.Fatal(e)
		}
	}
}
func TestSphereBridgeIndependentFixtures(t *testing.T) {
	b, e := os.ReadFile("../iges/testdata/spherical-support.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := iges.ReadSphericalSupports(bytes.NewReader(b), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = FromIGES(m); e != nil {
		t.Fatal(e)
	}
	b, e = os.ReadFile("../step/testdata/sphere-support.step")
	if e != nil {
		t.Fatal(e)
	}
	s, e := step.ReadSphereSupports(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ToIGES(s); e != nil {
		t.Fatal(e)
	}
}
