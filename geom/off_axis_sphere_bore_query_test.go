package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestOffAxisSphereBoreExactPoints(t *testing.T) {
	s, e := NewOffAxisSphereBore(5, 1, Vec2{X: 2}, 0)
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []struct {
		p Vec3
		l BorePointLocation
		m BoreBoundaryMask
	}{{Vec3{}, BoreInside, 0}, {Vec3{X: 2, Y: 0, Z: 1}, BoreOutside, 0}, {Vec3{X: 2, Z: 0}, BoreBoundary, BoreFloor}, {Vec3{X: 3, Z: 0}, BoreBoundary, BoreCylinder | BoreFloor}, {Vec3{X: 3, Z: 4}, BoreBoundary, BoreCylinder | BoreSphere}, {Vec3{X: 2, Z: -1}, BoreInside, 0}, {Vec3{Z: 5}, BoreBoundary, BoreSphere}, {Vec3{X: 2, Z: 5}, BoreOutside, 0}} {
		l, m, e := s.Classify(x.p)
		if e != nil || l != x.l || m != x.m {
			t.Fatal(x, l, m, e)
		}
	}
}
func TestOffAxisSphereBoreExactLines(t *testing.T) {
	s, _ := NewOffAxisSphereBore(5, 1, Vec2{X: 2}, 0)
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	ctx := context.Background()
	// Cylinder-center axial line sees the south sphere and the floor only;
	// its north sphere root is hidden inside the removed material.
	hits, e := s.IntersectLine(ctx, Vec3{X: 2, Z: -6}, Vec3{Z: 1}, 0, 12, o)
	if e != nil || len(hits) != 2 {
		t.Fatal(e, len(hits))
	}
	if hits[0].Boundary != BoreSphere || hits[1].Boundary != BoreFloor || hits[1].Bounds.Lower.Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatal("hidden surface filtering", hits)
	}
	// Horizontal line at z=4 crosses the sphere at±3; cylinder side x=3 is
	// simultaneously a spherical rim, deduplicated by exact root identity.
	hits, e = s.IntersectLine(ctx, Vec3{X: -6, Z: 4}, Vec3{X: 1}, 0, 12, o)
	if e != nil || len(hits) != 3 {
		t.Fatal(e, len(hits))
	}
	if hits[2].Boundary != BoreSphere|BoreCylinder {
		t.Fatal("rim merge", hits[2])
	}
	hits, e = s.IntersectLine(ctx, Vec3{X: -6, Y: 5, Z: -1}, Vec3{X: 1}, 0, 12, o)
	if e != nil || len(hits) != 0 {
		t.Fatal("miss", e)
	}
	hits, e = s.IntersectLine(ctx, Vec3{X: -6, Y: 5, Z: 0.5}, Vec3{X: 1}, 0, 12, o)
	if e != nil || len(hits) != 0 {
		t.Fatal("miss", e)
	}
	// Sphere tangent away from the cylinder and floor plane.
	hits, e = s.IntersectLine(ctx, Vec3{X: -6, Y: 3, Z: -4}, Vec3{X: 1}, 0, 12, o)
	if e != nil || len(hits) != 1 || hits[0].Multiplicity != 2 {
		t.Fatal("tangent", e, hits)
	}
	// Exact cylinder/floor seam on an oblique line, with a finite endpoint hit.
	hits, e = s.IntersectLine(ctx, Vec3{X: 3}, Vec3{X: 1, Z: 1}, 0, 1, o)
	if e != nil || len(hits) != 1 || hits[0].Boundary != BoreCylinder|BoreFloor {
		t.Fatal("floor rim", e, hits)
	}
}
func TestOffAxisSphereBoreQueryLimits(t *testing.T) {
	s0, _ := NewOffAxisSphereBore(5, 1, Vec2{X: 2}, 0)
	for _, z := range []float64{math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64} {
		loc, _, e := s0.Classify(Vec3{X: 2, Z: z})
		want := BoreInside
		if z > 0 {
			want = BoreOutside
		}
		if e != nil || loc != want {
			t.Fatal("subnormal boundary", loc, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s0.IntersectLine(ctx, Vec3{Z: -6}, Vec3{Z: 1}, 0, 12, ToroidLineOptions{ParameterTolerance: 1e-9}); e == nil {
		t.Fatal("cancel")
	}

	for _, p := range []struct {
		r float64
		c Vec2
		f float64
	}{{1, Vec2{X: 4}, 0}, {1, Vec2{X: 2}, 4}, {0, Vec2{}, 0}} {
		if _, e := NewOffAxisSphereBore(5, p.r, p.c, p.f); e == nil {
			t.Fatal("invalid")
		}
	}
	s, _ := NewOffAxisSphereBore(5, 1, Vec2{X: 2}, 0)
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	if _, e := s.IntersectLine(context.Background(), Vec3{X: -6}, Vec3{X: 1}, 0, 12, o); e == nil {
		t.Fatal("coincident floor")
	}
	if _, e := s.IntersectLine(context.Background(), Vec3{X: 3, Z: -1}, Vec3{Z: 1}, 0, 7, o); e == nil {
		t.Fatal("coincident cylinder")
	}
	o.Surface.MaxOperations = 1
	if hits, e := s.IntersectLine(context.Background(), Vec3{X: 2, Z: -6}, Vec3{Z: 1}, 0, 12, o); e == nil || len(hits) != 0 {
		t.Fatal("partial")
	}
}
