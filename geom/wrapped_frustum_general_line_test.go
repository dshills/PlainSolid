package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestGeneralRationalFrustumLineIdentities(t *testing.T) {
	ctx := context.Background()
	w, _, e := DraftWrappedFrustum(ctx, queryCylinder(t, false), 0, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := ToroidLineOptions{ParameterTolerance: 1e-11}
	for _, tc := range []struct {
		p, d     Vec3
		lo, hi   float64
		roots    []*big.Rat
		overlaps int
	}{
		{Vec3{Y: .75, Z: .5}, Vec3{X: 1}, -3, 3, []*big.Rat{big.NewRat(-21, 16), big.NewRat(21, 16)}, 0},
		{Vec3{X: 1.3125, Y: .75}, Vec3{Z: 1}, -2, 3, []*big.Rat{big.NewRat(1, 2), big.NewRat(2, 1)}, 0},
		{Vec3{X: .875, Y: .5, Z: -1}, Vec3{X: .875, Y: .5, Z: 3}, -1, 2, []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, 1},
		{Vec3{X: 1, Z: -1}, Vec3{X: 1, Z: 3}, -1, 2, []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, 1},
		{Vec3{Z: 2}, Vec3{X: 1, Y: 1}, -3, 3, []*big.Rat{big.NewRat(-10, 7), big.NewRat(10, 7)}, 1},
	} {
		r, e := IntersectWrappedFrustumLine(ctx, w, tc.p, tc.d, tc.lo, tc.hi, o)
		if e != nil || len(r.Hits) != len(tc.roots) || len(r.Overlaps) != tc.overlaps {
			t.Fatal(tc, r, e)
		}
		for i, want := range tc.roots {
			root := r.Hits[i].Parameter
			if root.Min.Cmp(want) > 0 || root.Max.Cmp(want) < 0 {
				t.Fatal("root", tc, i, root, want)
			}
		}
		if tc.overlaps == 1 && tc.d.Z != 0 {
			if r.Hits[0].Parts != (CylinderLowerCap|CylinderLateral) || r.Hits[1].Parts != (CylinderUpperCap|CylinderLateral) {
				t.Fatal("generator rim identities", r)
			}
		}
	}
	limited := o
	limited.Surface.MaxOperations = 1
	denied, e := IntersectWrappedFrustumLine(ctx, w, Vec3{}, Vec3{X: 1, Z: 1}, -3, 3, limited)
	if e == nil || len(denied.Hits) != 0 || len(denied.Overlaps) != 0 {
		t.Fatal("partial limit result", e)
	}

	// Circular API remains numerically and topologically compatible.
	circular, _, e := DraftWrappedFrustum(ctx, queryCylinder(t, true), 0, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := IntersectWrappedFrustumLine(ctx, circular, Vec3{Y: .5}, Vec3{X: 1, Z: .25}, -3, 3, o)
	if e != nil {
		t.Fatal(e)
	}
	b, e := IntersectWrappedCircularFrustumLine(ctx, circular, Vec3{Y: .5}, Vec3{X: 1, Z: .25}, -3, 3, o)
	if e != nil || len(a.Hits) != len(b.Hits) {
		t.Fatal(a, b, e)
	}
	for i := range a.Hits {
		if a.Hits[i].Parameter.Min.Cmp(b.Hits[i].Parameter.Max) > 0 || a.Hits[i].Parameter.Max.Cmp(b.Hits[i].Parameter.Min) < 0 {
			t.Fatal("circle differential")
		}
	}
}

func TestCircularCylinderCoincidentEndpointParts(t *testing.T) {
	r, e := IntersectWrappedCircularCylinderLine(context.Background(), queryCylinder(t, true), Vec3{X: 1}, Vec3{Z: 1}, -2, 3, ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(r.Hits) != 2 || len(r.Overlaps) != 1 {
		t.Fatal(r, e)
	}
	if r.Hits[0].Parts != (CylinderLateral|CylinderLowerCap) || r.Hits[1].Parts != (CylinderLateral|CylinderUpperCap) {
		t.Fatal("coincident endpoint ancestry", r)
	}
}
