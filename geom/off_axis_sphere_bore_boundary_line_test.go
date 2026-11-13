package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestOffAxisSphereBoreBoundaryIntervals(t *testing.T) {
	s, e := NewOffAxisSphereBore(5, 1, Vec2{X: 2}, 0)
	if e != nil {
		t.Fatal(e)
	}
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	check := func(r *ExactAlgebraicRoot, w int64) {
		t.Helper()
		q, _, e := NewExactRationalRoot(context.Background(), big.NewRat(w, 1), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		c, e := r.Compare(context.Background(), q, ExactCurvePlaneEventOptions{})
		if e != nil || !c.Proven || c.Order != 0 {
			t.Fatalf("root != %d: %+v %v", w, c, e)
		}
	}
	for _, tc := range []struct {
		name       string
		p, d       Vec3
		lo, hi     float64
		start, end int64
		sm, em     BoreBoundaryMask
		hits       int
	}{
		{"floor", Vec3{X: -6}, Vec3{X: 1}, 0, 12, 7, 9, BoreFloor | BoreCylinder, BoreFloor | BoreCylinder, 2},
		{"wall", Vec3{X: 3, Z: -6}, Vec3{Z: 1}, 0, 12, 6, 10, BoreFloor | BoreCylinder, BoreSphere | BoreCylinder, 1},
		{"reverse", Vec3{X: 3, Z: 6}, Vec3{Z: -1}, 0, 12, 2, 6, BoreSphere | BoreCylinder, BoreFloor | BoreCylinder, 1},
		{"clipped", Vec3{X: -6}, Vec3{X: 1}, 8, 9, 8, 9, BoreFloor, BoreFloor | BoreCylinder, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, e := s.IntersectLineBoundary(context.Background(), tc.p, tc.d, tc.lo, tc.hi, o)
			if e != nil {
				t.Fatal(e)
			}
			if len(r.Intervals) != 1 || len(r.Hits) != tc.hits {
				t.Fatalf("%+v", r)
			}
			v := r.Intervals[0]
			check(v.Start, tc.start)
			check(v.End, tc.end)
			if v.StartBoundary != tc.sm || v.EndBoundary != tc.em {
				t.Fatalf("masks %+v", v)
			}
			if _, e := s.IntersectLine(context.Background(), tc.p, tc.d, tc.lo, tc.hi, o); e == nil {
				t.Fatal("old API accepted coincidence")
			}
		})
	}
	r, e := s.IntersectLineBoundary(context.Background(), Vec3{X: -6, Y: 1}, Vec3{X: 1}, 0, 12, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Intervals) != 0 || len(r.Hits) != 3 {
		t.Fatalf("tangent %+v", r)
	}
	check(r.Hits[1].Parameter, 8)
	if r.Hits[1].Multiplicity != 2 || r.Hits[1].Boundary != BoreFloor|BoreCylinder {
		t.Fatal(r.Hits[1])
	}
	r, e = s.IntersectLineBoundary(context.Background(), Vec3{X: 1}, Vec3{Z: 1}, 0, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Intervals) != 1 || len(r.Hits) != 0 {
		t.Fatal(r)
	}
	v := r.Intervals[0]
	if v.EndBounds.Lower.Sign() <= 0 || new(big.Rat).Mul(v.EndBounds.Lower, v.EndBounds.Lower).Cmp(big.NewRat(24, 1)) > 0 || new(big.Rat).Mul(v.EndBounds.Upper, v.EndBounds.Upper).Cmp(big.NewRat(24, 1)) < 0 {
		t.Fatal("wrong sqrt24 bracket")
	}
	v.EndBounds.Lower.SetInt64(99)
	fresh := v.End.Interval()
	if fresh.Lower.Cmp(big.NewRat(99, 1)) == 0 {
		t.Fatal("mutable root")
	}
	r, e = s.IntersectLineBoundary(context.Background(), Vec3{X: 2}, Vec3{X: 1}, 0, 0, o)
	if e != nil || len(r.Hits) != 1 || len(r.Intervals) != 0 {
		t.Fatalf("point range %+v %v", r, e)
	}
}

func TestOffAxisSphereBoreBoundaryLimits(t *testing.T) {
	s, _ := NewOffAxisSphereBore(5, 1, Vec2{X: 2}, 0)
	for _, o := range []ToroidLineOptions{{ParameterTolerance: 1e-8, Surface: SurfacePatchOptions{MaxOperations: 1}}, {ParameterTolerance: 1e-30, MaxSubdivisions: 1}} {
		r, e := s.IntersectLineBoundary(context.Background(), Vec3{X: 1}, Vec3{Z: 1}, 0, 6, o)
		if e == nil || len(r.Hits)+len(r.Intervals) != 0 {
			t.Fatalf("non-atomic limit %+v %v", r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := s.IntersectLineBoundary(ctx, Vec3{}, Vec3{X: 1}, 0, 1, ToroidLineOptions{ParameterTolerance: 1e-8})
	if e != context.Canceled || len(r.Hits)+len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
}
