package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCylinderTorusCavityMaterial(t *testing.T) {
	ctx := context.Background()
	q, e := NewCylinderTorusCavityMaterial(5, 2, 8, -3, 4)
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	for _, tc := range []struct {
		x    int64
		l    BorePointLocation
		mask BoreBoundaryMask
	}{{0, BoreInside, 0}, {5, BoreOutside, 0}, {7, BoreBoundary, TorusCylinderTorus}, {8, BoreBoundary, TorusCylinderWall}, {9, BoreOutside, 0}} {
		l, m, e := q.ClassifyRational(ctx, [3]*big.Rat{r(tc.x, 1), r(0, 1), r(0, 1)}, SurfacePatchOptions{})
		if e != nil || l != tc.l || m != tc.mask {
			t.Fatal(tc, l, m, e)
		}
	}
	p := [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}
	d := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	line, e := q.ClipRationalLine(ctx, p, d, r(-9, 1), r(9, 1), ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(line.Intervals) != 3 || len(line.Points) != 0 {
		t.Fatal(line, e)
	}
	expected := [][2]int64{{-8, -7}, {-3, 3}, {7, 8}}
	for i, x := range line.Intervals {
		for j, root := range []*ExactAlgebraicRoot{x.Start, x.End} {
			b := root.Interval()
			v := r(expected[i][j], 1)
			if b.Lower.Cmp(v) > 0 || b.Upper.Cmp(v) < 0 {
				t.Fatal("root")
			}
		}
	}
	if _, e = q.ClipRationalLine(ctx, p, d, r(-9, 1), r(9, 1), ToroidLineOptions{ParameterTolerance: 1e-10, Surface: SurfacePatchOptions{MaxOperations: 1}}); e == nil {
		t.Fatal("budget")
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, sign := range []int64{-1, 1} {
		x := new(big.Rat).Add(r(7, 1), new(big.Rat).Mul(r(sign, 1), eps))
		l, _, e := q.ClassifyRational(ctx, [3]*big.Rat{x, r(0, 1), r(0, 1)}, SurfacePatchOptions{})
		want := BoreInside
		if sign < 0 {
			want = BoreOutside
		}
		if e != nil || l != want {
			t.Fatal("subfloat", l, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := q.ClipRationalLine(cancelled, p, d, r(-9, 1), r(9, 1), ToroidLineOptions{ParameterTolerance: 1e-10}); e == nil {
		t.Fatal("cancel")
	}

	for _, c := range []float64{7, 6} {
		if _, e = NewCylinderTorusCavityMaterial(5, 2, c, -3, 4); e == nil {
			t.Fatal("contact")
		}
	}
}
