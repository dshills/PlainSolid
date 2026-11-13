package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestCertifiedCircularToroidDistance(t *testing.T) {
	ctx := context.Background()
	w := locationToroid(t, true)
	o := FrustumDistanceOptions{SquaredTolerance: 1e-12}
	for _, tc := range []struct {
		q            Vec3
		want         *big.Rat
		axis, center bool
	}{{Vec3{}, big.NewRat(9, 4), true, false}, {Vec3{X: 2}, big.NewRat(1, 4), false, true}, {Vec3{X: 2.5}, big.NewRat(0, 1), false, false}, {Vec3{X: 2, Z: 1}, big.NewRat(1, 4), false, false}, {Vec3{X: 3, Y: 4}, big.NewRat(25, 4), false, false}} {
		r, e := BoundPointWrappedCircularToroidDistance(ctx, w, tc.q, o)
		if e != nil || r.Lower.Cmp(tc.want) > 0 || r.Upper.Cmp(tc.want) < 0 || r.AxisNonunique != tc.axis || r.CenterlineNonunique != tc.center {
			t.Fatal(tc, r, e)
		}
	}
	r, e := BoundPointWrappedCircularToroidDistance(ctx, w, Vec3{X: 1, Y: 1}, o)
	if e != nil {
		t.Fatal(e)
	}
	// True squared distance=(3/2−sqrt2)^2=17/4−3sqrt2.
	a := new(big.Rat).Sub(big.NewRat(17, 4), r.Lower)
	b := new(big.Rat).Sub(big.NewRat(17, 4), r.Upper)
	if a.Sign() < 0 || new(big.Rat).Mul(a, a).Cmp(big.NewRat(18, 1)) < 0 || b.Sign() < 0 || new(big.Rat).Mul(b, b).Cmp(big.NewRat(18, 1)) > 0 || new(big.Rat).Sub(r.Upper, r.Lower).Cmp(new(big.Rat).SetFloat64(o.SquaredTolerance)) > 0 {
		t.Fatal("radical bound", r)
	}
	bad := o
	bad.MaxBisections = 1
	denied, e := BoundPointWrappedCircularToroidDistance(ctx, w, Vec3{X: 1, Y: 1}, bad)
	if !errors.Is(e, ErrConvergence) || denied.Lower != nil {
		t.Fatal("partial limit", e)
	}
	_, e = BoundPointWrappedCircularToroidDistance(ctx, locationToroid(t, false), Vec3{}, o)
	if !errors.Is(e, ErrConvergence) {
		t.Fatal("noncircle", e)
	}
}
