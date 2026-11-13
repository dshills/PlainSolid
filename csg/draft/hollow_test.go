package draft

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"testing"
)

func TestHollowBoxAndTray(t *testing.T) {
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	for _, open := range []int{-1, 1} {
		thickness := []float64{.5, .5, .5, .5, .5, .5}
		want := 37.
		shells, euler := 2, 4
		if open == 1 {
			thickness[1] = 0
			want = 32.5
			shells, euler = 1, 2
		}
		r, e := Hollow(context.Background(), s, thickness, HollowOptions{OpenFace: open})
		if e != nil {
			t.Fatal(e)
		}
		m, e := r.Solid.MassProperties()
		if e != nil || math.Abs(m.Volume-want) > 1e-10 {
			t.Fatal(e, m)
		}
		top := r.Solid.Topology()
		if top.Shells != shells || top.EulerCharacteristic != euler {
			t.Fatal(top)
		}
		for _, src := range r.History.Faces {
			if src.Operand == poly.OperandB && !src.Reversed {
				t.Fatal("cavity source not reversed")
			}
		}
		if r.Interior.ExactVolume.Sign() <= 0 {
			t.Fatal("missing exact interior")
		}
	}
}
func TestIndependentThickness(t *testing.T) {
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	if e != nil {
		t.Fatal(e)
	}
	r, e := Hollow(context.Background(), s, []float64{.5, 1, .25, .5, .75, 1}, HollowOptions{OpenFace: -1, CoordinateThickness: true})
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.MassProperties()
	if e != nil || math.Abs(m.Volume-(120-2.5*4*4.5)) > 1e-10 {
		t.Fatal(e, m)
	}
	checkVolume(t, r.Interior, 45, 1)
}
func TestHollowFailures(t *testing.T) {
	s := cube(t)
	for _, th := range [][]float64{{.5, .5, .5, .5, .5, .5}, {0, .1, .1, .1, .1, .1}, {-1, 1, 1, 1, 1, 1}, {1}} {
		if _, e := Hollow(context.Background(), s, th, HollowOptions{OpenFace: -1}); e == nil {
			t.Fatal("accepted invalid cavity", th)
		}
	}
	if _, e := Hollow(context.Background(), s, []float64{.25, .25, .25, .25, .25, .25}, HollowOptions{OpenFace: -1, Reconstruction: Options{MaxWork: 1}}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, e := Hollow(context.Background(), s, []float64{.25, .25, .25, .25, .25, .25}, HollowOptions{OpenFace: 1}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
