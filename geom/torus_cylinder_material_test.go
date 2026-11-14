package geom

import (
	"context"
	"math/big"
	"reflect"
	"testing"
)

func TestTorusCylinderMaterial(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	zero := [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}
	for _, op := range []TorusCylinderOperation{TorusCylinderCommon, TorusCylinderDifference} {
		s, e := NewTorusCylinderMaterial(5, 2, 6, -3, 4, op)
		if e != nil {
			t.Fatal(e)
		}
		line, e := s.ClipRationalLine(ctx, zero, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-8, 1), r(8, 1), o)
		if e != nil || len(line.Intervals) != 2 {
			t.Fatal(line, e)
		}
		expected := [4]int64{-6, -3, 3, 6}
		masks := [4]BoreBoundaryMask{2, 1, 1, 2}
		if op == TorusCylinderDifference {
			expected = [4]int64{-7, -6, 6, 7}
			masks = [4]BoreBoundaryMask{1, 2, 2, 1}
		}
		for i, v := range line.Intervals {
			for j, root := range []*ExactAlgebraicRoot{v.Start, v.End} {
				b := root.Interval()
				if b.Lower.Cmp(r(expected[2*i+j], 1)) != 0 || b.Upper.Cmp(b.Lower) != 0 {
					t.Fatal("end", b)
				}
			}
			if v.StartBoundary != masks[2*i] || v.EndBoundary != masks[2*i+1] {
				t.Fatal("mask", v)
			}
		}
		// A full generator lies on the cylinder; both irrational rim ends have two owners.
		line, e = s.ClipRationalLine(ctx, [3]*big.Rat{r(6, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(-3, 1), r(3, 1), o)
		if e != nil || len(line.Intervals) != 1 {
			t.Fatal(line, e)
		}
		v := line.Intervals[0]
		if v.Boundary != 2 || v.StartBoundary != 3 || v.EndBoundary != 3 {
			t.Fatal("rim/generator", v)
		}
		for _, root := range []*ExactAlgebraicRoot{v.Start, v.End} {
			sg, e := root.SignPolynomial(ctx, []*big.Rat{r(-3, 1), r(0, 1), r(1, 1)}, ExactCurvePlaneEventOptions{})
			if e != nil || sg.Sign != 0 {
				t.Fatal("sqrt3", sg, e)
			}
		}
	}
	for _, op := range []TorusCylinderOperation{TorusCylinderCommon, TorusCylinderDifference} {
		s, _ := NewTorusCylinderMaterial(5, 2, 6, -3, 4, op)
		v, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(7, 1), r(0, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-1, 1), r(1, 1), o)
		if e != nil || len(v.Intervals) != 0 {
			t.Fatal("outer tangent", v, e)
		}
		if op == TorusCylinderCommon {
			if len(v.Points) != 0 {
				t.Fatal("buried contact", v)
			}
		} else {
			if len(v.Points) != 1 || v.Points[0].Boundary != 1 || v.Points[0].Multiplicity != 2 {
				t.Fatal("isolated torus touch", v)
			}
		}
	}
	s, _ := NewTorusCylinderMaterial(5, 2, 5, -3, 3, TorusCylinderCommon)
	l, m, e := s.ClassifyRational(ctx, [3]*big.Rat{r(5, 1), r(0, 1), r(2, 1)}, SurfacePatchOptions{})
	if e != nil || l != BoreBoundary || m != 3 {
		t.Fatal(l, m, e)
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, sign := range []int64{-1, 1} {
		x := new(big.Rat).Add(r(5, 1), new(big.Rat).Mul(r(sign, 1), eps))
		l, _, e = s.ClassifyRational(ctx, [3]*big.Rat{x, r(0, 1), r(0, 1)}, SurfacePatchOptions{})
		want := BoreInside
		if sign > 0 {
			want = BoreOutside
		}
		if e != nil || l != want {
			t.Fatal("subfloat", l, e)
		}
	}
	// Tangent to the inner torus: a zero-width gap must not delete material.
	line, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(3, 1), r(0, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-1, 1), r(1, 1), o)
	if e != nil || len(line.Intervals) == 0 {
		t.Fatal(line, e)
	}
	for _, args := range [][5]float64{{5, 2, 3, -3, 3}, {5, 2, 7, -3, 3}, {5, 2, 5, -2, 3}, {5, 2, 5, -3, 2}, {2, 2, 2, -3, 3}} {
		if _, e := NewTorusCylinderMaterial(args[0], args[1], args[2], args[3], args[4], TorusCylinderCommon); e == nil {
			t.Fatal("domain", args)
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := s.ClipRationalLine(c, zero, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-8, 1), r(8, 1), o); e == nil || !reflect.DeepEqual(v, TorusCylinderLine{}) {
		t.Fatal("cancel")
	}
	o.Surface.MaxOperations = 1
	if v, e := s.ClipRationalLine(ctx, zero, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-8, 1), r(8, 1), o); e == nil || !reflect.DeepEqual(v, TorusCylinderLine{}) {
		t.Fatal("budget")
	}
}
