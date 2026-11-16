package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"plainsolid/geom"
)

type parabolaTwoCrossingsSourceFixture struct {
	parabola [3]*big.Rat
	discs    [2]twoCircleStockProbe
	roots    [2][2]*big.Rat
	scale    *big.Rat
	order    [2]int
}

func parabolaTwoCrossingCirclesStaggered() parabolaTwoCrossingsSourceFixture {
	return parabolaTwoCrossingsSourceFixture{
		parabola: [3]*big.Rat{big.NewRat(3, 4), big.NewRat(-2, 1), big.NewRat(2, 1)},
		discs:    [2]twoCircleStockProbe{twoCircleProbe(112, 177, 32, 512), twoCircleProbe(400, 177, 32, 512)},
		roots:    [2][2]*big.Rat{{big.NewRat(7, 32), big.NewRat(9, 32)}, {big.NewRat(23, 32), big.NewRat(25, 32)}},
		scale:    big.NewRat(1225, 4096),
		order:    [2]int{0, 1},
	}
}

func parabolaTwoCrossingCirclesCoincidentProjection() parabolaTwoCrossingsSourceFixture {
	// p(1/4+t)=99/256-19t/12+25t²/12. Both disk projections are
	// [1/8,3/8]. The exact center distance is 91/256, greater than 1/4.
	// Circle intersections specified in disk center order, reversed from
	// the order in which the original parabola encounters their interiors.
	return parabolaTwoCrossingsSourceFixture{
		parabola: [3]*big.Rat{big.NewRat(701, 768), big.NewRat(-21, 8), big.NewRat(25, 12)},
		discs:    [2]twoCircleStockProbe{twoCircleProbe(64, 67, 32, 256), twoCircleProbe(64, 158, 32, 256)},
		roots:    [2][2]*big.Rat{{big.NewRat(1, 4), big.NewRat(37, 100)}, {big.NewRat(1, 8), big.NewRat(7, 40)}},
		scale:    big.NewRat(3675, 8192),
		order:    [2]int{1, 0},
	}
}

func parabolaTwoCrossingCirclesOverlappingProjections() parabolaTwoCrossingsSourceFixture {
	// The disks overlap in free projection on [21/32,89/128].
	// Their squared model-space center distance exceeds (23/64)².
	return parabolaTwoCrossingsSourceFixture{
		parabola: [3]*big.Rat{big.NewRat(171, 512), big.NewRat(119, 64), big.NewRat(-2, 1)},
		discs:    [2]twoCircleStockProbe{twoCircleProbe(48, 55, 41, 128), twoCircleProbe(2848, 2543, 160, 4096)},
		roots:    [2][2]*big.Rat{{big.NewRat(7, 128), big.NewRat(3, 8)}, {big.NewRat(89, 128), big.NewRat(47, 64)}},
		scale:    big.NewRat(1225, 512),
		order:    [2]int{0, 1},
	}
}

func (f parabolaTwoCrossingsSourceFixture) stocks(t *testing.T, axis int, perturb bool) [3]*geom.BSplineSurface {
	return parabolaCircleProductStock(t, axis, f.discs[:], f.parabola, f.scale, perturb)
}

func (f parabolaTwoCrossingsSourceFixture) ordinate(x *big.Rat) *big.Rat {
	p := new(big.Rat).Add(new(big.Rat).Mul(f.parabola[2], x), f.parabola[1])
	return p.Mul(p, x).Add(p, f.parabola[0])
}

func (f parabolaTwoCrossingsSourceFixture) region(x, y *big.Rat) int {
	region := 0
	for i, disc := range f.discs {
		dx, dy := new(big.Rat).Sub(x, disc.x), new(big.Rat).Sub(y, disc.y)
		rr := new(big.Rat).Add(dx.Mul(dx, dx), dy.Mul(dy, dy))
		if rr.Cmp(disc.radiusSquared) < 0 {
			region = 2 + 2*i
		}
	}
	if y.Cmp(f.ordinate(x)) > 0 {
		region++
	}
	return region
}

