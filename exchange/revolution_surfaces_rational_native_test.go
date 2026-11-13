package exchange

import (
	"bytes"
	"encoding/json"
	"plainsolid/geom"
	"testing"
)

func TestRationalRevolutionSurfacesNativeVersions(t *testing.T) {
	m := revolutionNativeFixture(t)
	var polynomial bytes.Buffer
	if e := WriteRevolutionSurfacesJSON(&polynomial, m); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadRationalRevolutionSurfacesJSON(bytes.NewReader(polynomial.Bytes()), RevolutionSurfaceJSONLimits{}); e != nil {
		t.Fatal("v1 compatibility", e)
	}
	c := m.Surfaces[0].Profile.Geometry
	c, e := geom.NewBSpline(c.Degree(), c.Poles(), c.Knots(), []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	m.Surfaces[0].Profile.Geometry = c
	var b bytes.Buffer
	if e = WriteRationalRevolutionSurfacesJSON(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	got, e := ReadRationalRevolutionSurfacesJSON(&b, RevolutionSurfaceJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	surface, e := got.Surfaces[0].Surface()
	if e != nil {
		t.Fatal(e)
	}
	point, e := surface.Eval(.5, .5)
	if e != nil || point.Z != 4./3 {
		t.Fatal(point, e)
	}
	if got.Surfaces[0].Profile.Geometry.Weights()[1] != .5 {
		t.Fatal("lost weights")
	}
	var again bytes.Buffer
	if e = WriteRationalRevolutionSurfacesJSON(&again, got); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("canonical", e)
	}
	if _, e = ReadRevolutionSurfacesJSON(bytes.NewReader(raw), RevolutionSurfaceJSONLimits{}); e == nil {
		t.Fatal("v1 reader accepted v2")
	}
	for _, version := range []int{0, 1, 3} {
		var d revolutionSurfacesJSON
		if e = json.Unmarshal(raw, &d); e != nil {
			t.Fatal(e)
		}
		d.Version = version
		bad, _ := json.Marshal(d)
		got, e = ReadRationalRevolutionSurfacesJSON(bytes.NewReader(bad), RevolutionSurfaceJSONLimits{})
		if e == nil || len(got.Surfaces) != 0 {
			t.Fatal("invalid version/geometry", version)
		}
	}

}
func TestRationalRevolutionSurfacesNativeProofAndLimits(t *testing.T) {
	m := revolutionNativeFixture(t)
	c := m.Surfaces[0].Profile.Geometry
	c, e := geom.NewBSpline(c.Degree(), c.Poles(), c.Knots(), []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	m.Surfaces[0].Profile.Geometry = c
	var b bytes.Buffer
	if e = WriteRationalRevolutionSurfacesJSON(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	for _, mutate := range []func(*revolutionSurfacesJSON){func(d *revolutionSurfacesJSON) { d.Surfaces[0].Profile.Poles[1].Z = 4 }, func(d *revolutionSurfacesJSON) { d.Surfaces[0].Profile.Weights[1] = 0 }, func(d *revolutionSurfacesJSON) { d.AngularUnit = "degree" }} {
		var d revolutionSurfacesJSON
		_ = json.Unmarshal(raw, &d)
		mutate(&d)
		bad, _ := json.Marshal(d)
		if got, e := ReadRationalRevolutionSurfacesJSON(bytes.NewReader(bad), RevolutionSurfaceJSONLimits{}); e == nil || len(got.Surfaces) != 0 {
			t.Fatal("invalid rational accepted")
		}
	}
	for _, l := range []RevolutionSurfaceJSONLimits{{MaxBytes: 16}, {MaxPoles: 2}} {
		if _, e := ReadRationalRevolutionSurfacesJSON(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
}
