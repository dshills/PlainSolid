package cylinder

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

func fixture() iges.CylindricalSurfaceModel {
	return iges.CylindricalSurfaceModel{LengthUnit: .3048, Surfaces: []iges.CylindricalSurface{{Location: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: -1}, Reference: geom.Vec3{Z: -1}, Radius: 2.5, Metadata: iges.CylinderDatumMetadata{Label: "CYLINDER"}, LocationMetadata: iges.CylinderDatumMetadata{Label: "CENTER"}, AxisMetadata: iges.CylinderDatumMetadata{Label: "AXIS"}, ReferenceMetadata: iges.CylinderDatumMetadata{Label: "RADIAL"}}}}
}
func TestCylinderBridgeRoundtrip(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		m := fixture()
		m.Surfaces[0].Axis.X = sign
		s, e := FromIGES(m)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = step.WriteCylinderSupports(&b, s); e != nil {
			t.Fatal(e)
		}
		s, e = step.ReadCylinderSupports(&b, step.Limits{})
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
		if e = iges.WriteCylindricalSurfaces(&b, got); e != nil {
			t.Fatal(e)
		}
		got, e = iges.ReadCylindricalSurfaces(&b, iges.Limits{})
		if e != nil || !reflect.DeepEqual(m, got) {
			t.Fatal(e, got)
		}
		got.Surfaces[0].Location.X = 9
		if m.Surfaces[0].Location.X != 2 {
			t.Fatal("alias")
		}
	}
}
func TestCylinderBridgeGates(t *testing.T) {
	m := fixture()
	for i := 0; i < 4; i++ {
		bad := fixture()
		s := &bad.Surfaces[0]
		meta := []*iges.CylinderDatumMetadata{&s.Metadata, &s.LocationMetadata, &s.AxisMetadata, &s.ReferenceMetadata}
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
	for _, mut := range []func(*step.CylinderSupportModel){func(s *step.CylinderSupportModel) { s.Name = "model" }, func(s *step.CylinderSupportModel) { s.SetName = "set" }, func(s *step.CylinderSupportModel) { s.Surfaces[0].PlacementName = "frame" }, func(s *step.CylinderSupportModel) { s.Surfaces[0].Name = "overlong-label" }, func(s *step.CylinderSupportModel) { s.Surfaces[0].Axis.X = 2 }} {
		bad := s
		bad.Surfaces = append([]step.CylinderSupport(nil), s.Surfaces...)
		mut(&bad)
		out, e := ToIGES(bad)
		if !errors.Is(e, ErrUnsupportedConversion) || len(out.Surfaces) != 0 {
			t.Fatal(e)
		}
	}
}
func TestCylinderBridgeIndependentFixtures(t *testing.T) {
	b, e := os.ReadFile("../iges/testdata/cylindrical-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := iges.ReadCylindricalSurfaces(bytes.NewReader(b), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = FromIGES(m); e != nil {
		t.Fatal(e)
	}
	b, e = os.ReadFile("../step/testdata/cylinder-support.step")
	if e != nil {
		t.Fatal(e)
	}
	s, e := step.ReadCylinderSupports(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ToIGES(s); e != nil {
		t.Fatal(e)
	}
}
