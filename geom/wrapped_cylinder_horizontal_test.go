package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestRationalCylinderHorizontalExactQuadrants(t *testing.T) {
	ctx := context.Background()
	w := queryCylinder(t, false)
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	for _, tc := range []struct {
		origin, direction Vec3
		roots             []*big.Rat
		overlap           int
	}{
		{Vec3{}, Vec3{X: 1}, []*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 1)}, 0},
		{Vec3{Y: .5}, Vec3{X: 1}, []*big.Rat{big.NewRat(-7, 8), big.NewRat(7, 8)}, 0},
		{Vec3{}, Vec3{X: 1, Y: 1}, []*big.Rat{big.NewRat(-5, 7), big.NewRat(5, 7)}, 0},
		{Vec3{Y: -.5, Z: 2}, Vec3{X: 1}, []*big.Rat{big.NewRat(-7, 8), big.NewRat(7, 8)}, 1},
		{Vec3{X: 1, Z: -1}, Vec3{Y: 1}, []*big.Rat{big.NewRat(0, 1)}, 0},
	} {
		r, e := IntersectWrappedCylinderHorizontalLine(ctx, w, tc.origin, tc.direction, -2, 2, o)
		if e != nil || len(r.Hits) != len(tc.roots) || len(r.Overlaps) != tc.overlap {
			t.Fatal(tc, r, e)
		}
		for i, want := range tc.roots {
			root := r.Hits[i].Parameter
			if root.Min.Cmp(want) > 0 || root.Max.Cmp(want) < 0 {
				t.Fatal("wrong quadrant conic branch", tc, i, root, want)
			}
		}
	}
	// A complete cap-plane segment inside the disk has no lateral hits.
	r, e := IntersectWrappedCylinderHorizontalLine(ctx, w, Vec3{Z: 2}, Vec3{X: 1}, -.25, .25, o)
	if e != nil || len(r.Hits) != 0 || len(r.Overlaps) != 1 {
		t.Fatal(r, e)
	}
	r, e = IntersectWrappedCylinderHorizontalLine(ctx, w, Vec3{Z: 2}, Vec3{X: 1}, 0, 0, o)
	if e != nil || len(r.Hits) != 1 || r.Hits[0].Parts != CylinderUpperCap {
		t.Fatal("singleton cap point", r, e)
	}
	bad := o
	bad.Surface.MaxOperations = 1
	r, e = IntersectWrappedCylinderHorizontalLine(ctx, w, Vec3{}, Vec3{X: 1}, -2, 2, bad)
	if e == nil || len(r.Hits) != 0 || len(r.Overlaps) != 0 {
		t.Fatal("budget", r, e)
	}
	_, e = IntersectWrappedCylinderHorizontalLine(ctx, w, Vec3{}, Vec3{X: 1, Z: 1}, -2, 2, o)
	if !errors.Is(e, ErrInvalid) {
		t.Fatal("nonhorizontal", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectWrappedCylinderHorizontalLine(canceled, w, Vec3{}, Vec3{X: 1}, -2, 2, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
