package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestOffAxisSphereThroughBore(t *testing.T) {
	s, e := NewOffAxisSphereThroughBore(5, 1, Vec2{X: 2})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p Vec3
		l BorePointLocation
		m BoreBoundaryMask
	}{
		{Vec3{}, BoreInside, 0}, {Vec3{X: 2}, BoreOutside, 0}, {Vec3{X: 2, Z: -4}, BoreOutside, 0},
		{Vec3{X: 3}, BoreBoundary, BoreCylinder}, {Vec3{X: 3, Z: 4}, BoreBoundary, BoreSphere | BoreCylinder},
		{Vec3{X: 3, Z: -4}, BoreBoundary, BoreSphere | BoreCylinder}, {Vec3{Z: 5}, BoreBoundary, BoreSphere},
		{Vec3{X: math.Nextafter(3, 2)}, BoreOutside, 0}, {Vec3{X: math.Nextafter(3, 4)}, BoreInside, 0},
	} {
		l, m, e := s.Classify(tc.p)
		if e != nil || l != tc.l || m != tc.m {
			t.Fatalf("%+v: %v %v %v", tc.p, l, m, e)
		}
	}
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	ctx := context.Background()
	rootIs := func(r *ExactAlgebraicRoot, v int64) {
		t.Helper()
		q, _, e := NewExactRationalRoot(ctx, big.NewRat(v, 1), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		c, e := r.Compare(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil || !c.Proven || c.Order != 0 {
			t.Fatalf("expected %d: %+v %v", v, c, e)
		}
	}
	hits, e := s.IntersectLine(ctx, Vec3{X: -6}, Vec3{X: 1}, 0, 12, o)
	if e != nil || len(hits) != 4 {
		t.Fatal(hits, e)
	}
	for i, v := range []int64{1, 7, 9, 11} {
		rootIs(hits[i].Parameter, v)
		if hits[i].Boundary&BoreFloor != 0 {
			t.Fatal("floor in through query")
		}
	}
	// Through material has no hidden lower sphere hit on the bore axis.
	hits, e = s.IntersectLine(ctx, Vec3{X: 2, Z: -6}, Vec3{Z: 1}, 0, 12, o)
	if e != nil || len(hits) != 0 {
		t.Fatal(hits, e)
	}
	for _, dir := range []float64{1, -1} {
		r, e := s.IntersectLineBoundary(ctx, Vec3{X: 3, Z: -6 * dir}, Vec3{Z: dir}, 0, 12, o)
		if e != nil || len(r.Hits) != 0 || len(r.Intervals) != 1 {
			t.Fatal(r, e)
		}
		v := r.Intervals[0]
		rootIs(v.Start, 2)
		rootIs(v.End, 10)
		if v.Boundary != BoreCylinder || v.StartBoundary != BoreCylinder|BoreSphere || v.EndBoundary != BoreCylinder|BoreSphere {
			t.Fatal(v)
		}
	}
	r, e := s.IntersectLineBoundary(ctx, Vec3{X: 3}, Vec3{Z: 1}, -1, 1, o)
	if e != nil || len(r.Intervals) != 1 {
		t.Fatal(r, e)
	}
	v := r.Intervals[0]
	rootIs(v.Start, -1)
	rootIs(v.End, 1)
	if v.StartBoundary != BoreCylinder || v.EndBoundary != BoreCylinder {
		t.Fatal(v)
	}
	r, e = s.IntersectLineBoundary(ctx, Vec3{X: 1}, Vec3{Z: 1}, -6, 6, o)
	if e != nil || len(r.Intervals) != 1 {
		t.Fatal(r, e)
	}
	v = r.Intervals[0]
	for _, b := range []ExactParameterInterval{v.StartBounds, v.EndBounds} {
		lo, hi := new(big.Rat).Mul(b.Lower, b.Lower), new(big.Rat).Mul(b.Upper, b.Upper)
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if lo.Cmp(big.NewRat(24, 1)) > 0 || hi.Cmp(big.NewRat(24, 1)) < 0 {
			t.Fatal("sqrt24 bracket", b)
		}
	}
	// Shared rim roots on a noncoincident line retain both supports.
	hits, e = s.IntersectLine(ctx, Vec3{Z: 4}, Vec3{X: 1}, 0, 4, o)
	if e != nil || len(hits) != 2 {
		t.Fatal(hits, e)
	}
	rootIs(hits[1].Parameter, 3)
	if hits[1].Boundary != BoreSphere|BoreCylinder {
		t.Fatal(hits[1])
	}
}

func TestOffAxisSphereThroughBoreLimits(t *testing.T) {
	for _, offset := range []Vec2{{X: 4}, {X: 5}, {X: math.Inf(1)}} {
		if _, e := NewOffAxisSphereThroughBore(5, 1, offset); e == nil {
			t.Fatal("invalid footprint", offset)
		}
	}
	var zero OffAxisSphereThroughBore
	if _, _, e := zero.Classify(Vec3{}); e == nil {
		t.Fatal("zero accepted")
	}
	s, _ := NewOffAxisSphereThroughBore(5, 1, Vec2{X: 2})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := s.IntersectLineBoundary(ctx, Vec3{X: 1}, Vec3{Z: 1}, -6, 6, ToroidLineOptions{ParameterTolerance: 1e-8})
	if e != context.Canceled || len(r.Hits)+len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
	r, e = s.IntersectLineBoundary(context.Background(), Vec3{X: 1}, Vec3{Z: 1}, -6, 6, ToroidLineOptions{ParameterTolerance: 1e-8, Surface: SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || len(r.Hits)+len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
}
