package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSphereCylinderMaterialDifference(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	s, e := NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 1}, -2, 2)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.ClipLine(ctx, Vec3{X: 1}, Vec3{Z: 1}, -6, 6, o)
	if e != nil || len(r.Intervals) != 2 || len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	sq := func(r *big.Rat) *big.Rat { return new(big.Rat).Mul(r, r) }
	a, b := r.Intervals[0], r.Intervals[1]
	if sq(a.StartBounds.Lower).Cmp(big.NewRat(24, 1)) < 0 || sq(a.StartBounds.Upper).Cmp(big.NewRat(24, 1)) > 0 || a.EndBounds.Upper.Cmp(big.NewRat(-2, 1)) != 0 || b.StartBounds.Lower.Cmp(big.NewRat(2, 1)) != 0 || sq(b.EndBounds.Lower).Cmp(big.NewRat(24, 1)) > 0 || sq(b.EndBounds.Upper).Cmp(big.NewRat(24, 1)) < 0 {
		t.Fatal("independent radical bounds", r)
	}
	if a.Boundary != 0 || b.Boundary != 0 || a.StartBoundary != BoreSphere || a.EndBoundary != BoreLowerCap || b.StartBoundary != BoreUpperCap || b.EndBoundary != BoreSphere {
		t.Fatal("support ownership", r)
	}
	// The exposed tool cap is material boundary along the whole circular chord.
	r, e = s.ClipLine(ctx, Vec3{Z: 2}, Vec3{X: 1}, -6, 6, o)
	if e != nil || len(r.Intervals) != 3 {
		t.Fatal("cap plane", r, e)
	}
	mid := r.Intervals[1]
	if mid.Boundary != BoreUpperCap || mid.StartBounds.Lower.Cmp(big.NewRat(-2, 1)) != 0 || mid.EndBounds.Upper.Cmp(big.NewRat(4, 1)) != 0 || mid.StartBoundary != BoreCylinder|BoreUpperCap || mid.EndBoundary != BoreCylinder|BoreUpperCap {
		t.Fatal(mid)
	}
	// Crossing the cap by one ULP changes topology exactly, not by root tolerance.
	for _, tc := range []struct {
		z     float64
		count int
	}{{math.Nextafter(2, 3), 1}, {math.Nextafter(2, 1), 2}} {
		near, e := s.ClipLine(ctx, Vec3{Z: tc.z}, Vec3{X: 1}, -6, 6, o)
		if e != nil || len(near.Intervals) != tc.count {
			t.Fatal("one-ULP cap side", tc, near, e)
		}
		for _, iv := range near.Intervals {
			if iv.Boundary != 0 {
				t.Fatal("spurious cap coincidence", iv)
			}
		}
	}
	// Raw parameter reversal/scaling preserves endpoints and swaps cap ancestry.
	r, e = s.ClipLine(ctx, Vec3{X: 1}, Vec3{Z: -2}, -3, 3, o)
	if e != nil || len(r.Intervals) != 2 || r.Intervals[0].EndBounds.Upper.Cmp(big.NewRat(-1, 1)) != 0 || r.Intervals[0].EndBoundary != BoreUpperCap {
		t.Fatal("raw units", r, e)
	}
}
func TestSphereCylinderMaterialRegularization(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	empty, e := NewOffAxisSphereFiniteCylinderDifference(5, 6, Vec2{}, -6, 6)
	if e != nil {
		t.Fatal(e)
	}
	r, e := empty.ClipLine(ctx, Vec3{}, Vec3{X: 1}, -10, 10, o)
	if e != nil || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal("empty regularized body", r, e)
	}
	unchanged, e := NewOffAxisSphereFiniteCylinderDifference(5, 2, Vec2{X: 7}, -2, 2)
	if e != nil {
		t.Fatal(e)
	}
	r, e = unchanged.ClipLine(ctx, Vec3{}, Vec3{X: 1}, -10, 10, o)
	if e != nil || len(r.Intervals) != 1 || r.Intervals[0].EndBoundary != BoreSphere || r.Intervals[0].EndBounds.Upper.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("external touch unchanged", r, e)
	}
	u, e := NewOffAxisSphereFiniteCylinderUnion(5, 3, Vec2{X: 1}, -6, 6)
	if e != nil {
		t.Fatal(e)
	}
	r, e = u.ClipLine(ctx, Vec3{X: 1}, Vec3{Z: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 1 || r.Intervals[0].Boundary != 0 || r.Intervals[0].StartBoundary != BoreLowerCap || r.Intervals[0].EndBoundary != BoreUpperCap || r.Intervals[0].StartBounds.Lower.Cmp(big.NewRat(-6, 1)) != 0 || r.Intervals[0].EndBounds.Upper.Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatal("hidden sphere roots", r, e)
	}
	u, e = NewOffAxisSphereFiniteCylinderUnion(5, 2, Vec2{X: 8}, -2, 2)
	if e != nil {
		t.Fatal(e)
	}
	r, e = u.ClipLine(ctx, Vec3{}, Vec3{X: 1}, -10, 12, o)
	if e != nil || len(r.Intervals) != 2 || r.Intervals[0].EndBounds.Upper.Cmp(big.NewRat(5, 1)) != 0 || r.Intervals[1].StartBounds.Lower.Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatal("separated material components", r, e)
	}
	r, e = u.ClipLine(ctx, Vec3{Y: 5}, Vec3{X: 1}, -2, 2, o)
	if e != nil || len(r.Points) != 1 || len(r.Intervals) != 0 || r.Points[0].Boundary != BoreSphere || r.Points[0].Multiplicity != 2 {
		t.Fatal("line tangent point", r, e)
	}
	// Constructor rejection remains unchanged at the internally tangent equator.
	if _, e = NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 2}, -2, 2); e == nil {
		t.Fatal("unqualified tangency")
	}
}
func TestSphereCylinderMaterialLineLimits(t *testing.T) {
	s, _ := NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 1}, -2, 2)
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := s.ClipLine(ctx, Vec3{X: 1}, Vec3{Z: 1}, -6, 6, o)
	if e != context.Canceled || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	o.Surface.MaxOperations = 1
	r, e = s.ClipLine(context.Background(), Vec3{X: 1}, Vec3{Z: 1}, -6, 6, o)
	if e == nil || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal("atomic limit", r, e)
	}
	o.Surface.MaxOperations = 0
	r, e = s.ClipLine(context.Background(), Vec3{}, Vec3{X: math.NaN()}, -1, 1, o)
	if e == nil || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal("invalid", r, e)
	}
}

func TestSphereCylinderMaterialFiniteClipping(t *testing.T) {
	s, _ := NewOffAxisSphereFiniteCylinderUnion(5, 3, Vec2{X: 1}, -6, 6)
	r, e := s.ClipLine(context.Background(), Vec3{X: 1}, Vec3{Z: 2}, -1, 1, ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || len(r.Intervals) != 1 || r.Intervals[0].Boundary != 0 || r.Intervals[0].StartBoundary != 0 || r.Intervals[0].EndBoundary != 0 {
		t.Fatal("finite artificial limits", r, e)
	}
	// Mutating exported rational brackets must not mutate the source recipe.
	r.Intervals[0].StartBounds.Lower.SetInt64(99)
	r, e = s.ClipLine(context.Background(), Vec3{X: 1}, Vec3{Z: 2}, -1, 1, ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || r.Intervals[0].StartBounds.Lower.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("alias", r, e)
	}
}
