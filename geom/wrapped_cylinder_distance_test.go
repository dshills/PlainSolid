package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestGeneralRationalCylinderDistance(t *testing.T) {
	ctx := context.Background()
	w := queryCylinder(t, false)
	o := CurveDistanceOptions{SquaredTolerance: 1e-3}
	for _, tc := range []struct {
		q    Vec3
		want *big.Rat
		part CylinderBoundaryPart
	}{
		{Vec3{Z: .5}, big.NewRat(1, 1), CylinderLateral},
		{Vec3{X: 2, Z: .5}, big.NewRat(1, 1), CylinderLateral},
		{Vec3{Z: -2}, big.NewRat(1, 1), CylinderLowerCap},
		{Vec3{X: 2, Z: 3}, big.NewRat(2, 1), CylinderLateral},
		{Vec3{X: .5, Y: .5, Z: 2}, big.NewRat(0, 1), CylinderUpperCap},
	} {
		r, e := BoundPointWrappedCylinderDistance(ctx, w, tc.q, o)
		if e != nil || r.Lower.Cmp(tc.want) > 0 || r.Upper.Cmp(tc.want) < 0 || new(big.Rat).Sub(r.Upper, r.Lower).Cmp(new(big.Rat).SetFloat64(.00101)) > 0 {
			t.Fatal(tc, r, e)
		}
		actual := new(big.Rat)
		coordinates := []float64{tc.q.X, tc.q.Y, tc.q.Z}
		for i, x := range coordinates {
			d := new(big.Rat).Sub(r.Witness[i], new(big.Rat).SetFloat64(x))
			actual.Add(actual, new(big.Rat).Mul(d, d))
		}
		if actual.Cmp(r.Upper) != 0 {
			t.Fatal("upper not actual exact witness")
		}
	}
	// Surface queries agree with independent exact conic points on the .75 arc.
	r, e := BoundPointWrappedCylinderDistance(ctx, w, Vec3{X: .875, Y: .5, Z: .5}, o)
	if e != nil || r.Lower.Sign() != 0 || r.Upper.Cmp(new(big.Rat).SetFloat64(.00101)) > 0 {
		t.Fatal("exact rational arc point", r, e)
	}
	bad := o
	bad.MaxOperations = 1
	r, e = BoundPointWrappedCylinderDistance(ctx, w, Vec3{}, bad)
	if e == nil || r.Lower != nil {
		t.Fatal("partial budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = BoundPointWrappedCylinderDistance(canceled, w, Vec3{}, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
