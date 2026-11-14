package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestPerpendicularCylinderTorusCavityMaterial(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	s, e := NewPerpendicularCylinderTorusCavityMaterial(3, 1, 6, -5, 5, Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		y int64
		l BorePointLocation
	}{{0, BoreInside}, {3, BoreOutside}, {4, BoreBoundary}} {
		l, _, e := s.ClassifyRational(ctx, [3]*big.Rat{r(1, 1), r(tc.y, 1), r(0, 1)}, SurfacePatchOptions{})
		if e != nil || l != tc.l {
			t.Fatal(tc, l, e)
		}
	}
	line, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(1, 1), r(0, 1)}, r(-7, 1), r(7, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || len(line.Intervals) != 3 {
		t.Fatal(line, e)
	}
	roots := []*ExactAlgebraicRoot{line.Intervals[0].End, line.Intervals[1].Start, line.Intervals[1].End, line.Intervals[2].Start}
	for i, n := range []int64{-4, -2, 2, 4} {
		b := roots[i].Interval()
		if b.Lower.Cmp(r(n, 1)) > 0 || b.Upper.Cmp(r(n, 1)) < 0 {
			t.Fatal("root")
		}
	}
	for _, center := range []Vec3{{X: 2}, {Z: 1}} {
		if _, e = NewPerpendicularCylinderTorusCavityMaterial(3, 1, 6, -5, 5, center); e == nil {
			t.Fatal("contact")
		}
	}
	if _, _, e = s.ClassifyRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
