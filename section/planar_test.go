package section

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"plainsolid/brep"
	"plainsolid/csg"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

func TestHoledNonconvexSection(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 1, Y: 2}}
	s, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewPlane(geom.Vec3{Z: 1.5}, geom.Vec3{X: .25, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := Planar(context.Background(), s, p, 0)
	if e != nil || len(r) != 2 {
		t.Fatal(r, e)
	}
	a := 0.
	for _, loop := range r {
		a += loop.SignedArea
		if loop.SignedArea > 0 && len(loop.Points) != 6 || loop.SignedArea < 0 && len(loop.Points) != 4 {
			t.Fatal(loop)
		}
		for _, x := range loop.Points {
			if math.Abs(p.SignedDistance(x)) > 1e-12 {
				t.Fatal(x)
			}
		}
	}
	if math.Abs(a-13*math.Hypot(1, .25)) > 1e-10 {
		t.Fatal(a)
	}
	p, e = geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Planar(context.Background(), s, p, 0); !errors.Is(e, ErrContact) {
		t.Fatal(e)
	}
	if _, e = Planar(context.Background(), s, p, 1); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}

func TestSectionCavityOrientation(t *testing.T) {
	a, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	b, e := brep.Box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	s, e := csg.Difference(a, b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	for _, sign := range []float64{1, -1} {
		p, e := geom.NewPlane(geom.Vec3{Z: 2}, geom.Vec3{Z: sign})
		if e != nil {
			t.Fatal(e)
		}
		loops, e := Planar(context.Background(), s, p, 0)
		if e != nil || len(loops) != 2 {
			t.Fatal(loops, e)
		}
		vals := []float64{loops[0].SignedArea, loops[1].SignedArea}
		slices.Sort(vals)
		if vals[0] != -4 || vals[1] != 16 {
			t.Fatal(vals)
		}
	}
}

func TestNonconvexSplit(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 1, Y: 2}}
	s, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewPlane(geom.Vec3{Z: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := Split(context.Background(), s, p, poly.Options{})
	if e != nil || len(r.Loops) != 2 {
		t.Fatal(e, len(r.Loops))
	}
	for _, tc := range []struct {
		v    brep.Solid
		want float64
	}{{r.Negative, 13}, {r.Positive, 26}} {
		m, e := tc.v.MassProperties()
		if e != nil || math.Abs(m.Volume-tc.want) > 1e-9 {
			t.Fatal(m, e, tc.want)
		}
	}
}

func TestNonconvexObliqueSplit(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 4}, {Y: 4}}
	s, e := brep.Prism(outer, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewPlane(geom.Vec3{Z: 1.4}, geom.Vec3{X: .13, Y: .1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := Split(context.Background(), s, p, poly.Options{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := r.Negative.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	b, e := r.Positive.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	// Rectangle minus the upper-right notch. Integrate z=1.4-.13x-.1y.
	want := 16*(1.4-.13*2-.1*2) - 2*(1.4-.13*3.5-.1*3)
	if math.Abs(a.Volume-want) > 1e-9 || math.Abs(a.Volume+b.Volume-42) > 1e-9 {
		t.Fatal(a.Volume, b.Volume, want)
	}
}
