package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestExactSplineRationalEvaluationRetainsOriginalKnotSide(t *testing.T) {
	c, err := NewBSpline(1, []Vec3{{}, {X: 1}, {X: 3}}, []float64{0, 0, .5, 1, 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	x, err := NewExactSplineCurve(c, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, v := range []struct{ t, p, d *big.Rat }{
		{big.NewRat(1, 3), big.NewRat(2, 3), big.NewRat(2, 1)},
		{big.NewRat(1, 2), big.NewRat(1, 1), big.NewRat(4, 1)},
		{big.NewRat(1, 1), big.NewRat(3, 1), big.NewRat(4, 1)},
	} {
		got, err := x.EvaluateRational(ctx, v.t, SurfacePatchEvaluationOptions{})
		if err != nil || got.Point[0].Cmp(v.p) != 0 || got.Derivative[0].Cmp(v.d) != 0 {
			t.Fatal("original curve station or one-sided physical derivative changed", v, got, err)
		}
		lo, hi := got.PointBounds.Min.X, got.PointBounds.Max.X
		if new(big.Rat).SetFloat64(lo).Cmp(v.p) > 0 || new(big.Rat).SetFloat64(hi).Cmp(v.p) < 0 ||
			(lo != hi && math.Nextafter(lo, math.Inf(1)) != hi) {
			t.Fatal("curve must retain directed numerical enclosures", v, got.PointBounds)
		}
	}
	if _, err := x.EvaluateRational(ctx, big.NewRat(1, 3), SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(err, ErrConvergence) {
		t.Fatal("short exact evaluation must refuse", err)
	}
}
