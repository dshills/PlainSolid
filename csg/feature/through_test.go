package feature

import (
	"context"
	"errors"
	"math"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"testing"
)

func TestThroughPocket(t *testing.T) {
	s, p := fixture(t)
	for _, dx := range []float64{0, .5} {
		r, e := ThroughPocket(context.Background(), s, 1, 0, p, geom.Vec3{X: dx, Z: -4}, Options{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := r.Solid.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(m.Volume-48) > 1e-10 {
			t.Fatal(m)
		}
		want := 88 + 16 + 4*math.Hypot(4, dx)
		if math.Abs(m.SurfaceArea-want) > 1e-10 {
			t.Fatal(m.SurfaceArea, want)
		}
		if m.Centroid.Sub(geom.Vec3{X: 2 - dx/6, Y: 2, Z: 2}).Length() > 1e-10 {
			t.Fatal(m.Centroid)
		}
		top := r.Solid.Topology()
		if top.Shells != 1 || top.EulerCharacteristic != 0 {
			t.Fatal(top)
		}
		sources := map[int]bool{}
		for _, src := range r.History.Faces {
			if src.Operand == poly.OperandB {
				sources[src.Face] = true
				if !src.Reversed {
					t.Fatal("wall orientation")
				}
			}
		}
		if len(sources) != 4 || sources[r.Mouth] || sources[r.Terminal] {
			t.Fatal(sources)
		}
	}
}
func TestThroughPocketReject(t *testing.T) {
	s, p := fixture(t)
	for _, tc := range []struct {
		exit int
		d    geom.Vec3
		want error
	}{{1, geom.Vec3{Z: -4}, ErrInvalid}, {2, geom.Vec3{Z: -4}, ErrInvalid}, {0, geom.Vec3{Z: -3}, ErrThrough}, {0, geom.Vec3{Z: -5}, ErrThrough}, {0, geom.Vec3{X: 1, Z: -4}, ErrThrough}} {
		_, e := ThroughPocket(context.Background(), s, 1, tc.exit, p, tc.d, Options{})
		if !errors.Is(e, tc.want) {
			t.Fatal(tc, e)
		}
	}
}
