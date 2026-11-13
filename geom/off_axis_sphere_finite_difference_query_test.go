package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestOffAxisSphereFiniteDifference(t *testing.T) {
	s, e := NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 4}, 0, 4)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p Vec3
		l BorePointLocation
		m BoreBoundaryMask
	}{
		{Vec3{X: 2, Z: 1}, BoreOutside, 0}, {Vec3{}, BoreInside, 0}, {Vec3{X: 2, Z: -1}, BoreInside, 0},
		{Vec3{X: 2}, BoreBoundary, BoreLowerCap}, {Vec3{X: 5}, BoreBoundary, BoreLowerCap | BoreSphere},
		{Vec3{X: 1, Z: 2}, BoreBoundary, BoreCylinder}, {Vec3{X: 3, Z: 4}, BoreBoundary, BoreUpperCap | BoreSphere},
		{Vec3{Z: 5}, BoreBoundary, BoreSphere}, {Vec3{X: 4, Z: 3}, BoreOutside, 0},
	} {
		l, m, e := s.Classify(tc.p)
		if e != nil || l != tc.l || m != tc.m {
			t.Fatalf("%+v: %v %v %v", tc.p, l, m, e)
		}
	}
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	eq := func(r *ExactAlgebraicRoot, w int64) {
		t.Helper()
		q, _, e := NewExactRationalRoot(ctx, big.NewRat(w, 1), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v, e := r.Compare(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil || !v.Proven || v.Order != 0 {
			t.Fatal(w, v, e)
		}
	}
	r, e := s.IntersectLineBoundary(ctx, Vec3{}, Vec3{X: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 1 || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
	eq(r.Hits[0].Parameter, -5)
	eq(r.Intervals[0].Start, 1)
	eq(r.Intervals[0].End, 5)
	h, e := s.IntersectLine(ctx, Vec3{Z: 2}, Vec3{X: 1}, -8, 8, o)
	if e != nil || len(h) != 2 || h[0].Boundary != BoreSphere || h[1].Boundary != BoreCylinder {
		t.Fatal(h, e)
	}
	eq(h[1].Parameter, 1)
	r, e = s.IntersectLineBoundary(ctx, Vec3{X: 1}, Vec3{Z: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 1 || len(r.Hits) != 2 {
		t.Fatal(r, e)
	}
	eq(r.Intervals[0].Start, 0)
	eq(r.Intervals[0].End, 4)
	// Independent finite contained pocket: both floors, no infinite bore.
	b, e := NewOffAxisSphereFiniteCylinderDifference(5, 1, Vec2{X: 2}, -1, 1)
	if e != nil {
		t.Fatal(e)
	}
	h, e = b.IntersectLine(ctx, Vec3{X: 2}, Vec3{Z: 1}, -8, 8, o)
	if e != nil || len(h) != 4 {
		t.Fatal(h, e)
	}
	eq(h[1].Parameter, -1)
	eq(h[2].Parameter, 1)
	if h[1].Boundary != BoreLowerCap || h[2].Boundary != BoreUpperCap {
		t.Fatal(h)
	}
}

func TestOffAxisSphereFiniteDifferenceRegularization(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	// External radial tangency leaves the sphere; no inward cylinder face exists.
	s, e := NewOffAxisSphereFiniteCylinderDifference(5, 1, Vec2{X: 6}, -1, 1)
	if e != nil {
		t.Fatal(e)
	}
	l, m, e := s.Classify(Vec3{X: 5})
	if e != nil || l != BoreBoundary || m != BoreSphere {
		t.Fatal(l, m, e)
	}
	h, e := s.IntersectLine(ctx, Vec3{}, Vec3{X: 1}, -8, 8, o)
	if e != nil || len(h) != 2 || h[1].Boundary != BoreSphere {
		t.Fatal(h, e)
	}
	// A covering tool leaves no isolated tangent remnants.
	for _, tc := range []struct {
		r float64
		c Vec2
	}{{5, Vec2{}}, {6, Vec2{X: 1}}} {
		s, e := NewOffAxisSphereFiniteCylinderDifference(5, tc.r, tc.c, -5, 5)
		if e != nil {
			t.Fatal(e)
		}
		l, m, e := s.Classify(Vec3{X: -5})
		if e != nil || l != BoreOutside || m != 0 {
			t.Fatal(l, m, e)
		}
		r, e := s.IntersectLineBoundary(ctx, Vec3{}, Vec3{X: 1}, -8, 8, o)
		if e != nil || len(r.Hits)+len(r.Intervals) != 0 {
			t.Fatal(r, e)
		}
	}
	// Pole-only cap contact also leaves no false cap boundary.
	s, e = NewOffAxisSphereFiniteCylinderDifference(5, 2, Vec2{}, 5, 6)
	if e != nil {
		t.Fatal(e)
	}
	_, m, e = s.Classify(Vec3{Z: 5})
	if e != nil || m != BoreSphere {
		t.Fatal(m, e)
	}
	for _, tc := range []struct {
		r      float64
		c      Vec2
		lo, hi float64
	}{{1, Vec2{X: 4}, -1, 1}, {3, Vec2{}, -4, 4}, {2, Vec2{}, -5, 0}} {
		if _, e := NewOffAxisSphereFiniteCylinderDifference(5, tc.r, tc.c, tc.lo, tc.hi); e == nil {
			t.Fatal("unproved tangency accepted", tc)
		}
	}
	s, _ = NewOffAxisSphereFiniteCylinderDifference(5, 1, Vec2{X: 2}, -1, 1)
	r, e := s.IntersectLineBoundary(ctx, Vec3{}, Vec3{X: 1}, -8, 8, ToroidLineOptions{ParameterTolerance: 1e-8, Surface: SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || len(r.Hits)+len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := s.IntersectLineBoundary(canceled, Vec3{}, Vec3{X: 1}, -8, 8, o); e != context.Canceled {
		t.Fatal(e)
	}
}
