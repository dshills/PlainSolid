package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestExactCurvePointInversionRationalAndAlgebraic(t *testing.T) {
	ctx := context.Background()
	s, _ := NewBezier([]Vec3{{}, {X: 1}, {X: 2, Y: 1}, {X: 3, Y: 3, Z: 1}}, nil)
	c := isoIntersectionFixture(t, s)
	point := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 3), big.NewRat(1, 27)}
	r, e := FindExactCurvePointParameters(ctx, c, point, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Roots) != 1 || r.Roots[0].Parameter.RatString() != "1/3" {
		t.Fatal(r, e)
	}
	r.Point[0].SetInt64(9)
	if point[0].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("alias")
	}
	// x=(2t-1)² revisits x=1/2 at two irrational parameters.
	s, _ = NewBezier([]Vec3{{X: 1}, {X: -1}, {X: 1}}, nil)
	c = isoIntersectionFixture(t, s)
	point = [3]*big.Rat{big.NewRat(1, 2), new(big.Rat), new(big.Rat)}
	r, e = FindExactCurvePointParameters(ctx, c, point, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Roots) != 2 {
		t.Fatal(r, e)
	}
	for _, root := range r.Roots {
		if root.Parameter != nil {
			t.Fatal(root)
		}
		mid, _ := new(big.Rat).Quo(new(big.Rat).Add(root.Lower, root.Upper), big.NewRat(2, 1)).Float64()
		if math.Abs((2*mid-1)*(2*mid-1)-.5) > 1e-8 {
			t.Fatal(root)
		}
	}
	point[0].SetInt64(0)
	r, e = FindExactCurvePointParameters(ctx, c, point, ExactCurveSegmentOptions{})
	if e != nil || len(r.Roots) != 1 || r.Roots[0].Multiplicity != 2 || r.Roots[0].Parameter.RatString() != "1/2" {
		t.Fatal(r, e)
	}
}
func TestExactCurvePointConstantAbsentAndLimits(t *testing.T) {
	ctx := context.Background()
	s, _ := NewBezier([]Vec3{{X: 1}, {X: 1}}, []float64{1, 2})
	c := isoIntersectionFixture(t, s)
	point := rationalPoint(1, 0, 0)
	r, e := FindExactCurvePointParameters(ctx, c, point, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.ConstantSpans) != 1 {
		t.Fatal(r, e)
	}
	point[2].SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	r, e = FindExactCurvePointParameters(ctx, c, point, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.ConstantSpans) != 0 || len(r.Roots) != 0 {
		t.Fatal(r, e)
	}
	_, e = FindExactCurvePointParameters(ctx, c, point, ExactCurveSegmentOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = FindExactCurvePointParameters(canceled, c, point, ExactCurveSegmentOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
