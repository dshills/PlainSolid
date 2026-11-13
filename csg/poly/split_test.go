package poly

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/csg"
	"plainsolid/geom"
	"testing"
)

func splitPlane(t *testing.T, o, n geom.Vec3) geom.Plane {
	t.Helper()
	p, e := geom.NewPlane(o, n)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSplitNonconvexOblique(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 4}, {Y: 4}}
	s, e := brep.Prism(outer, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	p := splitPlane(t, geom.Vec3{Z: 1.4}, geom.Vec3{X: .13, Y: .1, Z: 1})
	a, b, e := SplitPlane(context.Background(), s, p, Options{})
	if e != nil {
		t.Fatal(e)
	}
	want := 16*(1.4-.13*2-.1*2) - 2*(1.4-.13*3.5-.1*3)
	if math.Abs(volume(t, a)-want) > 1e-9 || math.Abs(volume(t, a)+volume(t, b)-42) > 1e-9 {
		t.Fatal(volume(t, a), volume(t, b), want)
	}
}
func TestSplitCavityOblique(t *testing.T) {
	outer, inner := box(t, -2, -2, -2, 2, 2, 2), box(t, -1, -1, -1, 1, 1, 1)
	s := run(t, outer, inner, Difference, 56)
	p := splitPlane(t, geom.Vec3{}, geom.Vec3{X: .13, Y: .1, Z: 1})
	a, b, e := SplitPlane(context.Background(), s, p, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(volume(t, a)-28) > 1e-9 || math.Abs(volume(t, b)-28) > 1e-9 {
		t.Fatal(volume(t, a), volume(t, b))
	}
}
func TestSplitDisjointAndTangency(t *testing.T) {
	s := box(t, 0, 0, 0, 2, 2, 2)
	for _, c := range []struct {
		o, n     geom.Vec3
		negative float64
	}{{geom.Vec3{Z: 3}, geom.Vec3{Z: 1}, 8}, {geom.Vec3{Z: 2}, geom.Vec3{Z: 1}, 8}, {geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1}, 0}, {geom.Vec3{X: 2}, geom.Vec3{X: 1}, 8}, {geom.Vec3{X: 1}, geom.Vec3{X: 1}, 4}} {
		a, b, e := SplitPlane(context.Background(), s, splitPlane(t, c.o, c.n), Options{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(volume(t, a)-c.negative) > 1e-10 || math.Abs(volume(t, a)+volume(t, b)-8) > 1e-10 {
			t.Fatal(volume(t, a), volume(t, b))
		}
	}
	disjoint := concatenate(t, box(t, -3, -1, -1, -2, 1, 1), box(t, 2, -1, -1, 3, 1, 1))
	a, b, e := SplitPlane(context.Background(), disjoint, splitPlane(t, geom.Vec3{}, geom.Vec3{X: 1}), Options{})
	if e != nil {
		t.Fatal(e)
	}
	if volume(t, a) != 4 || volume(t, b) != 4 {
		t.Fatal(volume(t, a), volume(t, b))
	}
}
func TestSplitErrors(t *testing.T) {
	s := box(t, 0, 0, 0, 2, 2, 2)
	p := splitPlane(t, geom.Vec3{X: 1}, geom.Vec3{X: 1})
	for _, o := range []Options{{MaxWork: 1}, {MaxBits: 1}, {MaxFragments: 1}, {MaxVertices: 1}} {
		a, b, e := SplitPlane(context.Background(), s, p, o)
		if !errors.Is(e, ErrLimit) || len(a.Faces()) != 0 || len(b.Faces()) != 0 {
			t.Fatal(e)
		}
	}
	if _, _, e := SplitPlane(context.Background(), s, geom.Plane{}, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := SplitPlane(ctx, s, p, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	a, b, e := SplitPlane(context.Background(), brep.Solid{}, p, Options{})
	if e != nil || len(a.Faces()) != 0 || len(b.Faces()) != 0 {
		t.Fatal(e)
	}
}

func TestSplitThreeDimensionalOracle(t *testing.T) {
	s, parts := cells(t, [][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, geom.Affine{{1, .25, 0, 0}, {0, 1, .25, 0}, {.25, 0, 1, 0}})
	p := splitPlane(t, geom.Vec3{Z: .7}, geom.Vec3{X: .13, Y: .1, Z: 1})
	want := 0.
	for _, part := range parts {
		r, e := csg.Split(part, p, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		want += volume(t, r.Negative)
	}
	a, b, e := SplitPlane(context.Background(), s, p, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(volume(t, a)-want) > 1e-9 || math.Abs(volume(t, a)+volume(t, b)-volume(t, s)) > 1e-9 {
		t.Fatal(volume(t, a), volume(t, b), want)
	}
}
