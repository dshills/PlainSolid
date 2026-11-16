package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
)

type twoCircleStockProbe struct {
	x, y, radiusSquared *big.Rat
}

func twoCircleProbe(x, y, r, denominator int64) twoCircleStockProbe {
	return twoCircleStockProbe{big.NewRat(x, denominator), big.NewRat(y, denominator), big.NewRat(r*r, denominator*denominator)}
}

func twoCirclePrimaryStock() [2]twoCircleStockProbe {
	return [2]twoCircleStockProbe{twoCircleProbe(2, 6, 1, 8), twoCircleProbe(12, 12, 1, 16)}
}

func parabolaTwoCirclesStock(t *testing.T, axis int, circles [2]twoCircleStockProbe, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	// Author a tensor spline from 1225/8192*(v-1/4-u²/8)*Q0*Q1.
	// 1225 cancels the odd factors of both degree-eight Bernstein bases.
	power := map[[2]int]*big.Rat{{0, 1}: big.NewRat(1225, 8192), {0, 0}: big.NewRat(-1225, 32768), {2, 0}: big.NewRat(-1225, 65536)}
	for _, disc := range circles {
		constant := new(big.Rat).Add(new(big.Rat).Mul(disc.x, disc.x), new(big.Rat).Mul(disc.y, disc.y))
		constant.Sub(constant, disc.radiusSquared)
		terms := map[[2]int]*big.Rat{
			{0, 0}: constant, {2, 0}: big.NewRat(1, 1), {0, 2}: big.NewRat(1, 1),
			{1, 0}: new(big.Rat).Mul(big.NewRat(-2, 1), disc.x),
			{0, 1}: new(big.Rat).Mul(big.NewRat(-2, 1), disc.y),
		}
		product := make(map[[2]int]*big.Rat)
		for ij, a := range power {
			for kl, b := range terms {
				p := [2]int{ij[0] + kl[0], ij[1] + kl[1]}
				if product[p] == nil {
					product[p] = new(big.Rat)
				}
				product[p].Add(product[p], new(big.Rat).Mul(a, b))
			}
		}
		power = product
	}
	basis := func(i, j int) *big.Rat {
		return new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(8, int64(j)))
	}
	var surfaces [3]*geom.BSplineSurface
	knots := append(make([]float64, 9), 1, 1, 1, 1, 1, 1, 1, 1, 1)
	for body := range surfaces {
		var poles []geom.Vec3
		for i := range 9 {
			for j := range 9 {
				z := 0.0
				if body > 0 {
					z = 2
				}
				if body == 2 {
					exact := big.NewRat(2, 1)
					for uv, x := range power {
						if axis == 1 {
							uv[0], uv[1] = uv[1], uv[0]
						}
						term := new(big.Rat).Mul(basis(i, uv[0]), basis(j, uv[1]))
						exact.Add(exact, term.Mul(term, x))
					}
					var ok bool
					z, ok = exact.Float64()
					if !ok {
						t.Fatal("original tensor control is not dyadic", exact)
					}
					if perturb && i == 4 && j == 5 {
						z = math.Nextafter(z, math.Inf(1))
					}
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 8, Y: float64(j) / 8, Z: z})
			}
		}
		var err error
		surfaces[body], err = geom.NewBSplineSurface(8, 8, 9, 9, poles, knots, knots, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return surfaces
}

func parabolaTwoCirclesPatches(t *testing.T, a, b *geom.BSplineSurface) [2]*geom.SurfacePatch {
	t.Helper()
	var out [2]*geom.SurfacePatch
	for i, s := range [2]*geom.BSplineSurface{a, b} {
		var err error
		out[i], err = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestParabolaTwoCirclesSourceDiscovery(t *testing.T) {
	ctx := context.Background()
	for layout, discs := range [2][2]twoCircleStockProbe{twoCirclePrimaryStock(),
		{twoCircleProbe(4, 2, 1, 16), twoCircleProbe(24, 4, 1, 32)}} {
		for axis := range 2 {
			src := parabolaTwoCirclesStock(t, axis, discs, false)
			for order := range 2 {
				p := parabolaTwoCirclesPatches(t, src[1+order], src[2-order])
				r, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, p[0], p[1], axis, geom.GraphPointContactOptions{})
				if err != nil {
					t.Fatal(layout, axis, order, err)
				}
				sides := [4]int{1, -1, 1, 1}
				if layout == 1 {
					sides[2], sides[3] = -1, -1
				}
				if order == 1 {
					for i := range sides {
						sides[i] = -sides[i]
					}
				}
				if r.CirclesAbove != (layout == 0) || r.RegionSides != sides || !r.Graph.ProvenInjective {
					t.Fatal("incorrect signed region nesting", r)
				}
				for i, d := range discs {
					if r.Circles[i].Center[axis].Cmp(d.x) != 0 || r.Circles[i].Center[1-axis].Cmp(d.y) != 0 ||
						r.Circles[i].RadiusSquared.Cmp(d.radiusSquared) != 0 {
						t.Fatal("factor recovery changed the ordered exact circle", i, r.Circles[i])
					}
				}
				for i, ordin := range [3]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 4), big.NewRat(3, 8)} {
					if r.ParabolaControls[i][axis].Cmp(big.NewRat(int64(i), 2)) != 0 ||
						r.ParabolaControls[i][1-axis].Cmp(ordin) != 0 {
						t.Fatal("factor recovery changed the open parabola", r.ParabolaControls)
					}
				}
			}
		}
	}
}

func TestParabolaTwoCirclesSourceRefusals(t *testing.T) {
	ctx := context.Background()
	primary := twoCirclePrimaryStock()
	for _, trial := range []struct {
		name    string
		discs   [2]twoCircleStockProbe
		changed bool
		axis    int
		work    int
	}{
		{name: "changed coefficient", discs: primary, changed: true},
		{name: "wrong free axis", discs: primary, axis: 1},
		{name: "low work budget", discs: primary, work: 1},
		{name: "touching circles", discs: [2]twoCircleStockProbe{primary[0], twoCircleProbe(7, 12, 1, 16)}},
		{name: "nested circles", discs: [2]twoCircleStockProbe{primary[0], twoCircleProbe(4, 12, 1, 16)}},
		{name: "different center ordinates", discs: [2]twoCircleStockProbe{primary[0], twoCircleProbe(12, 11, 1, 16)}},
		{name: "intersecting parabola", discs: [2]twoCircleStockProbe{twoCircleProbe(2, 3, 1, 8), twoCircleProbe(12, 6, 1, 16)}},
		{name: "stock-tangent circle", discs: [2]twoCircleStockProbe{twoCircleProbe(2, 7, 1, 8), twoCircleProbe(12, 14, 1, 16)}},
	} {
		t.Run(trial.name, func(t *testing.T) {
			src := parabolaTwoCirclesStock(t, 0, trial.discs, trial.changed)
			p := parabolaTwoCirclesPatches(t, src[1], src[2])
			r, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, p[0], p[1], trial.axis, geom.GraphPointContactOptions{MaxOperations: trial.work})
			if r != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("unsupported arrangement published a geometry proof", r, err)
			}
		})
	}
}
