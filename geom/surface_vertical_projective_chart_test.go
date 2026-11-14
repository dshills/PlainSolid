package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerticalProjectiveChartPreservesWholeSurface(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	// The base chart is (u,u²,v) and the original replaces u by
	// f(s)=2s/(1+s). Homogeneous control scaling implements f exactly.
	h := make([][4]*big.Rat, 0, 6)
	for i := int64(0); i < 3; i++ {
		k := int64(1) << i
		for j := int64(0); j < 2; j++ {
			y := int64(0)
			if i == 2 {
				y = k
			}
			h = append(h, [4]*big.Rat{q(i*k, 2), q(y, 1), q(j*k, 1), q(k, 1)})
		}
	}
	old, e := NewExactHomogeneousBezierSurface(ctx, 2, 1, h, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	k, work, e := old.VerticalProjectiveScale(ctx, 0, SurfacePatchEvaluationOptions{})
	if e != nil || work <= 0 || k.Cmp(q(2, 1)) != 0 {
		t.Fatal("exact weight ratio", k, work, e)
	}
	after, work, e := old.NormalizeVerticalProjectiveChart(ctx, 0, k, SurfacePatchOptions{})
	if e != nil || work <= 0 || after.Domain() != old.Domain() {
		t.Fatal("equivalent positive chart", after, work, e)
	}
	for _, u := range []*big.Rat{q(0, 1), q(1, 7), q(5, 9), q(1, 1)} {
		s := new(big.Rat).Quo(u, new(big.Rat).Sub(q(2, 1), u))
		a, e := old.EvaluateRational(ctx, s, q(5, 11), SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := after.EvaluateRational(ctx, u, q(5, 11), SurfacePatchEvaluationOptions{})
		want := [3]*big.Rat{u, new(big.Rat).Mul(u, u), q(5, 11)}
		for i, v := range want {
			if e != nil || a.Point[i].Cmp(v) != 0 || b.Point[i].Cmp(v) != 0 {
				t.Fatal("nonbinary projected parameter must preserve the whole model", u, a.Point, b.Point, e)
			}
		}
	}
	h[len(h)-1][3].Add(h[len(h)-1][3], q(1, 1<<54))
	bad, e := NewExactHomogeneousBezierSurface(ctx, 2, 1, h, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got, _, e := bad.VerticalProjectiveScale(ctx, 0, SurfacePatchEvaluationOptions{}); got != nil || !errors.Is(e, ErrUnrepresentable) {
		t.Fatal("independent weight perturbation must not be normalized by fitting", got, e)
	}
}
