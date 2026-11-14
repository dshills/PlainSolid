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
	for _, a := range []*geom.SurfacePatch{build(false, big.NewRat(5, 8), new(big.Rat)), build(false, big.NewRat(9, 8), big.NewRat(1, 1024))} {
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

func TestAffineRadialPhysicalRectangle(t *testing.T) {
	ctx := context.Background()
	// Exact nonbinary rectangle [2,4] x [-1/3,8/3], unequal source
	// footprints and arbitrary original chart scales. Heights are authored
	// independently from physical Bernstein squares, not normalized circles.
	center := [2]*big.Rat{big.NewRat(3, 1), big.NewRat(7, 6)}
	radius := big.NewRat(1, 4)
	square := func(lo, hi, c *big.Rat) [3]*big.Rat {
		a := new(big.Rat).Sub(lo, c)
		b := new(big.Rat).Sub(hi, c)
		return [3]*big.Rat{new(big.Rat).Mul(a, a), new(big.Rat).Mul(a, b), new(big.Rat).Mul(b, b)}
	}
	build := func(x0, x1, y0, y1 *big.Rat, curved, wrong bool, r *big.Rat) *geom.SurfacePatch {
		xx, yy := square(x0, x1, center[0]), square(y0, y1, center[1])
		if wrong {
			xx, yy = square(new(big.Rat), big.NewRat(1, 1), big.NewRat(1, 2)), square(new(big.Rat), big.NewRat(1, 1), big.NewRat(1, 2))
		}
		var controls [][4]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				x := new(big.Rat).Add(x0, new(big.Rat).Mul(new(big.Rat).Sub(x1, x0), big.NewRat(int64(i), 2)))
				y := new(big.Rat).Add(y0, new(big.Rat).Mul(new(big.Rat).Sub(y1, y0), big.NewRat(int64(j), 2)))
				z := new(big.Rat)
				if curved {
					z.Sub(new(big.Rat).Add(xx[i], yy[j]), new(big.Rat).Mul(r, r))
				}
				controls = append(controls, [4]*big.Rat{x, y, z, big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 2, 2, controls, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	a := build(big.NewRat(1, 1), big.NewRat(5, 1), big.NewRat(-2, 1), big.NewRat(3, 1), false, false, radius)
	b := build(big.NewRat(2, 1), big.NewRat(4, 1), big.NewRat(-1, 3), big.NewRat(8, 3), true, false, radius)
	proof, e := geom.CertifyAffineRadialGraphContours(ctx, a, b, center, []*big.Rat{radius}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rect := proof.PhysicalRectangle()
	if rect[0][0].Cmp(big.NewRat(2, 1)) != 0 || rect[1][0].Cmp(big.NewRat(-1, 3)) != 0 || rect[1][1].Cmp(big.NewRat(8, 3)) != 0 {
		t.Fatal(rect)
	}
	rect[0][0].SetInt64(999)
	if proof.PhysicalRectangle()[0][0].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("mutable physical rectangle")
	}
	if _, e := geom.CertifyAffineRadialGraphContours(ctx, a, b, center, []*big.Rat{radius}, geom.GraphPointContactOptions{MaxOperations: proof.ExactOperations()}); e != nil {
		t.Fatal("exact budget", e)
	}
	for _, bad := range []struct {
		b *geom.SurfacePatch
		r *big.Rat
	}{
		{build(big.NewRat(2, 1), big.NewRat(4, 1), big.NewRat(-1, 3), big.NewRat(8, 3), true, true, radius), radius},
		{build(big.NewRat(2, 1), big.NewRat(4, 1), big.NewRat(-1, 3), big.NewRat(8, 3), true, false, big.NewRat(5, 4)), big.NewRat(5, 4)},
	} {
		if got, e := geom.CertifyAffineRadialGraphContours(ctx, a, bad.b, center, []*big.Rat{bad.r}, geom.GraphPointContactOptions{}); got != nil || e == nil {
			t.Fatal("normalized circle or outside radius accepted", e)
		}
	}
}

func TestAffineRadialEmptyCertificate(t *testing.T) {
	var empty geom.AffineRadialContours
	if empty.PhysicalRectangle()[0][0] != nil {
		t.Fatal("empty bounds")
	}
	if _, e := empty.OriginalAxis(0, 0); e == nil {
		t.Fatal("empty axis")
	}
	if _, _, _, e := empty.OriginalBox(context.Background(), 0, geom.ParameterRectangle{UMax: 1, VMax: 1}, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("empty box")
	}
}
