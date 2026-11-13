package collision

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"sync"
	"testing"
)

func expectLocation(t *testing.T, l *PointLocator, p geom.Vec3, want Location) LocationResult {
	t.Helper()
	r, e := l.Locate(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	if r.Location != want {
		t.Fatalf("point %+v got %v want %v", p, r.Location, want)
	}
	for i, h := range r.Hits {
		if i > 0 && r.Hits[i-1].Parameter.Cmp(h.Parameter) >= 0 {
			t.Fatal("unsorted hits")
		}
		tr := l.triangles[h.Triangle]
		if h.Face != tr.face {
			t.Fatal("ancestry")
		}
		q := point(h.ExactPoint)
		if dot(tr.n, q).Cmp(tr.d) != 0 {
			t.Fatal("hit off exact plane")
		}
		if inside, on := triangleMembership(q, tr); !inside || on {
			t.Fatal("nontransverse hit")
		}
		if !same(q, plus(exact(p), scale(exact(r.RayDirection), h.Parameter))) {
			t.Fatal("ray parameter mismatch")
		}
	}
	return r
}
func TestPointLocationCubeAndRayDegeneracy(t *testing.T) {
	s := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	l, e := NewPointLocator(context.Background(), s, LocationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r := expectLocation(t, l, geom.Vec3{X: .5, Y: .5, Z: .5}, Inside)
	if r.RayAttempts < 2 || r.Winding != 1 {
		t.Fatal("vertex-hit direction was not retried", r)
	}
	for _, p := range []geom.Vec3{{}, {X: 1, Y: .5, Z: .5}, {X: 1, Y: 1, Z: .5}, {X: 1, Y: 1, Z: 1}} {
		r := expectLocation(t, l, p, OnBoundary)
		if r.Face < 0 || r.Triangle < 0 || len(r.Hits) != 0 {
			t.Fatal(r)
		}
		again := expectLocation(t, l, p, OnBoundary)
		if again.Face != r.Face || again.Triangle != r.Triangle {
			t.Fatal("unstable boundary ancestry")
		}
	}
	expectLocation(t, l, geom.Vec3{X: -1, Y: -1, Z: -1}, Outside)
	expectLocation(t, l, geom.Vec3{X: math.Nextafter(1, 0), Y: .5, Z: .5}, Inside)
	expectLocation(t, l, geom.Vec3{X: math.Nextafter(1, 2), Y: .5, Z: .5}, Outside)
	limited, e := NewPointLocator(context.Background(), s, LocationOptions{MaxRays: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = limited.Locate(context.Background(), geom.Vec3{X: .5, Y: .5, Z: .5}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
func locationJoin(t *testing.T, ss ...brep.Solid) brep.Solid {
	t.Helper()
	var vs []geom.Vec3
	var es []brep.Edge
	var fs []brep.Face
	for _, s := range ss {
		vo, eo := len(vs), len(es)
		vs = append(vs, s.Vertices()...)
		for _, e := range s.Edges() {
			es = append(es, brep.Edge{Start: e.Start + vo, End: e.End + vo})
		}
		for _, f := range s.Faces() {
			for i := range f.Boundary {
				f.Boundary[i].Edge += eo
			}
			for _, h := range f.Holes {
				for i := range h {
					h[i].Edge += eo
				}
			}
			fs = append(fs, f)
		}
	}
	s, e := brep.NewSolid(vs, es, fs)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestPointLocationCavityAndIsland(t *testing.T) {
	outer := dbox(t, geom.Vec3{}, geom.Vec3{X: 6, Y: 6, Z: 6})
	hole := dbox(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 5, Y: 5, Z: 5})
	island := dbox(t, geom.Vec3{X: 2, Y: 2, Z: 2}, geom.Vec3{X: 4, Y: 4, Z: 4})
	l, e := NewPointLocator(context.Background(), locationJoin(t, outer, hole.Reversed(), island), LocationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	expectLocation(t, l, geom.Vec3{X: .5, Y: .5, Z: .5}, Inside)
	expectLocation(t, l, geom.Vec3{X: 1.5, Y: 1.5, Z: 1.5}, Outside)
	expectLocation(t, l, geom.Vec3{X: 3, Y: 3, Z: 3}, Inside)
	expectLocation(t, l, geom.Vec3{X: 1, Y: 3, Z: 3}, OnBoundary)
	expectLocation(t, l, geom.Vec3{X: 7, Y: 3, Z: 3}, Outside)
	for _, invalid := range []brep.Solid{outer.Reversed(), locationJoin(t, outer, hole), locationJoin(t, outer, island.Reversed(), hole.Reversed()), locationJoin(t, outer, dbox(t, geom.Vec3{X: 5, Y: 5, Z: 5}, geom.Vec3{X: 7, Y: 7, Z: 7}))} {
		if _, e := NewPointLocator(context.Background(), invalid, LocationOptions{}); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid shell arrangement", e)
		}
	}
}
func TestPointLocationConcavityAndHoles(t *testing.T) {
	s, e := brep.Prism([]geom.Vec3{{}, {X: 4}, {X: 4, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 4}, {Y: 4}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	l, e := NewPointLocator(context.Background(), s, LocationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	expectLocation(t, l, geom.Vec3{X: 2, Y: 2, Z: 1}, Outside)
	expectLocation(t, l, geom.Vec3{X: .5, Y: 2, Z: 1}, Inside)
	expectLocation(t, l, geom.Vec3{X: 1, Y: 2, Z: 1}, OnBoundary)
	s, e = brep.PrismWithHoles([]geom.Vec3{{}, {X: 6}, {X: 6, Y: 6}, {Y: 6}}, [][]geom.Vec3{{{X: 2, Y: 2}, {X: 2, Y: 4}, {X: 4, Y: 4}, {X: 4, Y: 2}}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	l, e = NewPointLocator(context.Background(), s, LocationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	expectLocation(t, l, geom.Vec3{X: 3, Y: 3, Z: 1}, Outside)
	expectLocation(t, l, geom.Vec3{X: 3, Y: 3}, Outside)
	expectLocation(t, l, geom.Vec3{X: 1, Y: 1, Z: 1}, Inside)
	expectLocation(t, l, geom.Vec3{X: 2, Y: 3, Z: 1}, OnBoundary)
}
func TestPointLocationBudgetsAndCancellation(t *testing.T) {
	s := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	for _, o := range []LocationOptions{{MaxWork: 1}, {MaxTriangles: 1}, {MaxCollisionPairs: 1}} {
		if _, e := NewPointLocator(context.Background(), s, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	for _, o := range []LocationOptions{{MaxWork: -1}, {MaxRays: 1025}} {
		if _, e := NewPointLocator(context.Background(), s, o); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := NewPointLocator(ctx, s, LocationOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	l, e := NewPointLocator(context.Background(), s, LocationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = l.Locate(ctx, geom.Vec3{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = l.Locate(context.Background(), geom.Vec3{X: math.Inf(1)}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	r, e := LocatePoint(context.Background(), brep.Solid{}, geom.Vec3{}, LocationOptions{})
	if e != nil || r.Location != Outside || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
}
func TestPointLocatorConcurrentAndOwnedHits(t *testing.T) {
	s := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	l, e := NewPointLocator(context.Background(), s, LocationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := l.Locate(context.Background(), geom.Vec3{X: .5, Y: .5, Z: .5})
			if e != nil || r.Location != Inside {
				t.Error(e)
			}
			if len(r.Hits) > 0 {
				r.Hits[0].Parameter.SetInt64(99)
				r.Hits[0].ExactPoint[0].SetInt64(99)
			}
		}()
	}
	wg.Wait()
	expectLocation(t, l, geom.Vec3{X: .5, Y: .5, Z: .5}, Inside)
}

func FuzzPointLocationShearedBox(f *testing.F) {
	f.Add(int16(4), int16(4), int16(4))
	f.Add(int16(0), int16(4), int16(4))
	f.Add(int16(-1), int16(8), int16(4))
	base, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		f.Fatal(e)
	}
	xf := geom.Affine{{1, .25, 0, 3}, {0, 1, .25, -2}, {.25, 0, 1, 7}}
	s, e := base.Transformed(xf)
	if e != nil {
		f.Fatal(e)
	}
	l, e := NewPointLocator(context.Background(), s, LocationOptions{})
	if e != nil {
		f.Fatal(e)
	}
	f.Fuzz(func(t *testing.T, x, y, z int16) {
		p := geom.Vec3{X: float64(x%24) / 8, Y: float64(y%24) / 8, Z: float64(z%24) / 8}
		want := Inside
		if p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 || p.Z < 0 || p.Z > 1 {
			want = Outside
		} else if p.X == 0 || p.X == 1 || p.Y == 0 || p.Y == 1 || p.Z == 0 || p.Z == 1 {
			want = OnBoundary
		}
		expectLocation(t, l, xf.Apply(p), want)
	})
}

func TestPointLocationSubnormalDisconnectedAndEmpty(t *testing.T) {
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	b := dbox(t, geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 1, Z: 1})
	l, e := NewPointLocator(context.Background(), locationJoin(t, a, b), LocationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	expectLocation(t, l, geom.Vec3{X: math.SmallestNonzeroFloat64, Y: .5, Z: .5}, Inside)
	expectLocation(t, l, geom.Vec3{X: -math.SmallestNonzeroFloat64, Y: .5, Z: .5}, Outside)
	expectLocation(t, l, geom.Vec3{X: 1.5, Y: .5, Z: .5}, Outside)
	expectLocation(t, l, geom.Vec3{X: 2.5, Y: .5, Z: .5}, Inside)
	expectLocation(t, l, geom.Vec3{X: math.MaxFloat64, Y: .5, Z: .5}, Outside)
	expectLocation(t, &PointLocator{}, geom.Vec3{}, Outside)
	touching := dbox(t, geom.Vec3{X: 1}, geom.Vec3{X: 2, Y: 1, Z: 1})
	if _, e = NewPointLocator(context.Background(), locationJoin(t, a, touching), LocationOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
