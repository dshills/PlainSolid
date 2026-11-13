package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCylinderPairSetLine(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	a := RawCylinder{Axis: Vec3{Z: 2}, Radius: 2, Min: -1, Max: 1}
	b := RawCylinder{Axis: Vec3{Z: 3}, Radius: 1, Min: -1, Max: 1}
	s, e := NewCylinderPairCommon(a, b)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.ClipLineClosedDifference(ctx, Vec3{}, Vec3{X: 2}, -2, 2, o)
	if e != nil || len(r.Intervals) != 2 || len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	want := [2][2]*big.Rat{{big.NewRat(-1, 1), big.NewRat(-1, 2)}, {big.NewRat(1, 2), big.NewRat(1, 1)}}
	for i, iv := range r.Intervals {
		if iv.StartBounds.Lower.Cmp(want[i][0]) != 0 || iv.EndBounds.Upper.Cmp(want[i][1]) != 0 {
			t.Fatal(iv)
		}
	}
	if r.Intervals[0].StartBoundary != RawCylinderFirstSide || r.Intervals[0].EndBoundary != RawCylinderSecondSide {
		t.Fatal("masks", r)
	}
	r, e = s.ClipLineUnion(ctx, Vec3{}, Vec3{X: 2}, -2, 2, o)
	if e != nil || len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	for _, iv := range r.Intervals {
		if iv.Boundary != 0 {
			t.Fatal("hidden boundary", iv)
		}
	}
	// Identical operands: closed difference keeps lower-dimensional wall data.
	s, _ = NewCylinderPairCommon(a, a)
	r, e = s.ClipLineClosedDifference(ctx, Vec3{}, Vec3{X: 1}, -3, 3, o)
	if e != nil || len(r.Points) != 2 || len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
	r, e = s.ClipLineClosedDifference(ctx, Vec3{X: 2}, Vec3{Z: 2}, -2, 2, o)
	if e != nil || len(r.Intervals) != 1 || r.Intervals[0].Boundary != RawCylinderFirstSide|RawCylinderSecondSide {
		t.Fatal(r, e)
	}
	// Nonparallel raw axes, irrational isolated cutter endpoints.
	b = RawCylinder{Axis: Vec3{X: 3, Y: 4}, Radius: 1, Min: -2, Max: 2}
	s, _ = NewCylinderPairCommon(a, b)
	r, e = s.ClipLineClosedDifference(ctx, Vec3{X: .5}, Vec3{Z: 1}, -3, 3, o)
	if e != nil || len(r.Intervals) != 2 {
		t.Fatal(r, e)
	}
	for _, iv := range r.Intervals {
		if iv.StartBounds.Lower.Cmp(iv.StartBounds.Upper) > 0 || iv.EndBounds.Lower.Cmp(iv.EndBounds.Upper) > 0 {
			t.Fatal("bounds")
		}
	}
	cancelctx, cancel := context.WithCancel(ctx)
	cancel()
	r, e = s.ClipLineUnion(cancelctx, Vec3{}, Vec3{X: 1}, -1, 1, o)
	if e != context.Canceled || len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
}
