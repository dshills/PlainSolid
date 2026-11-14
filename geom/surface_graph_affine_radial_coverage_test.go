package geom_test

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAffineRadialOriginalTransport(t *testing.T) {
	ctx := context.Background()
	base, _ := coverageCircleSources(t)
	build := func(swapped bool, upper, z *big.Rat) *geom.SurfacePatch {
		var net [][4]*big.Rat
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				x := big.NewRat(-1, 8)
				if i == 1 {
					x = new(big.Rat).Set(upper)
				}
				y := big.NewRat(-1, 8)
				if j == 1 {
					y = new(big.Rat).Set(upper)
				}
				if swapped {
					x, y = y, new(big.Rat).Sub(big.NewRat(1, 1), x)
				}
				net = append(net, [4]*big.Rat{x, y, z, big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, net, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	radii := []*big.Rat{big.NewRat(1, 4)}
	for _, swap := range []bool{false, true} {
		original := build(swap, big.NewRat(9, 8), new(big.Rat))
		proof, e := geom.CertifyAffineRadialGraphContours(ctx, original, base[1], center, radii, geom.GraphPointContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		normalized := proof.CanonicalSupports()
		same, e := geom.CompareExactSurfaces(ctx, normalized[0], base[0], geom.ExactSurfaceComparisonOptions{})
		if e != nil || !same.Equal || !same.Complete {
			t.Fatal("canonical restriction", same, e)
		}
		bounds, exact, work, e := proof.OriginalBox(ctx, 0, geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}, geom.SurfacePatchOptions{})
		if e != nil || exact || work == 0 {
			t.Fatal("nonbinary original cuts", bounds, exact, work, e)
		}
		if new(big.Rat).SetFloat64(bounds.UMin).Cmp(big.NewRat(1, 10)) > 0 || new(big.Rat).SetFloat64(bounds.UMax).Cmp(big.NewRat(9, 10)) < 0 {
			t.Fatal("inward box")
		}
		axis, e := proof.OriginalAxis(0, 0)
		want := 0
		if swap {
			want = 1
		}
		if e != nil || axis != want {
			t.Fatal(axis, e)
		}
		if _, e := geom.CertifyAffineRadialGraphContours(ctx, original, base[1], center, radii, geom.GraphPointContactOptions{MaxOperations: proof.ExactOperations()}); e != nil {
			t.Fatal("exact budget", e)
		}
		if got, e := geom.CertifyAffineRadialGraphContours(ctx, original, base[1], center, radii, geom.GraphPointContactOptions{MaxOperations: proof.ExactOperations() - 1}); got != nil || e == nil {
			t.Fatal("budget", e)
		}
	}
	for _, a := range []*geom.SurfacePatch{build(false, big.NewRat(7, 8), new(big.Rat)), build(false, big.NewRat(9, 8), big.NewRat(1, 1024))} {
		if got, e := geom.CertifyAffineRadialGraphContours(ctx, a, base[1], center, radii, geom.GraphPointContactOptions{}); got != nil || e == nil {
			t.Fatal("partial footprint/height mutation", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := geom.CertifyAffineRadialGraphContours(cancelled, base[0], base[1], center, radii, geom.GraphPointContactOptions{}); got != nil || !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
