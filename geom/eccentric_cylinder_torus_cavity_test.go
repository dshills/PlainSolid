package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestEccentricCylinderTorusCavityMaterial(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	q, e := NewEccentricCylinderTorusCavityMaterial(5, 2, 10, -3, 4, 1, 1)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		x int64
		l BorePointLocation
	}{{1, BoreInside}, {6, BoreOutside}, {8, BoreBoundary}} {
		l, _, e := q.ClassifyRational(ctx, [3]*big.Rat{r(tc.x, 1), r(1, 1), r(0, 1)}, SurfacePatchOptions{})
		if e != nil || l != tc.l {
			t.Fatal(tc, l, e)
		}
	}
	line, e := q.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(1, 1), r(0, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-11, 1), r(11, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || len(line.Intervals) != 3 {
		t.Fatal(line, e)
	}
	roots := []*ExactAlgebraicRoot{line.Intervals[0].End, line.Intervals[1].Start, line.Intervals[1].End, line.Intervals[2].Start}
	for i, want := range []int64{-6, -2, 4, 8} {
		b := roots[i].Interval()
		if b.Lower.Cmp(r(want, 1)) > 0 || b.Upper.Cmp(r(want, 1)) < 0 {
			t.Fatal("shifted roots")
		}
	}
	for _, offset := range []float64{3, 4} {
		if _, e = NewEccentricCylinderTorusCavityMaterial(5, 2, 10, -3, 4, offset, 0); e == nil {
			t.Fatal("contact")
		}
	}
	if _, e := NewEccentricCylinderTorusCavityMaterial(5, 2, 10, -3, 4, math.Nextafter(3, 0), 0); e != nil {
		t.Fatal("exact tiny clearance", e)
	}
	if _, e := NewEccentricCylinderTorusCavityMaterial(5, 2, 10, -2, 4, 1, 1); e == nil {
		t.Fatal("cap contact")
	}
	if _, _, e = q.ClassifyRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
