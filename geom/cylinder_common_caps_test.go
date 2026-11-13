package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCylinderCommonCaps(t *testing.T) {
	u, v := Vec3{X: 2}, Vec3{X: 3, Y: 4}
	a := [2]*big.Rat{big.NewRat(-13, 8), big.NewRat(13, 8)}
	c := [2]*big.Rat{big.NewRat(-11, 20), big.NewRat(11, 20)}
	p, e := ClassifyCylinderCommonCaps(context.Background(), u, v, 1, 2, &a, &c, SurfacePatchEvaluationOptions{})
	if e != nil || !p.Complete || !p.Covers || p.TouchMask != 15 {
		t.Fatal(p, e)
	}
	// Independent in-plane maximizing point: y=1 and 4x-3y=10.
	// x=13/4, raw u projection=13/8. V-axis maximum exchanges roles.
	for i, x := range []*big.Rat{big.NewRat(13, 8), big.NewRat(11, 20)} {
		if p.Extents[i].Bounds[0].Cmp(x) > 0 || p.Extents[i].Bounds[1].Cmp(x) < 0 {
			t.Fatal("bounds")
		}
	}
	a[1] = big.NewRat(3, 2)
	p, e = ClassifyCylinderCommonCaps(context.Background(), u, v, 1, 2, &a, &c, SurfacePatchEvaluationOptions{})
	if e != nil || p.Covers {
		t.Fatal(p, e)
	}
	// Irrational angle and unequal norms; exact comparisons around extent.
	a = [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}
	p, e = ClassifyCylinderCommonCaps(context.Background(), Vec3{X: 1}, Vec3{X: 1, Y: 1, Z: 1}, 1, 1, &a, nil, SurfacePatchEvaluationOptions{})
	if e != nil || !p.Covers || p.TouchMask != 0 {
		t.Fatal(p, e)
	}
	a[1] = big.NewRat(1, 1)
	p, e = ClassifyCylinderCommonCaps(context.Background(), Vec3{X: 1}, Vec3{X: 1, Y: 1, Z: 1}, 1, 1, &a, nil, SurfacePatchEvaluationOptions{})
	if e != nil || p.Covers {
		t.Fatal(p, e)
	}
	if p, e = ClassifyCylinderCommonCaps(context.Background(), u, v, 1, 2, nil, nil, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil || p.Complete || p.Covers {
		t.Fatal("budget", p, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ClassifyCylinderCommonCaps(ctx, u, v, 1, 2, nil, nil, SurfacePatchEvaluationOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e = ClassifyCylinderCommonCaps(context.Background(), u, u, 1, 2, nil, nil, SurfacePatchEvaluationOptions{}); e != ErrDegenerate {
		t.Fatal(e)
	}
}
