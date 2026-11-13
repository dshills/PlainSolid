package feature

import (
	"context"
	"errors"
	"math"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"testing"
)

func square(x, y, w float64) []geom.Vec3 {
	return []geom.Vec3{{X: x, Y: y, Z: 4}, {X: x + w, Y: y, Z: 4}, {X: x + w, Y: y + w, Z: 4}, {X: x, Y: y + w, Z: 4}}
}
func TestDisjointProfileFeatures(t *testing.T) {
	s, _ := fixture(t)
	profiles := [][]geom.Vec3{square(.5, .5, .5), square(2, 2, 1)}
	for _, kind := range []Kind{Protrusion, Pocket} {
		d := geom.Vec3{X: .125, Z: 1}
		want := 65.25
		if kind == Pocket {
			d.Z = -1
			want = 62.75
		}
		r, e := ExtrudeProfiles(context.Background(), s, 1, profiles, d, kind, BatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := r.Solid.MassProperties()
		if e != nil || math.Abs(m.Volume-want) > 1e-10 {
			t.Fatal(e, m)
		}
		if r.Tool.Topology().Shells != 2 || r.Solid.Topology().Shells != 1 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("topology")
		}
		seen := map[int]bool{}
		for _, src := range r.History.Faces {
			if src.Operand == poly.OperandB {
				seen[r.ToolFaces[src.Face]] = true
			}
		}
		if len(seen) != 2 || len(r.Profiles) != 2 || len(r.ToolEdges) != len(r.Tool.Edges()) {
			t.Fatal("profile ancestry")
		}
		for i, p := range r.Profiles {
			for _, wall := range p.SketchWalls {
				if r.ToolFaces[wall] != i {
					t.Fatal("wrong wall")
				}
			}
		}
	}
}
func TestProfileConflictAtomic(t *testing.T) {
	s, _ := fixture(t)
	a := square(.5, .5, 1)
	for _, b := range [][]geom.Vec3{square(1, 1, 1), square(1.5, .5, 1), square(.75, .75, .25), a} {
		r, e := ExtrudeProfiles(context.Background(), s, 1, [][]geom.Vec3{a, b}, geom.Vec3{Z: 1}, Protrusion, BatchOptions{})
		if !errors.Is(e, ErrConflict) || len(r.Solid.Faces()) != 0 || len(r.History.Faces) != 0 {
			t.Fatal(e)
		}
	}
	bad := square(0, 0, 1)
	if r, e := ExtrudeProfiles(context.Background(), s, 1, [][]geom.Vec3{a, bad}, geom.Vec3{Z: 1}, Protrusion, BatchOptions{}); !errors.Is(e, ErrInvalid) || len(r.ToolFaces) != 0 {
		t.Fatal(e)
	}
	if _, e := ExtrudeProfiles(context.Background(), s, 1, [][]geom.Vec3{a, a}, geom.Vec3{Z: 1}, Protrusion, BatchOptions{MaxProfiles: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
func TestProfilePositiveSmallGap(t *testing.T) {
	s, _ := fixture(t)
	gap := math.Ldexp(1, -20)
	_, e := ExtrudeProfiles(context.Background(), s, 1, [][]geom.Vec3{square(.5, .5, .5), square(1+gap, .5, .5)}, geom.Vec3{Z: 1}, Protrusion, BatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
}