func TestParabolaTwoCrossingCirclesSource(t *testing.T) {
	ctx := context.Background()
	for layout, f := range []parabolaTwoCrossingsSourceFixture{parabolaTwoCrossingCirclesStaggered(), parabolaTwoCrossingCirclesCoincidentProjection(), parabolaTwoCrossingCirclesOverlappingProjections()} {
		for axis := range 2 {
			stocks := f.stocks(t, axis, false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					p := parabolaTwoCirclesPatches(t, stocks[1+order], stocks[2-order])
					got, err := geom.DiscoverSplineGraphParabolaTwoCrossingCircles(ctx, p[0], p[1], axis, geom.GraphPointContactOptions{})
					if err != nil {
						t.Fatal(err)
					}
					sign := 1 - 2*order
					want := [6]int{sign, -sign, -sign, sign, -sign, sign}
					if got.FreeAxis != axis || got.CircleOrder != f.order || got.RegionSides != want {
						t.Fatal("changed whole-domain signed coverage or attachment order", got)
					}
					for i, d := range f.discs {
						c := got.Circles[i]
						if c.Center[axis].Cmp(d.x) != 0 || c.Center[1-axis].Cmp(d.y) != 0 ||
							c.RadiusSquared.Cmp(d.radiusSquared) != 0 ||
							got.CircleProjections[i][0].Cmp(new(big.Rat).Sub(d.x, got.CircleRadii[i])) != 0 ||
							got.CircleProjections[i][1].Cmp(new(big.Rat).Add(d.x, got.CircleRadii[i])) != 0 {
							t.Fatal("changed original circle or its complete projection", i, got)
						}
						for j, node := range got.Nodes[i] {
							x, y := f.roots[i][j], f.ordinate(f.roots[i][j])
							if node.Parameters[axis].Cmp(x) != 0 || node.Parameters[1-axis].Cmp(y) != 0 ||
								node.Point[axis].Cmp(x) != 0 || node.Point[1-axis].Cmp(y) != 0 || node.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
								t.Fatal("changed original junction", node)
							}
							r := node.Rays[got.CircleCCWRays[i][j]]
							radial := [2]*big.Rat{}
							radial[axis], radial[1-axis] = new(big.Rat).Sub(x, d.x), new(big.Rat).Sub(y, d.y)
							if r[0].Cmp(new(big.Rat).Neg(radial[1])) != 0 || r[1].Cmp(radial[0]) != 0 {
								t.Fatal("changed original CCW arc ray", node)
							}
							slope := new(big.Rat).Add(f.parabola[1], new(big.Rat).Mul(x, new(big.Rat).Mul(big.NewRat(2, 1), f.parabola[2])))
							if node.Rays[0][axis].Cmp(big.NewRat(1, 1)) != 0 || node.Rays[0][1-axis].Cmp(slope) != 0 {
								t.Fatal("changed original forward parabola ray", node)
							}
							var seen [6]bool
							for k, a := range node.Rays {
								b := node.Rays[(k+1)%4]
								det := new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0]))
								if det.Sign() <= 0 || a[2].Sign() != 0 {
									t.Fatal("changed physical transverse order or roof jet", node)
								}
								w := [2]*big.Rat{}
								for d := range w {
									w[d] = new(big.Rat).Add(node.Parameters[d], new(big.Rat).Mul(big.NewRat(1, 100000), new(big.Rat).Add(a[d], b[d])))
								}
								region := node.SectorRegions[k]
								if seen[region] || f.region(w[axis], w[1-axis]) != region ||
									want[region] == want[node.SectorRegions[(k+1)%4]] {
									t.Fatal("changed independent physical sector", node)
								}
								seen[region] = true
							}
						}
					}
					for i, uv := range got.RegionWitnesses {
						left, err := p[0].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil {
							t.Fatal(err)
						}
						right, err := p[1].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil || left.Point[2].Cmp(right.Point[2]) != want[i] || f.region(uv[axis], uv[1-axis]) != i {
							t.Fatal("changed original proof witness", i, err)
						}
					}
				})
			}
		}
	}
}

