package occtcheck

import (
	"context"
	"plainsolid/brep"
	"plainsolid/csg/feature"
	"plainsolid/geom"
	"testing"
)

func TestOCCTAnnularFeatures(t *testing.T) {
	refs := runOracle(t, "feature")
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	square := func(lo, hi float64) []geom.Vec3 {
		return []geom.Vec3{{X: lo, Y: lo, Z: 4}, {X: hi, Y: lo, Z: 4}, {X: hi, Y: hi, Z: 4}, {X: lo, Y: hi, Z: 4}}
	}
	outer, hole := square(.5, 3.5), square(1.5, 2.5)
	for mode, name := range []string{"annular-protrusion", "annular-blind", "annular-through"} {
		t.Run(name, func(t *testing.T) {
			var r feature.HoledResult
			var e error
			if mode == 2 {
				r, e = feature.ThroughPocketWithHoles(context.Background(), s, 1, 0, outer, [][]geom.Vec3{hole}, geom.Vec3{Z: -4}, feature.Options{})
			} else {
				kind, dz := feature.Protrusion, 1.
				if mode == 1 {
					kind, dz = feature.Pocket, -1
				}
				r, e = feature.ExtrudeWithHoles(context.Background(), s, 1, outer, [][]geom.Vec3{hole}, geom.Vec3{Z: dz}, kind, feature.Options{})
			}
			if e != nil {
				t.Fatal(e)
			}
			m, e := r.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			compare(t, refs[name], m.SurfaceArea, m.Volume, m.Centroid)
		})
	}
}
