package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestRationalCylinderObliqueAndAxialLines(t *testing.T) {
	ctx := context.Background()
	w := queryCylinder(t, false)
	o := ToroidLineOptions{ParameterTolerance: 1e-12}
	for _, tc := range []struct {
		p, d     Vec3
		want     []*big.Rat
		parts    []CylinderBoundaryPart
		overlaps int
	}{
		{Vec3{}, Vec3{X: 1, Z: 3}, []*big.Rat{big.NewRat(-1, 3), big.NewRat(2, 3)}, []CylinderBoundaryPart{CylinderLowerCap, CylinderUpperCap}, 0},
		{Vec3{Y: .5}, Vec3{X: 7, Z: 16}, []*big.Rat{big.NewRat(-1, 16), big.NewRat(1, 8)}, []CylinderBoundaryPart{CylinderLowerCap, CylinderLateral | CylinderUpperCap}, 0},
		{Vec3{X: .875, Y: .5}, Vec3{Z: 3}, []*big.Rat{big.NewRat(-1, 3), big.NewRat(2, 3)}, []CylinderBoundaryPart{CylinderLowerCap | CylinderLateral, CylinderUpperCap | CylinderLateral}, 1},
		{Vec3{}, Vec3{X: 1, Y: 1, Z: 1}, []*big.Rat{big.NewRat(-5, 7), big.NewRat(5, 7)}, []CylinderBoundaryPart{CylinderLateral, CylinderLateral}, 0},
		{Vec3{Y: .5}, Vec3{X: -7, Z: -16}, []*big.Rat{big.NewRat(-1, 8), big.NewRat(1, 16)}, []CylinderBoundaryPart{CylinderLateral | CylinderUpperCap, CylinderLowerCap}, 0},
	} {
		r, e := IntersectWrappedCylinderLine(ctx, w, tc.p, tc.d, -2, 2, o)
		if e != nil || len(r.Hits) != len(tc.want) || len(r.Overlaps) != tc.overlaps {
			t.Fatal(tc, r, e)
		}
		for i, v := range tc.want {
			hit := r.Hits[i]
			if hit.Parameter.Min.Cmp(v) > 0 || hit.Parameter.Max.Cmp(v) < 0 || hit.Parts != tc.parts[i] {
				t.Fatal("root/parts", tc, i, hit, v)
			}
		}
	}
	// Entirely clipped above the top, even though the infinite side conic crosses.
	r, e := IntersectWrappedCylinderLine(ctx, w, Vec3{Z: 10}, Vec3{X: 1, Z: 1}, -2, 2, o)
	if e != nil || len(r.Hits) != 0 || len(r.Overlaps) != 0 {
		t.Fatal("height clipping", r, e)
	}
	// Singleton rim parameter stays a point, never an invented interval.
	r, e = IntersectWrappedCylinderLine(ctx, w, Vec3{X: 1}, Vec3{Z: 1}, 2, 2, o)
	if e != nil || len(r.Hits) != 1 || len(r.Overlaps) != 0 || r.Hits[0].Parts != (CylinderUpperCap|CylinderLateral) {
		t.Fatal("singleton rim", r, e)
	}
	bad := o
	bad.Surface.MaxOperations = 1
	r, e = IntersectWrappedCylinderLine(ctx, w, Vec3{}, Vec3{X: 1, Z: 1}, -2, 2, bad)
	if e == nil || len(r.Hits) != 0 || len(r.Overlaps) != 0 {
		t.Fatal("partial budget", r, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectWrappedCylinderLine(canceled, w, Vec3{}, Vec3{X: 1, Z: 1}, -2, 2, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
