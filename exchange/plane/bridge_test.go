package plane

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

func fixture() iges.PlanarSupportModel {
	return iges.PlanarSupportModel{LengthUnit: .3048, Surfaces: []iges.PlanarSupport{{Location: geom.Vec3{X: 2, Y: 3, Z: 4}, Normal: geom.Vec3{X: -1}, Reference: geom.Vec3{Z: -1}, Metadata: iges.PlaneDatumMetadata{Label: "PLANE"}, LocationMetadata: iges.PlaneDatumMetadata{Label: "CENTER"}, NormalMetadata: iges.PlaneDatumMetadata{Label: "AXIS"}, ReferenceMetadata: iges.PlaneDatumMetadata{Label: "RADIAL"}}}}
}
func TestPlaneBridgeRoundtrip(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		m := fixture()
		m.Surfaces[0].Normal.X = sign
		s, e := FromIGES(m)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = step.WritePlaneSupports(&b, s); e != nil {
			t.Fatal(e)
		}
		s, e = step.ReadPlaneSupports(&b, step.Limits{})
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
		if e = iges.WritePlanarSupports(&b, got); e != nil {
			t.Fatal(e)
		}
		got, e = iges.ReadPlanarSupports(&b, iges.Limits{})
		if e != nil || !reflect.DeepEqual(m, got) {
			t.Fatal(e, got)
		}
		got.Surfaces[0].Location.X = 9
		if m.Surfaces[0].Location.X != 2 {
			t.Fatal("alias")
		}
	}
}
func TestPlaneBridgeGates(t *testing.T) {
	m := fixture()
	for i := 0; i < 4; i++ {
		bad := fixture()
		s := &bad.Surfaces[0]
		meta := []*iges.PlaneDatumMetadata{&s.Metadata, &s.LocationMetadata, &s.NormalMetadata, &s.ReferenceMetadata}
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
	for _, mut := range []func(*step.PlaneSupportModel){func(s *step.PlaneSupportModel) { s.Name = "model" }, func(s *step.PlaneSupportModel) { s.SetName = "set" }, func(s *step.PlaneSupportModel) { s.Surfaces[0].PlacementName = "frame" }, func(s *step.PlaneSupportModel) { s.Surfaces[0].Name = "overlong-label" }, func(s *step.PlaneSupportModel) { s.Surfaces[0].Normal.X = 2 }} {
		bad := s
		bad.Surfaces = append([]step.PlaneSupport(nil), s.Surfaces...)
		mut(&bad)
		out, e := ToIGES(bad)
		if !errors.Is(e, ErrUnsupportedConversion) || len(out.Surfaces) != 0 {
			t.Fatal(e)
		}
	}
}
func TestPlaneBridgeIndependentFixtures(t *testing.T) {
	b, e := os.ReadFile("../iges/testdata/planar-support.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := iges.ReadPlanarSupports(bytes.NewReader(b), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = FromIGES(m); e != nil {
		t.Fatal(e)
	}
	b, e = os.ReadFile("../step/testdata/plane-support.step")
	if e != nil {
		t.Fatal(e)
	}
	s, e := step.ReadPlaneSupports(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ToIGES(s); e != nil {
		t.Fatal(e)
	}
}
