package prism

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func rectangle(x, y, a, b float64) []geom.Vec2 {
	return []geom.Vec2{{X: x, Y: y}, {X: a, Y: y}, {X: a, Y: b}, {X: x, Y: b}}
}

func TestConcaveProfileBoolean(t *testing.T) {
	a := Profile{Outer: []geom.Vec2{{}, {X: 4}, {X: 4, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 4}, {Y: 4}}}
	b := Profile{Outer: []geom.Vec2{{X: 1, Y: 1}, {X: 3.5, Y: 1}, {X: 1, Y: 3.5}}}
	for _, tc := range []struct {
		o Operation
		v float64
	}{{Intersection, 6}, {Union, 24.25}, {Difference, 18}} {
		ss, e := Boolean(context.Background(), a, b, 2, tc.o, Options{})
		if e != nil || len(ss) != 1 {
			t.Fatal(len(ss), e)
		}
		p, e := ss[0].MassProperties()
		if e != nil || math.Abs(p.Volume-tc.v) > 1e-10 {
			t.Fatalf("%+v %v", p, e)
		}
	}
}
func TestHoleDisjointAndContact(t *testing.T) {
	ctx := context.Background()
	a := Profile{Outer: rectangle(0, 0, 4, 4)}
	for _, tc := range []struct {
		o     Operation
		b     Profile
		count int
		vol   float64
	}{
		{Difference, Profile{Outer: rectangle(1, 1, 3, 3)}, 1, 12},
		{Union, Profile{Outer: rectangle(4, 0, 5, 4)}, 1, 20},
		{Union, Profile{Outer: rectangle(5, 0, 6, 4)}, 2, 20},
		{Intersection, Profile{Outer: rectangle(4, 0, 5, 4)}, 0, 0},
		{Difference, Profile{Outer: rectangle(1, -1, 3, 5)}, 2, 8},
	} {
		ss, e := Boolean(ctx, a, tc.b, 1, tc.o, Options{})
		if e != nil || len(ss) != tc.count {
			t.Fatalf("%+v: %d %v", tc, len(ss), e)
		}
		sum := 0.
		for _, s := range ss {
			p, e := s.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			sum += p.Volume
		}
		if math.Abs(sum-tc.vol) > 1e-10 {
			t.Fatalf("%+v volume %g", tc, sum)
		}
	}
	if _, e := Boolean(ctx, a, Profile{Outer: rectangle(4, 4, 6, 6)}, 1, Union, Options{}); !errors.Is(e, ErrNonmanifold) {
		t.Fatal(e)
	}
	if _, e := Boolean(ctx, a, a, 1, Union, Options{MaxWork: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}

func FuzzRectangleVolumeIdentities(f *testing.F) {
	f.Add(int8(2), int8(3))
	f.Add(int8(4), int8(5))
	f.Fuzz(func(t *testing.T, x, y int8) {
		if x < -6 || x > 6 || y < -6 || y > 6 {
			return
		}
		a := Profile{Outer: rectangle(0, 0, 4, 5)}
		b := Profile{Outer: rectangle(float64(x), float64(y), float64(x)+3, float64(y)+4)}
		area := max(0., min(4., float64(x)+3)-max(0., float64(x))) * max(0., min(5., float64(y)+4)-max(0., float64(y)))
		for _, tt := range []struct {
			op Operation
			v  float64
		}{{Union, 32 - area}, {Intersection, area}, {Difference, 20 - area}} {
			out, e := Boolean(context.Background(), a, b, 1, tt.op, Options{})
			if errors.Is(e, ErrNonmanifold) && tt.op == Union && area == 0 {
				continue
			}
			if e != nil {
				t.Fatal(x, y, tt.op, e)
			}
			got := 0.
			for _, s := range out {
				p, e := s.MassProperties()
				if e != nil {
					t.Fatal(e)
				}
				got += p.Volume
			}
			if math.Abs(got-tt.v) > 1e-10 {
				t.Fatal(x, y, tt.op, got, tt.v)
			}
		}
	})
}

func TestBooleanWithHoledOperand(t *testing.T) {
	a := Profile{Outer: rectangle(0, 0, 6, 6), Holes: [][]geom.Vec2{rectangle(1, 1, 5, 5)}}
	b := Profile{Outer: rectangle(2, 0, 4, 6)}
	for _, tc := range []struct {
		op  Operation
		n   int
		vol float64
	}{{Union, 1, 28}, {Intersection, 2, 4}, {Difference, 2, 16}} {
		ss, e := Boolean(context.Background(), a, b, 1, tc.op, Options{})
		if e != nil || len(ss) != tc.n {
			t.Fatalf("op %d: %d %v", tc.op, len(ss), e)
		}
		v := 0.
		for _, s := range ss {
			p, e := s.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			v += p.Volume
		}
		if math.Abs(v-tc.vol) > 1e-10 {
			t.Fatalf("op %d volume %g", tc.op, v)
		}
	}
}

func TestPromotedPrismSolids(t *testing.T) {
	a, e := brep.Prism([]geom.Vec3{{Z: 2}, {X: 4, Z: 2}, {X: 4, Y: 2, Z: 2}, {X: 2, Y: 2, Z: 2}, {X: 2, Y: 4, Z: 2}, {Y: 4, Z: 2}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e := brep.Prism([]geom.Vec3{{X: 1, Y: 1, Z: 2}, {X: 3.5, Y: 1, Z: 2}, {X: 1, Y: 3.5, Z: 2}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	s, e := Solids(context.Background(), a, b, Difference, Options{})
	if e != nil || len(s) != 1 {
		t.Fatal(len(s), e)
	}
	p, e := s[0].MassProperties()
	if e != nil || math.Abs(p.Volume-18) > 1e-10 || math.Abs(p.Centroid.Z-3) > 1e-10 {
		t.Fatal(p, e)
	}
	if _, e = Solids(context.Background(), a.Reversed(), b, Union, Options{}); e == nil {
		t.Fatal("accepted inward prism")
	}
}
