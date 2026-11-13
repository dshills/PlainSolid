package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestCertifiedCircularFrustumBoundaryDistance(t *testing.T) {
	ctx := context.Background()
	cylinder := queryCylinder(t, true)
	o := FrustumDistanceOptions{SquaredTolerance: 1e-12}
	for _, tc := range []struct {
		q         Vec3
		want      *big.Rat
		part      CylinderBoundaryPart
		nonunique bool
	}{
		{Vec3{Z: .5}, big.NewRat(1, 1), CylinderLateral, true},
		{Vec3{Z: -2}, big.NewRat(1, 1), CylinderLowerCap, false},
		{Vec3{X: 3, Y: 4, Z: .5}, big.NewRat(16, 1), CylinderLateral, false},
		{Vec3{X: 1, Z: 2}, big.NewRat(0, 1), CylinderLateral, false},
	} {
		r, e := BoundPointWrappedCircularFrustumDistance(ctx, cylinder, tc.q, o)
		if e != nil || r.Lower.Cmp(tc.want) != 0 || r.Upper.Cmp(tc.want) != 0 {
			t.Fatal(tc, r, e)
		}
		if tc.want.Sign() != 0 && r.Witness.Part != tc.part {
			t.Fatal("witness part", r)
		}
		if r.Witness.RotationallyNonunique != tc.nonunique {
			t.Fatal("nonunique axis", r)
		}
	}
	frustum, _, e := DraftWrappedFrustum(ctx, cylinder, 0, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundPointWrappedCircularFrustumDistance(ctx, frustum, Vec3{Z: .5}, o)
	if e != nil || r.Lower.Cmp(big.NewRat(81, 40)) != 0 || r.Upper.Cmp(big.NewRat(81, 40)) != 0 || r.Witness.Radius.Cmp(big.NewRat(27, 20)) != 0 || r.Witness.Height.Cmp(big.NewRat(1, 20)) != 0 || !r.Witness.RotationallyNonunique {
		t.Fatal("independent trapezoid projection", r, e)
	}
	r, e = BoundPointWrappedCircularFrustumDistance(ctx, cylinder, Vec3{X: 1, Y: 1, Z: .5}, o)
	if e != nil {
		t.Fatal(e)
	}
	// True squared distance is3−2sqrt(2); compare exact squares to8.
	a := new(big.Rat).Sub(big.NewRat(3, 1), r.Lower)
	b := new(big.Rat).Sub(big.NewRat(3, 1), r.Upper)
	if a.Sign() < 0 || new(big.Rat).Mul(a, a).Cmp(big.NewRat(8, 1)) < 0 || b.Sign() < 0 || new(big.Rat).Mul(b, b).Cmp(big.NewRat(8, 1)) > 0 || new(big.Rat).Sub(r.Upper, r.Lower).Cmp(new(big.Rat).SetFloat64(o.SquaredTolerance)) > 0 {
		t.Fatal("radical enclosure", r)
	}
	if r.Witness.AzimuthSquared.Cmp(big.NewRat(2, 1)) != 0 || r.Witness.Radius.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("algebraic witness", r)
	}
	bad := o
	bad.MaxBisections = 1
	denied, e := BoundPointWrappedCircularFrustumDistance(ctx, cylinder, Vec3{X: 1, Y: 1, Z: .5}, bad)
	if !errors.Is(e, ErrConvergence) || denied.Lower != nil {
		t.Fatal("partial budget", denied, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = BoundPointWrappedCircularFrustumDistance(canceled, cylinder, Vec3{}, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = BoundPointWrappedCircularFrustumDistance(ctx, queryCylinder(t, false), Vec3{}, o)
	if !errors.Is(e, ErrConvergence) {
		t.Fatal("noncircle accepted", e)
	}
}
