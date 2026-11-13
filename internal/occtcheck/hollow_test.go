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
