package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestPerpendicularCylinderCommonQuery(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-12}
	s, e := NewPerpendicularCylinderCommon(5, 3, [2]float64{-2, 2}, [2]float64{-4, 4})
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []struct {
		p Vec3
		l BorePointLocation
		m BoreBoundaryMask
	}{
		{Vec3{}, BoreInside, 0},
		{Vec3{X: 2, Z: 4}, BoreBoundary, PerpendicularCylinderXUpper | PerpendicularCylinderZUpper},
		{Vec3{Y: 3, Z: 4}, BoreBoundary, PerpendicularCylinderXSide | PerpendicularCylinderZSide | PerpendicularCylinderZUpper},
		{Vec3{Y: 3, Z: 4.125}, BoreOutside, 0},
	} {
		l, m, e := s.Classify(c.p)
		if e != nil || l != c.l || m != c.m {
			t.Fatal(c, l, m, e)
		}
	}
	// The line is on the Z cylinder and clips against both the X
	// cylinder and the Z-cylinder axial caps at exactly the same roots.
	out, e := s.IntersectLineBoundary(ctx, Vec3{Y: 3}, Vec3{Z: 2}, -3, 3, o)
	if e != nil || len(out.Intervals) != 1 || len(out.Hits) != 0 {
		t.Fatal(out, e)
	}
	iv := out.Intervals[0]
	if iv.StartBounds.Lower.Cmp(big.NewRat(-2, 1)) != 0 || iv.EndBounds.Upper.Cmp(big.NewRat(2, 1)) != 0 ||
		iv.Boundary != PerpendicularCylinderZSide ||
		iv.StartBoundary != PerpendicularCylinderXSide|PerpendicularCylinderZSide|PerpendicularCylinderZLower ||
		iv.EndBoundary != PerpendicularCylinderXSide|PerpendicularCylinderZSide|PerpendicularCylinderZUpper {
		t.Fatal(iv)
	}
	q, e := s.IntersectLineBoundary(ctx, Vec3{Y: 2}, Vec3{X: 1, Z: 1}, -4, 4, o)
	if e != nil || len(q.Hits) != 2 || len(q.Intervals) != 0 || q.Hits[0].Boundary != PerpendicularCylinderXLower || q.Hits[1].Boundary != PerpendicularCylinderXUpper {
		t.Fatal(q, e)
	}
	q, e = s.IntersectLineBoundary(ctx, Vec3{Z: 3}, Vec3{Y: 1}, -4, 4, o)
	if e != nil || len(q.Hits) != 2 || q.Hits[0].Boundary != PerpendicularCylinderZSide || q.Hits[1].Boundary != PerpendicularCylinderZSide {
		t.Fatal(q, e)
	}
}
