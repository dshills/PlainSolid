package analytic

import (
	"bytes"
	"errors"
	"os"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/sphere"
	"plainsolid/exchange/step"
	"reflect"
	"testing"
)

func fixture(t *testing.T) iges.AnalyticSupportModel {
	t.Helper()
	b, e := os.ReadFile("../iges/testdata/analytic-supports.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := iges.ReadAnalyticSupports(bytes.NewReader(b), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestAnalyticBridgeRoundtrip(t *testing.T) {
	m := fixture(t)
	m.LengthUnit = .3048
	s, e := FromIGES(m)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = step.WriteAnalyticSupports(&b, s); e != nil {
		t.Fatal(e)
	}
	s, e = step.ReadAnalyticSupports(&b, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	back, e := ToIGES(s)
	if e != nil || !reflect.DeepEqual(back, m) {
		t.Fatal(back, e)
	}
	if e = iges.WriteAnalyticSupports(&b, back); e != nil {
		t.Fatal(e)
	}
	back, e = iges.ReadAnalyticSupports(&b, iges.Limits{})
	if e != nil || !reflect.DeepEqual(back, m) {
		t.Fatal(back, e)
	}
	back.Surfaces[0].Torus.Center.X = 99
	if m.Surfaces[0].Torus.Center.X == 99 {
		t.Fatal("alias")
	}
	b.Reset()
	raw, e := os.ReadFile("../step/testdata/analytic-supports.step")
	if e != nil {
		t.Fatal(e)
	}
	s, e = step.ReadAnalyticSupports(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	im, e := ToIGES(s)
	if e != nil {
		t.Fatal(e)
	}
	got, e := FromIGES(im)
	if e != nil || !reflect.DeepEqual(got, s) {
		t.Fatal(got, e)
	}
}
func TestAnalyticBridgeIndexedAtomicFailure(t *testing.T) {
	m := fixture(t)
	m.Surfaces[2].Sphere.Metadata.Subscript = 7
	out, e := FromIGES(m)
	var indexed *ConversionError
	var detail *sphere.UnsupportedConversionError
	if len(out.Surfaces) != 0 || !errors.Is(e, ErrUnsupportedConversion) || !errors.As(e, &indexed) || indexed.Support != 2 || !errors.As(e, &detail) {
		t.Fatal(out, e)
	}
	m = fixture(t)
	s, e := FromIGES(m)
	if e != nil {
		t.Fatal(e)
	}
	s.Name = "not transferable"
	got, e := ToIGES(s)
	if len(got.Surfaces) != 0 || !errors.As(e, &indexed) || indexed.Support != -1 {
		t.Fatal(e)
	}
	s.Name = ""
	s.Surfaces[3].Plane = s.Surfaces[1].Plane
	got, e = ToIGES(s)
	if len(got.Surfaces) != 0 || !errors.As(e, &indexed) || indexed.Support != 3 {
		t.Fatal(e)
	}
}
