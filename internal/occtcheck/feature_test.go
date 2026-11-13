package occtcheck

import (
	"context"
	"plainsolid/brep"
	"plainsolid/csg/feature"
	"plainsolid/geom"
	"testing"
)

func TestOCCTPlanarFeatures(t *testing.T) {
	refs := runOracle(t, "feature")
	for _, concave := range []bool{false, true} {
		for _, pocket := range []bool{false, true} {
			name := "protrusion"
			kind := feature.Protrusion
			d := geom.Vec3{Z: 1}
			if pocket {
				name = "pocket"
				kind = feature.Pocket
				d.Z = -1
			}
			p := []geom.Vec3{{X: 1, Y: 1, Z: 4}, {X: 3, Y: 1, Z: 4}, {X: 3, Y: 3, Z: 4}, {X: 1, Y: 3, Z: 4}}
			if concave {
				name = "oblique-" + name
				d.X = .25
				d.Y = .125
				p = []geom.Vec3{{X: 1, Y: 1, Z: 4}, {X: 3, Y: 1, Z: 4}, {X: 3, Y: 2, Z: 4}, {X: 2, Y: 2, Z: 4}, {X: 2, Y: 3, Z: 4}, {X: 1, Y: 3, Z: 4}}
			}
			t.Run(name, func(t *testing.T) {
				s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
				if e != nil {
					t.Fatal(e)
				}
				r, e := feature.Extrude(context.Background(), s, 1, p, d, kind, feature.Options{})
				if e != nil {
					t.Fatal(e)
				}
				m, e := r.Solid.MassProperties()
				if e != nil {
					t.Fatal(e)
				}
				compare(t, refs[name], m.SurfaceArea, m.Volume, m.Centroid)
				top := r.Solid.Topology()
				if top.Shells != 1 || top.EulerCharacteristic != 2 {
					t.Fatal(top)
				}
			})
		}
	}
}
func TestOCCTThroughPockets(t *testing.T) {
	refs := runOracle(t, "feature")
	for _, dx := range []float64{0, .5} {
		name := "through"
		if dx != 0 {
			name = "oblique-through"
		}
		t.Run(name, func(t *testing.T) {
			s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
			if e != nil {
				t.Fatal(e)
			}
			p := []geom.Vec3{{X: 1, Y: 1, Z: 4}, {X: 3, Y: 1, Z: 4}, {X: 3, Y: 3, Z: 4}, {X: 1, Y: 3, Z: 4}}
			r, e := feature.ThroughPocket(context.Background(), s, 1, 0, p, geom.Vec3{X: dx, Z: -4}, feature.Options{})
			if e != nil {
				t.Fatal(e)
			}
			m, e := r.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			compare(t, refs[name], m.SurfaceArea, m.Volume, m.Centroid)
			if r.Solid.Topology().EulerCharacteristic != 0 {
				t.Fatal("expected tunnel")
			}
		})
	}
}
