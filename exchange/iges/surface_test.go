package iges

import (
	"bytes"
	"math"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestSurfaceExchange(t *testing.T) {
	w := math.Sqrt(.5)
	g, e := geom.NewBSplineSurface(2, 1, 3, 2, []geom.Vec3{{X: 2}, {X: 2, Z: 2}, {X: 2, Y: 2}, {X: 2, Y: 2, Z: 2}, {Y: 2}, {Y: 2, Z: 2}}, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, w, w, 1, 1})
	if e != nil {
		t.Fatal(e)
	}
	tr := geom.Translation(geom.Vec3{Z: 3})
	entity := Entity{Surface: &Surface{Geometry: g, UStart: 0, UEnd: 1, VStart: .25, VEnd: .75}, Transform: &tr}
	var b bytes.Buffer
	if e = Write(&b, Model{LengthUnit: .001, Entities: []Entity{entity}}); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	m, e := Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, uv := range [][2]float64{{0, 0}, {.5, .5}, {1, 1}} {
		p, e := m.Entities[0].EvalSurface(uv[0], uv[1])
		if e != nil || math.Abs(math.Hypot(p.X, p.Y)-2) > 1e-12 || math.Abs(p.Z-(3.5+uv[1])) > 1e-12 {
			t.Fatal(p, e)
		}
	}
	if _, e = m.Entities[0].Eval(.5); e == nil {
		t.Fatal("curve evaluation accepted")
	}
	if _, e = Read(bytes.NewReader(raw), Limits{MaxControlPoints: 5}); e == nil {
		t.Fatal("control limit")
	}
	entity.Surface.PeriodicU = true
	if e = Write(&b, Model{LengthUnit: 1, Entities: []Entity{entity}}); e == nil {
		t.Fatal("periodic")
	}
}

// Parameters authored independently in IGES order: U varies fastest.
func TestIndependentSurfaceParameters(t *testing.T) {
	raw := "128,1,1,1,1,0,0,1,0,0,0,0,1,1,0,0,1,1,1,1,1,1,0,0,0,2,0,0,0,3,0,2,3,0,0,1,0,1;"
	budget := 1000
	p, e := parseParameters(raw, ',', ';', &budget)
	if e != nil {
		t.Fatal(e)
	}
	ent, e := decode(128, p, 4)
	if e != nil {
		t.Fatal(e)
	}
	v, e := ent.EvalSurface(.25, .75)
	if e != nil || v != (geom.Vec3{X: .5, Y: 2.25}) {
		t.Fatal(v, e)
	}
	for _, n := range []int{10, 20, len(p) - 1} {
		if _, e = decode(128, p[:n], 4); e == nil {
			t.Fatal("truncation", n)
		}
	}
}

func FuzzSurfaceParameters(f *testing.F) {
	f.Add(strings.Join(periodic128Parameters(f, periodic128Fixture(f, "uv")), ",") + ";")
	f.Add("128,1,1,1,1,0,0,1,0,0,0,0,1,1,0,0,1,1,1,1,1,1,0,0,0,2,0,0,0,3,0,2,3,0,0,1,0,1;")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 65536 {
			return
		}
		budget := 3000
		p, e := parseParameters(s, ',', ';', &budget)
		if e != nil {
			return
		}
		ent, e := decodeGeometry(128, p, 100, true)
		if e == nil {
			if e = ent.validate(); e != nil {
				t.Fatal(e)
			}
			if _, e = ent.EvalSurface(.5, .5); e != nil {
				t.Fatal(e)
			}
		}
	})
}
