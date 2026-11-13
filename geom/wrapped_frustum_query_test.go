package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactFrustumPointAndLineQueries(t *testing.T) {
	ctx := context.Background()
	w, _, e := DraftWrappedFrustum(ctx, queryCylinder(t, true), 0, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Radius is 1+(z+1)/3, from1 atz=-1 to2 atz=2.
	for _, tc := range []struct {
		p    Vec3
		want CylinderLocation
	}{{Vec3{}, CylinderInside}, {Vec3{X: 1, Z: -1}, CylinderBoundary}, {Vec3{X: 1.5, Z: .5}, CylinderBoundary}, {Vec3{X: 1.75, Z: .5}, CylinderOutside}, {Vec3{X: 1.75, Z: 2}, CylinderBoundary}} {
		got, e := LocateWrappedFrustum(ctx, w, tc.p, SurfacePatchOptions{})
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	o := ToroidLineOptions{ParameterTolerance: 1e-11}
	for _, tc := range []struct {
		p, d    Vec3
		roots   []*big.Rat
		parts   []CylinderBoundaryPart
		overlap int
	}{
		{Vec3{Z: .5}, Vec3{X: 1}, []*big.Rat{big.NewRat(-3, 2), big.NewRat(3, 2)}, []CylinderBoundaryPart{CylinderLateral, CylinderLateral}, 0},
		{Vec3{}, Vec3{Z: 1}, []*big.Rat{big.NewRat(-1, 1), big.NewRat(2, 1)}, []CylinderBoundaryPart{CylinderLowerCap, CylinderUpperCap}, 0},
		{Vec3{X: 1, Z: -1}, Vec3{X: 1, Z: 3}, []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, []CylinderBoundaryPart{CylinderLowerCap | CylinderLateral, CylinderUpperCap | CylinderLateral}, 1},
		{Vec3{Z: 2}, Vec3{X: 1}, []*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}, []CylinderBoundaryPart{CylinderLateral | CylinderUpperCap, CylinderLateral | CylinderUpperCap}, 1},
		{Vec3{X: 1.5, Z: .5}, Vec3{Y: 1}, []*big.Rat{big.NewRat(0, 1)}, []CylinderBoundaryPart{CylinderLateral}, 0},
	} {
		r, e := IntersectWrappedCircularFrustumLine(ctx, w, tc.p, tc.d, -3, 3, o)
		if e != nil || len(r.Hits) != len(tc.roots) || len(r.Overlaps) != tc.overlap {
			t.Fatal(tc, r, e)
		}
		for i, want := range tc.roots {
			h := r.Hits[i]
			if h.Parameter.Min.Cmp(want) > 0 || h.Parameter.Max.Cmp(want) < 0 || h.Parts != tc.parts[i] {
				t.Fatal("root parts", tc, i, h, want)
			}
		}
	}
	// The noncircular classification uses the conic, not a circle approximation.
	rational, _, e := DraftWrappedFrustum(ctx, queryCylinder(t, false), 0, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	got, e := LocateWrappedFrustum(ctx, rational, Vec3{X: 1.03125, Y: 1.03125, Z: .5}, SurfacePatchOptions{})
	if e != nil || got != CylinderInside {
		t.Fatal(got, e)
	}
	_, e = IntersectWrappedCircularFrustumLine(ctx, rational, Vec3{}, Vec3{X: 1}, -3, 3, o)
	if !errors.Is(e, ErrConvergence) {
		t.Fatal("noncircular line silently approximated", e)
	}
	bad := o
	bad.Surface.MaxOperations = 1
	r, e := IntersectWrappedCircularFrustumLine(ctx, w, Vec3{}, Vec3{X: 1}, -3, 3, bad)
	if e == nil || len(r.Hits) != 0 || len(r.Overlaps) != 0 {
		t.Fatal("budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectWrappedCircularFrustumLine(canceled, w, Vec3{}, Vec3{X: 1}, -3, 3, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
