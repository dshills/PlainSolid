package occtcheck

import (
	"context"
	"plainsolid/brep"
	"plainsolid/csg/draft"
	"plainsolid/geom"
	"testing"
)

func TestOCCTHollow(t *testing.T) {
	refs := runOracle(t, "feature")
	for _, open := range []int{-1, 1} {
		name := "hollow-cavity"
		thickness := []float64{.5, .5, .5, .5, .5, .5}
		if open == 1 {
			name = "hollow-tray"
			thickness[1] = 0
		}
		t.Run(name, func(t *testing.T) {
			s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
			if e != nil {
				t.Fatal(e)
			}
			r, e := draft.Hollow(context.Background(), s, thickness, draft.HollowOptions{OpenFace: open})
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

func TestOCCTHollowThrough(t *testing.T) {
	refs := runOracle(t, "feature")
	for _, name := range []string{"hollow-through-box", "hollow-through-sheared"} {
		t.Run(name, func(t *testing.T) {
			s, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
			if err != nil {
				t.Fatal(err)
			}
			shear := name == "hollow-through-sheared"
			if shear {
				s, err = s.Transformed(geom.Affine{{1, 0, .5, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
				if err != nil {
					t.Fatal(err)
				}
			}
			r, err := draft.HollowThrough(context.Background(), s, []float64{0, 0, .25, .5, .75, 1},
				draft.ThroughHollowOptions{OpenFaces: [2]int{0, 1}, CoordinateThickness: shear})
			if err != nil {
				t.Fatal(err)
			}
			top := r.Solid.Topology()
			ref, ok := refs[name]
			if !ok || top.Shells != 1 || top.EulerCharacteristic != 0 || r.ExactVolume.RatString() != "60" {
				t.Fatal("expected a genus-one shell and exact volume 60", top, r.ExactVolume)
			}
			m, err := r.Solid.MassProperties()
			if err != nil {
				t.Fatal(err)
			}
			compare(t, ref, m.SurfaceArea, m.Volume, m.Centroid)
		})
	}
}
