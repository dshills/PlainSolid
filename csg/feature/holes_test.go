package feature

import (
	"context"
	"math"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"testing"
)

func TestAnnularSketchFeatures(t *testing.T) {
	s, _ := fixture(t)
	outer := square(.5, .5, 3)
	hole := square(1.5, 1.5, 1)
	for _, mode := range []int{0, 1, 2} {
		var r HoledResult
		var e error
		want := 72.
		if mode == 0 {
			r, e = ExtrudeWithHoles(context.Background(), s, 1, outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 1}, Protrusion, Options{})
		} else if mode == 1 {
			want = 56
			r, e = ExtrudeWithHoles(context.Background(), s, 1, outer, [][]geom.Vec3{hole}, geom.Vec3{Z: -1}, Pocket, Options{})
		} else {
			want = 32
			r, e = ThroughPocketWithHoles(context.Background(), s, 1, 0, outer, [][]geom.Vec3{hole}, geom.Vec3{Z: -4}, Options{})
		}
		if e != nil {
			t.Fatal(mode, e)
		}
		m, e := r.Solid.MassProperties()
		if e != nil || math.Abs(m.Volume-want) > 1e-10 {
			t.Fatal(mode, e, m)
		}
		shells := 1
		if mode == 2 {
			shells = 2
		}
		if r.Solid.Topology().Shells != shells {
			t.Fatal(mode, "shells")
		}
		seen := map[int]bool{}
		for _, src := range r.History.Faces {
			if src.Operand == poly.OperandB {
				seen[src.Face] = true
			}
		}
		if len(r.HoleWalls) != 1 || len(r.HoleWalls[0]) != 4 {
			t.Fatal("hole wall map")
		}
		for _, wall := range r.HoleWalls[0] {
			if !seen[wall] {
				t.Fatal("hole wall ancestry")
			}
		}
		if mode == 2 && (seen[r.Mouth] || seen[r.Terminal]) {
			t.Fatal("through cap survives")
		}
	}
}
func TestHoledSketchRejectsInvalidRings(t *testing.T) {
	s, _ := fixture(t)
	outer := square(.5, .5, 3)
	for _, holes := range [][][]geom.Vec3{{square(3, 3, .75)}, {square(1, 1, 1), square(1.5, 1.5, 1)}, {square(.5, 1, 1)}} {
		r, e := ExtrudeWithHoles(context.Background(), s, 1, outer, holes, geom.Vec3{Z: -1}, Pocket, Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("accepted hole conflict")
		}
	}
}
