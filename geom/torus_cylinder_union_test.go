package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestTorusCylinderUnion(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	s, e := NewTorusCylinderMaterial(5, 2, 6, -3, 4, TorusCylinderUnion)
	if e != nil {
		t.Fatal(e)
	}
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	for _, tc := range []struct {
		p [3]*big.Rat
		l BorePointLocation
		m BoreBoundaryMask
	}{
		{[3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, BoreInside, 0},
		{[3]*big.Rat{r(3, 1), r(0, 1), r(0, 1)}, BoreInside, 0},
		{[3]*big.Rat{r(6, 1), r(0, 1), r(0, 1)}, BoreInside, 0},
		{[3]*big.Rat{r(7, 1), r(0, 1), r(0, 1)}, BoreBoundary, 1},
		{[3]*big.Rat{r(0, 1), r(0, 1), r(-3, 1)}, BoreBoundary, 4},
		{[3]*big.Rat{r(6, 1), r(0, 1), r(4, 1)}, BoreBoundary, 10},
		{[3]*big.Rat{r(7, 1), r(0, 1), r(4, 1)}, BoreOutside, 0},
	} {
		l, m, e := s.ClassifyRational(ctx, tc.p, SurfacePatchOptions{})
		if e != nil || l != tc.l || m != tc.m {
			t.Fatal(tc, l, m, e)
		}
	}
	for _, tc := range []struct {
		x          int64
		continuous bool
	}{{0, false}, {6, true}} {
		v, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(tc.x, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(-5, 1), r(5, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		if !tc.continuous {
			if len(v.Intervals) != 1 || v.Intervals[0].StartBoundary != 4 || v.Intervals[0].EndBoundary != 8 {
				t.Fatal(v)
			}
		} else {
			if len(v.Intervals) != 3 {
				t.Fatal("split hidden wall", v)
			}
			want := [3]BoreBoundaryMask{2, 0, 2}
			for i, w := range v.Intervals {
				if w.Boundary != want[i] {
					t.Fatal("wall buried", v)
				}
			}
			if v.Intervals[0].StartBoundary != 6 || v.Intervals[0].EndBoundary != 3 || v.Intervals[2].StartBoundary != 3 || v.Intervals[2].EndBoundary != 10 {
				t.Fatal("end owners", v)
			}
		}
	}
	// A line lying in a cap retains the full disk, with two-owner rim endpoints.
	v, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(4, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-8, 1), r(8, 1), o)
	if e != nil || len(v.Intervals) != 1 || v.Intervals[0].Boundary != 8 || v.Intervals[0].StartBoundary != 10 || v.Intervals[0].EndBoundary != 10 {
		t.Fatal(v, e)
	}
}