func TestParabolaTwoCrossingCirclesSourceRefusals(t *testing.T) {
	ctx := context.Background()
	staggered := parabolaTwoCrossingCirclesStaggered()
	for _, tc := range []struct {
		name      string
		disc      twoCircleStockProbe
		perturbed bool
		options   geom.GraphPointContactOptions
		want      error
	}{
		{"unmatched tensor coefficient", staggered.discs[1], true, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"small operation budget", staggered.discs[1], false, geom.GraphPointContactOptions{MaxOperations: 1}, geom.ErrConvergence},
		{"small cell budget", staggered.discs[1], false, geom.GraphPointContactOptions{MaxCells: 1}, geom.ErrConvergence},
		{"second disk disjoint from parabola", twoCircleProbe(12, 13, 1, 16), false, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"double root", twoCircleProbe(8, 5, 1, 16), false, geom.GraphPointContactOptions{}, geom.ErrDegenerate},
		{"irrational crossings", twoCircleProbe(26, 14, 2, 32), false, geom.GraphPointContactOptions{}, geom.ErrUnrepresentable},
		{"disk-disk touch", twoCircleProbe(176, 177, 32, 512), false, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"disk-disk overlap", twoCircleProbe(175, 177, 32, 512), false, geom.GraphPointContactOptions{}, geom.ErrConvergence},
		{"stock touch", twoCircleProbe(31, 23, 1, 32), false, geom.GraphPointContactOptions{}, geom.ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := staggered
			f.discs[1] = tc.disc
			stocks := f.stocks(t, 0, tc.perturbed)
			p := parabolaTwoCirclesPatches(t, stocks[1], stocks[2])
			got, err := geom.DiscoverSplineGraphParabolaTwoCrossingCircles(ctx, p[0], p[1], 0, tc.options)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatal("published unresolved source arrangement", got, err, "want", tc.want)
			}
		})
	}
	t.Run("small coefficient budget", func(t *testing.T) {
		stocks := parabolaTwoCrossingCirclesCoincidentProjection().stocks(t, 0, false)
		p := parabolaTwoCirclesPatches(t, stocks[1], stocks[2])
		got, err := geom.DiscoverSplineGraphParabolaTwoCrossingCircles(ctx, p[0], p[1], 0, geom.GraphPointContactOptions{MaxCoefficientBits: 64})
		if got != nil || !errors.Is(err, geom.ErrConvergence) {
			t.Fatal("published source arrangement before exact coefficient budget passed", got, err)
		}
	})
	t.Run("four simple roots on one original circle", func(t *testing.T) {
		// On the central circle of radius 1/4 the four contact stations
		// are x=1/2±3/20 and x=1/2±1/5.
		parabola := [3]*big.Rat{big.NewRat(231, 280), big.NewRat(-20, 7), big.NewRat(20, 7)}
		discs := [2]twoCircleStockProbe{twoCircleProbe(4, 3, 2, 8), twoCircleProbe(28, 24, 1, 32)}
		stocks := parabolaCircleProductStock(t, 0, discs[:], parabola, big.NewRat(8575, 32768), false)
		p := parabolaTwoCirclesPatches(t, stocks[1], stocks[2])
		got, err := geom.DiscoverSplineGraphParabolaTwoCrossingCircles(ctx, p[0], p[1], 0, geom.GraphPointContactOptions{})
		if got != nil || !errors.Is(err, geom.ErrConvergence) {
			t.Fatal("published more than two crossings on a circle", got, err)
		}
	})
	stocks := staggered.stocks(t, 0, false)
	p := parabolaTwoCirclesPatches(t, stocks[1], stocks[2])
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	got, err := geom.DiscoverSplineGraphParabolaTwoCrossingCircles(cancelled, p[0], p[1], 0, geom.GraphPointContactOptions{})
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
}
