package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestCylinderPairCommonExactOblique(t *testing.T) {
	ctx := context.Background()
	origin := Vec3{X: 1, Y: -2, Z: 3}
	a := RawCylinder{Origin: origin, Axis: Vec3{X: 3, Y: 4}, Radius: 2, Min: -2, Max: 2}
	b := RawCylinder{Origin: origin, Axis: Vec3{X: 8, Y: 6}, Radius: 2, Min: -2, Max: 2}
	s, e := NewCylinderPairCommon(a, b)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []struct {
		p    Vec3
		loc  BorePointLocation
		mask BoreBoundaryMask
	}{
		{origin, BoreInside, 0},
		{origin.Add(Vec3{Z: 2}), BoreBoundary, RawCylinderFirstSide | RawCylinderSecondSide},
		{Vec3{X: origin.X, Y: origin.Y, Z: math.Nextafter(origin.Z+2, math.Inf(1))}, BoreOutside, 0},
	} {
		l, m, e := s.Classify(c.p)
		if e != nil || l != c.loc || m != c.mask {
			t.Fatal(c, l, m, e)
		}
	}
	o := ToroidLineOptions{ParameterTolerance: 1e-12}
	h, e := s.IntersectLineBoundary(ctx, origin, Vec3{Z: 2}, -4, 4, o)
	if e != nil || len(h.Hits) != 2 || len(h.Intervals) != 0 {
		t.Fatal(h, e)
	}
	for i, z := range []int64{-1, 1} {
		x := h.Hits[i]
		if x.Bounds.Lower.Cmp(big.NewRat(z, 1)) != 0 || x.Bounds.Upper.Cmp(big.NewRat(z, 1)) != 0 ||
			x.Boundary != RawCylinderFirstSide|RawCylinderSecondSide {
			t.Fatal(x)
		}
	}
	// One raw generator lies on the first wall, while the other
	// cylinder bounds it to the single shared tangent point.
	h, e = s.IntersectLineBoundary(ctx, origin.Add(Vec3{Z: 2}), a.Axis, -1, 1, o)
	if e != nil || len(h.Hits) != 1 || h.Hits[0].Multiplicity != 2 || len(h.Intervals) != 0 {
		t.Fatal(h, e)
	}
	a.Min, a.Max = -1, 1
	s, e = NewCylinderPairCommon(a, a)
	if e != nil {
		t.Fatal(e)
	}
	h, e = s.IntersectLineBoundary(ctx, origin.Add(Vec3{Z: 2}), a.Axis, -2, 2, o)
	if e != nil || len(h.Intervals) != 1 || len(h.Hits) != 0 {
		t.Fatal(h, e)
	}
	iv := h.Intervals[0]
	if iv.Boundary != RawCylinderFirstSide|RawCylinderSecondSide ||
		iv.StartBoundary != iv.Boundary|RawCylinderFirstMin|RawCylinderSecondMin ||
		iv.EndBoundary != iv.Boundary|RawCylinderFirstMax|RawCylinderSecondMax {
		t.Fatal(iv)
	}
}
