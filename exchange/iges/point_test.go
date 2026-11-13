package iges

import (
	"bytes"
	"math"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestIndependentPoint116(t *testing.T) {
	data, e := os.ReadFile("testdata/points.iges")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(data), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	want := geom.Vec3{X: 10, Y: -2, Z: 3.5}
	if m.Entities[0].Point == nil || *m.Entities[0].Point != want || m.Entities[0].Label != "POINT" || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	for _, v := range []float64{0, .5, 1} {
		p, e := m.Entities[0].Eval(v)
		if e != nil || p != want {
			t.Fatal(p, e)
		}
	}
	for _, limit := range []Limits{{MaxEntities: 2}, {MaxValues: 3}, {MaxBytes: 80}, {MaxRecords: 3}} {
		if _, e := Read(bytes.NewReader(data), limit); e == nil {
			t.Fatal("ignored limit", limit)
		}
	}
	for _, text := range []string{"116,1.,2.,3.,4,0,0;", "116,1.,2.,3.;", "116,1.,2.,1.E999,0;", "116,1.,2.,3.,0,1,0;"} {
		lines := strings.Split(string(data), "\n")
		lines[9] = text + strings.Repeat(" ", 64-len(text)) + lines[9][64:]
		if _, e := Read(strings.NewReader(strings.Join(lines, "\n")), Limits{}); e == nil {
			t.Fatal("accepted", text)
		}
	}
}
func TestPoint116MixedRoundTrip(t *testing.T) {
	p := geom.Vec3{X: 1, Y: 2, Z: 3}
	tr := geom.Translation(geom.Vec3{X: 4, Y: -2, Z: 8})
	m := Model{LengthUnit: .001, Entities: []Entity{{Point: &p, Transform: &tr, Label: "DATUM"}, {Line: &Line{Start: geom.Vec3{}, End: geom.Vec3{X: 1}}}}}
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := Read(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := got.Entities[0].Eval(.3)
	if e != nil || q != (geom.Vec3{X: 5, Z: 11}) {
		t.Fatal(q, e)
	}
	var again bytes.Buffer
	if e := Write(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("unstable roundtrip", e)
	}
	for _, bad := range []Entity{{Point: &p, Form: 1}, {Point: &geom.Vec3{X: math.NaN()}}, {Point: &p, Line: m.Entities[1].Line}} {
		var out bytes.Buffer
		if e := Write(&out, Model{LengthUnit: .001, Entities: []Entity{bad}}); e == nil || out.Len() != 0 {
			t.Fatal("invalid point published", e)
		}
	}
}
