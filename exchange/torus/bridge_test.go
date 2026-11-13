package torus

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

func fixture() iges.ToroidalSupportModel {
	return iges.ToroidalSupportModel{LengthUnit: .3048, Surfaces: []iges.ToroidalSupport{{Center: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: -1}, Reference: geom.Vec3{Z: -1}, MajorRadius: 4, MinorRadius: 1.25, Metadata: iges.TorusDatumMetadata{Label: "TORUS"}, CenterMetadata: iges.TorusDatumMetadata{Label: "CENTER"}, AxisMetadata: iges.TorusDatumMetadata{Label: "AXIS"}, ReferenceMetadata: iges.TorusDatumMetadata{Label: "RADIAL"}}}}
}
func TestTorusBridgeRoundtrip(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		m := fixture()
		m.Surfaces[0].Axis.X = sign
		s, e := FromIGES(m)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = step.WriteTorusSupports(&b, s); e != nil {
			t.Fatal(e)
		}
		s, e = step.ReadTorusSupports(&b, step.Limits{})
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
		if e = iges.WriteToroidalSupports(&b, got); e != nil {
			t.Fatal(e)
		}
		got, e = iges.ReadToroidalSupports(&b, iges.Limits{})
		if e != nil || !reflect.DeepEqual(m, got) {
			t.Fatal(e, got)
		}
		got.Surfaces[0].Center.X = 9
		if m.Surfaces[0].Center.X != 2 {
			t.Fatal("alias")
		}
	}
}
func TestTorusBridgeGates(t *testing.T) {
	m := fixture()
	for i := 0; i < 4; i++ {
		bad := fixture()
		s := &bad.Surfaces[0]
		meta := []*iges.TorusDatumMetadata{&s.Metadata, &s.CenterMetadata, &s.AxisMetadata, &s.ReferenceMetadata}
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
	for _, mut := range []func(*step.TorusSupportModel){func(s *step.TorusSupportModel) { s.Name = "model" }, func(s *step.TorusSupportModel) { s.SetName = "set" }, func(s *step.TorusSupportModel) { s.Surfaces[0].PlacementName = "frame" }, func(s *step.TorusSupportModel) { s.Surfaces[0].Name = "overlong-label" }, func(s *step.TorusSupportModel) { s.Surfaces[0].Axis.X = 2 }} {
		bad := s
		bad.Surfaces = append([]step.TorusSupport(nil), s.Surfaces...)
		mut(&bad)
		out, e := ToIGES(bad)
		if !errors.Is(e, ErrUnsupportedConversion) || len(out.Surfaces) != 0 {
			t.Fatal(e)
		}
	}
}
func TestTorusBridgeIndependentFixtures(t *testing.T) {
	b, e := os.ReadFile("../iges/testdata/toroidal-support.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := iges.ReadToroidalSupports(bytes.NewReader(b), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = FromIGES(m); e != nil {
		t.Fatal(e)
	}
	b, e = os.ReadFile("../step/testdata/torus-support.step")
	if e != nil {
		t.Fatal(e)
	}
	s, e := step.ReadTorusSupports(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ToIGES(s); e != nil {
		t.Fatal(e)
	}
}
