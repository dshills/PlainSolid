package iges

import (
	"bytes"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestIndependentPolyline106(t *testing.T) {
	data, err := os.ReadFile("testdata/polyline.iges")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Read(bytes.NewReader(data), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	e := m.Entities[0]
	if e.Polyline == nil || e.Form != 12 || e.Label != "PATH" || m.LengthUnit != .001 {
		t.Fatal(e)
	}
	for i, want := range []geom.Vec3{{}, {X: 1}, {X: 2}, {X: 2, Y: 2, Z: 3}, {X: 2, Y: 4, Z: 6}} {
		p, err := e.Eval(float64(i) / 4)
		if err != nil || p != want {
			t.Fatal(i, p, err)
		}
	}
	// Three polyline points and three spline controls share one aggregate budget.
	if _, err := Read(bytes.NewReader(data), Limits{MaxControlPoints: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(bytes.NewReader(data), Limits{MaxControlPoints: 5}); err == nil {
		t.Fatal("aggregate budget")
	}
	if _, err := Read(bytes.NewReader(data), Limits{MaxControlPoints: 2}); err == nil {
		t.Fatal("point allocation budget")
	}
	for _, params := range []string{"106,2,999999999,0,0;", "106,1,2,0,0,0,1,1,1;", "106,2,2,0,0,0;", "106,2,2,0,0,0,0,0,0;", "106,2,2,0,0,0,1.E999,1,1;"} {
		lines := strings.Split(string(data), "\n")
		lines[9] = params + strings.Repeat(" ", 64-len(params)) + lines[9][64:]
		if _, err := Read(strings.NewReader(strings.Join(lines, "\n")), Limits{}); err == nil {
			t.Fatal("accepted", params)
		}
	}
	for _, form := range []string{"2", "11", "13"} {
		if _, err := Read(strings.NewReader(replaceField(string(data), 'D', 2, 4, form)), Limits{}); err == nil {
			t.Fatal("accepted form", form)
		}
	}
}
func TestPolylinePlacedRoundTrip(t *testing.T) {
	tr := geom.Translation(geom.Vec3{X: 5, Y: 6, Z: 7})
	point := geom.Vec3{Z: 1}
	m := Model{LengthUnit: .0254, Entities: []Entity{{Form: 12, Transform: &tr, Polyline: &Polyline{Points: []geom.Vec3{{}, {X: 2}, {X: 2, Y: 4}}}}, {Point: &point}}}
	var b bytes.Buffer
	if err := Write(&b, m); err != nil {
		t.Fatal(err)
	}
	got, err := Read(bytes.NewReader(b.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := got.Entities[0].Eval(.75)
	if err != nil || p != (geom.Vec3{X: 7, Y: 8, Z: 7}) || got.LengthUnit != m.LengthUnit {
		t.Fatal(p, err)
	}
	var again bytes.Buffer
	if err := Write(&again, got); err != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("unstable", err)
	}
	m.Entities[0].Form = 2
	var bad bytes.Buffer
	if err := Write(&bad, m); err == nil || bad.Len() != 0 {
		t.Fatal("point set accepted as connected")
	}
}
