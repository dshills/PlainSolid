package intersect_test

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func TestRadialPhysicalRectangleCoverage(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	build := func(x0, x1, y0, y1 *big.Rat, curved bool) *geom.ExactBezierSurface {
		square := func(lo, hi, c *big.Rat) [3]*big.Rat {
			a := new(big.Rat).Sub(lo, c)
			b := new(big.Rat).Sub(hi, c)
			return [3]*big.Rat{new(big.Rat).Mul(a, a), new(big.Rat).Mul(a, b), new(big.Rat).Mul(b, b)}
		}
		xx, yy := square(x0, x1, big.NewRat(3, 1)), square(y0, y1, big.NewRat(1, 2))
		var net [][4]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				x := new(big.Rat).Add(x0, new(big.Rat).Mul(new(big.Rat).Sub(x1, x0), big.NewRat(int64(i), 2)))
				y := new(big.Rat).Add(y0, new(big.Rat).Mul(new(big.Rat).Sub(y1, y0), big.NewRat(int64(j), 2)))
				z := new(big.Rat)
				if curved {
					z.Sub(new(big.Rat).Add(xx[i], yy[j]), big.NewRat(1, 16))
				}
				net = append(net, [4]*big.Rat{x, y, z, big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 2, 2, net, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s, e := geom.NewExactBezierSurface(p)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a := intersect.Patch{Surface: build(big.NewRat(1, 1), big.NewRat(5, 1), big.NewRat(-2, 1), big.NewRat(3, 1), false), Max: geom.Vec2{X: 1, Y: 1}}
	b := intersect.Patch{Surface: build(big.NewRat(2, 1), big.NewRat(4, 1), big.NewRat(-1, 1), big.NewRat(2, 1), true), Max: geom.Vec2{X: 1, Y: 1}}
	center := [2]*big.Rat{big.NewRat(3, 1), big.NewRat(1, 2)}
	radii := []*big.Rat{big.NewRat(1, 4)}
	seeds, e := intersect.DiscoverOverlappingAffineRadialGraphContourTubes(ctx, a, b, center, radii, geom.SurfacePatchOptions{}, geom.RadialTubeCoverageOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := intersect.SplineTubeCoverageOptions{RadialOverlap: &intersect.RadialOverlapCoverageOptions{AffineCharts: true, Center: center, Radii: radii}}
	report, e := intersect.CertifySplineTubeCoverage(ctx, a, b, seeds, o)
	if e != nil || !report.CoversAllContacts || report.RadialOverlap == nil {
		t.Fatal(e)
	}
	t.Log("physical circle cover", len(seeds.Attempts), report.RadialOverlap.ExactOperations)
	missing := seeds
	missing.Attempts = missing.Attempts[:len(missing.Attempts)/2]
	if got, e := intersect.CertifySplineTubeCoverage(ctx, a, b, missing, o); e == nil || got.CoversAllContacts || got.RadialOverlap != nil {
		t.Fatal("missing original occurrence", e)
	}
	if got, e := intersect.DiscoverOverlappingAffineRadialGraphContourTubes(ctx, a, b, center, radii, geom.SurfacePatchOptions{}, geom.RadialTubeCoverageOptions{MaxTubes: 1}); e == nil || len(got.Attempts) != 0 {
		t.Fatal("atomic proposals", e)
	}
}
