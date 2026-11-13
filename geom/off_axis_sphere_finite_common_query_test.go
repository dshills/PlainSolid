package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestOffAxisSphereFiniteCommonPartialCaps(t *testing.T) {
	s, e := NewOffAxisSphereFiniteCylinderCommon(5, 3, Vec2{X: 4}, 0, 4)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p Vec3
		l BorePointLocation
		m BoreBoundaryMask
	}{
		{Vec3{X: 2, Z: 1}, BoreInside, 0}, {Vec3{}, BoreOutside, 0}, {Vec3{X: 2}, BoreBoundary, BoreLowerCap},
		{Vec3{X: 1}, BoreBoundary, BoreLowerCap | BoreCylinder}, {Vec3{X: 5}, BoreBoundary, BoreLowerCap | BoreSphere},
		{Vec3{X: 3, Z: 4}, BoreBoundary, BoreUpperCap | BoreSphere}, {Vec3{X: 4, Z: 4}, BoreOutside, 0},
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
			t.Fatal("root", w, v, e)
		}
	}
	for _, tc := range []struct {
		p, d         Vec3
		lo, hi       float64
		a, b         int64
		mask, am, bm BoreBoundaryMask
	}{
		{Vec3{}, Vec3{X: 1}, -8, 8, 1, 5, BoreLowerCap, BoreLowerCap | BoreCylinder, BoreLowerCap | BoreSphere},
		{Vec3{Z: 4}, Vec3{X: 1}, -8, 8, 1, 3, BoreUpperCap, BoreUpperCap | BoreCylinder, BoreUpperCap | BoreSphere},
		{Vec3{X: 1}, Vec3{Z: 1}, -8, 8, 0, 4, BoreCylinder, BoreCylinder | BoreLowerCap, BoreCylinder | BoreUpperCap},
		{Vec3{}, Vec3{X: -1}, -8, 8, -5, -1, BoreLowerCap, BoreLowerCap | BoreSphere, BoreLowerCap | BoreCylinder},
		{Vec3{}, Vec3{X: 1}, 2, 3, 2, 3, BoreLowerCap, BoreLowerCap, BoreLowerCap},
	} {
		r, e := s.IntersectLineBoundary(ctx, tc.p, tc.d, tc.lo, tc.hi, o)
		if e != nil || len(r.Intervals) != 1 || len(r.Hits) != 0 {
			t.Fatal(r, e)
		}
		v := r.Intervals[0]
		eq(v.Start, tc.a)
		eq(v.End, tc.b)
		if v.Boundary != tc.mask || v.StartBoundary != tc.am || v.EndBoundary != tc.bm {
			t.Fatal(v)
		}
	}
	h, e := s.IntersectLine(ctx, Vec3{Z: 2}, Vec3{X: 1}, -8, 8, o)
	if e != nil || len(h) != 2 {
		t.Fatal(h, e)
	}
	eq(h[0].Parameter, 1)
	if h[0].Boundary != BoreCylinder || h[1].Boundary != BoreSphere {
		t.Fatal(h)
	}
	b := h[1].Bounds
	if new(big.Rat).Mul(b.Lower, b.Lower).Cmp(big.NewRat(21, 1)) > 0 || new(big.Rat).Mul(b.Upper, b.Upper).Cmp(big.NewRat(21, 1)) < 0 {
		t.Fatal("sqrt21")
	}
	if _, e := s.IntersectLine(ctx, Vec3{}, Vec3{X: 1}, -8, 8, o); e == nil {
		t.Fatal("continuous accepted")
	}
}

func TestOffAxisSphereFiniteCommonDegenerateAndLimits(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	for _, offset := range []float64{6, 7} {
		s, e := NewOffAxisSphereFiniteCylinderCommon(5, 1, Vec2{X: offset}, -1, 1)
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.IntersectLineBoundary(ctx, Vec3{}, Vec3{X: 1}, -8, 8, o)
		want := 0
		if offset == 6 {
			want = 1
		}
		if e != nil || len(r.Hits) != want || len(r.Intervals) != 0 {
			t.Fatal(r, e)
		}
		if want == 1 && r.Hits[0].Boundary != BoreSphere|BoreCylinder {
			t.Fatal(r)
		}
	}
	// A cap tangent to the sphere is an isolated closed-set point.
	s, _ := NewOffAxisSphereFiniteCylinderCommon(5, 10, Vec2{}, 5, 6)
	r, e := s.IntersectLineBoundary(ctx, Vec3{Z: 5}, Vec3{X: 1}, -8, 8, o)
	if e != nil || len(r.Hits) != 1 || len(r.Intervals) != 0 || r.Hits[0].Boundary != BoreSphere|BoreLowerCap {
		t.Fatal(r, e)
	}
	for _, bad := range []ToroidLineOptions{{ParameterTolerance: 1e-8, Surface: SurfacePatchOptions{MaxOperations: 1}}, {ParameterTolerance: 1e-8, MaxSubdivisions: -1}} {
		r, e := s.IntersectLineBoundary(ctx, Vec3{}, Vec3{Z: 1}, -8, 8, bad)
		if e == nil || len(r.Hits)+len(r.Intervals) != 0 {
			t.Fatal(r, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := s.IntersectLineBoundary(canceled, Vec3{}, Vec3{Z: 1}, -8, 8, o); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e := NewOffAxisSphereFiniteCylinderCommon(5, 1, Vec2{}, 1, 1); e == nil {
		t.Fatal("zero height")
	}
}
