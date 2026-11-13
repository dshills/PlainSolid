package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestCertifiedGeneralFrustumDistance(t *testing.T) {
	ctx := context.Background()
	w, _, e := DraftWrappedFrustum(ctx, queryCylinder(t, false), 0, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := SurfaceDistanceOptions{SquaredTolerance: .005}
	for _, tc := range []struct {
		q    Vec3
		want *big.Rat
	}{
		{Vec3{X: 1, Z: .5}, big.NewRat(9, 40)},
		{Vec3{X: 2, Z: .5}, big.NewRat(9, 40)},
		{Vec3{Z: 4}, big.NewRat(4, 1)},
		{Vec3{X: 2, Z: 2}, big.NewRat(0, 1)},
		{Vec3{X: 3, Z: 3}, big.NewRat(2, 1)},
	} {
		r, e := BoundPointWrappedFrustumDistance(ctx, w, tc.q, o)
		if e != nil || r.Lower.Cmp(tc.want) > 0 || r.Upper.Cmp(tc.want) < 0 || new(big.Rat).Sub(r.Upper, r.Lower).Cmp(new(big.Rat).SetFloat64(.00501)) > 0 {
			t.Fatal(tc, r, e)
		}
		actual := new(big.Rat)
		for i, v := range []float64{tc.q.X, tc.q.Y, tc.q.Z} {
			d := new(big.Rat).Sub(r.Witness[i], new(big.Rat).SetFloat64(v))
			actual.Add(actual, new(big.Rat).Mul(d, d))
		}
		if actual.Cmp(r.Upper) != 0 {
			t.Fatal("exact witness upper")
		}
		if tc.q.X == 0 && r.VisitedCells != 0 {
			t.Fatal("cap projection wasted search")
		}
	}
	// The same bounded API reduces to the independently tested cylinder case.
	cylinder := queryCylinder(t, false)
	r, e := BoundPointWrappedFrustumDistance(ctx, cylinder, Vec3{X: 2, Z: .5}, o)
	if e != nil || r.Lower.Cmp(big.NewRat(1, 1)) > 0 || r.Upper.Cmp(big.NewRat(1, 1)) < 0 {
		t.Fatal(r, e)
	}
	bad := o
	bad.MaxOperations = 1
	r, e = BoundPointWrappedFrustumDistance(ctx, w, Vec3{X: 1, Z: .5}, bad)
	if e == nil || r.Lower != nil {
		t.Fatal("partial budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = BoundPointWrappedFrustumDistance(canceled, w, Vec3{}, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
