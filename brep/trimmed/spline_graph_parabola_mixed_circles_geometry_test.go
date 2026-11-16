package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func mixedCirclePrimaryStock(side int) [2]twoCircleStockProbe {
	cross := twoCircleProbe(64, 49, 32, 128)
	if side == 0 {
		return [2]twoCircleStockProbe{cross, twoCircleProbe(14, 2, 1, 16)}
	}
	return [2]twoCircleStockProbe{twoCircleProbe(2, 12, 1, 16), cross}
}

func parabolaMixedCirclesStock(t *testing.T, axis int, circles [2]twoCircleStockProbe, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	// Author a tensor spline from 1225/8192*(v-3/8-(u-1/2)²/8)*Q0*Q1.
	// 1225 cancels the odd factors of both degree-eight Bernstein bases.
	return parabolaCircleProductStock(t, axis, circles, [3]*big.Rat{big.NewRat(13, 32), big.NewRat(-1, 8), big.NewRat(1, 8)},
		big.NewRat(1225, 8192), perturb)
}

func parabolaCircleProductStock(t *testing.T, axis int, circles [2]twoCircleStockProbe, parabola [3]*big.Rat,
	scale *big.Rat, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	power := map[[2]int]*big.Rat{{0, 1}: scale}
	for i, coefficient := range parabola {
		power[[2]int{i, 0}] = new(big.Rat).Mul(new(big.Rat).Neg(scale), coefficient)
	}
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

func TestParabolaMixedCirclesSource(t *testing.T) {
	ctx := context.Background()
	twoContacts := twoCircleProbe(64, 49, 32, 128)
	for layout, fixture := range []struct {
		discs [2]twoCircleStockProbe
		side  int
		cross int
	}{
		{mixedCirclePrimaryStock(0), 0, 0},
		{mixedCirclePrimaryStock(1), 1, 1},
		// The isolated disk overlaps the global ordinate hull of the
		// parabola, so the proof needs the complete local quartic.
		{[2]twoCircleStockProbe{twoContacts, twoCircleProbe(112, 48, 1, 128)}, 0, 0},
		{[2]twoCircleStockProbe{twoContacts, twoCircleProbe(112, 52, 1, 128)}, 1, 0},
	} {
		for axis := range 2 {
			stock := parabolaMixedCirclesStock(t, axis, fixture.discs, false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					src := parabolaTwoCirclesPatches(t, stock[1+order], stock[2-order])
					got, err := geom.DiscoverSplineGraphParabolaMixedCircles(ctx, src[0], src[1], axis, geom.GraphPointContactOptions{})
					if err != nil {
						t.Fatal(err)
					}
					sign := 1 - 2*order
					want := [5]int{sign, -sign, -sign, sign, sign * (2*fixture.side - 1)}
					if got.Crossing.FreeAxis != axis || got.CrossingCircle != fixture.cross || got.SeparatedRegion != fixture.side ||
						got.RegionSides != want || got.ExactOperations <= got.Crossing.ExactOperations {
						t.Fatal("changed full signed coverage or circle role", got)
					}
					for i, d := range fixture.discs {
						if got.Circles[i].Center[axis].Cmp(d.x) != 0 || got.Circles[i].Center[1-axis].Cmp(d.y) != 0 ||
							got.Circles[i].RadiusSquared.Cmp(d.radiusSquared) != 0 {
							t.Fatal("changed an original exact circle owner", i)
						}
					}
					for i, node := range got.Crossing.Nodes {
						if node.Parameters[axis].Cmp(big.NewRat(int64(2*i+1), 4)) != 0 ||
							node.Parameters[1-axis].Cmp(twoContacts.y) != 0 || node.Point[2].Cmp(big.NewRat(2, 1)) != 0 ||
							node.Rays[0][1-axis].Cmp(big.NewRat(int64(2*i-1), 16)) != 0 || node.Rays[1][axis].Sign() != 0 {
							t.Fatal("changed an original transverse node or ray", node)
						}
						for j, sector := range node.SectorRegions {
							next := node.SectorRegions[(j+1)%4]
							if got.RegionSides[sector] == got.RegionSides[next] {
								t.Fatal("changed the signed alternating physical sectors", node)
							}
						}
					}
					for i, v := range got.SeparatedExtrema {
						for owner, direction := range v.Tangents {
							expect := [4][2]int64{{0, 1}, {-1, 0}, {0, -1}, {1, 0}}[i]
							for dim, component := range expect {
								if direction[dim].Cmp(big.NewRat(component, 1)) != 0 || v.Parameters[dim].Cmp(v.Point[dim]) != 0 {
									t.Fatal("changed a separated quarter-edge owner tangent", owner, v)
								}
							}
							if direction[2].Sign() != 0 || v.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
								t.Fatal("changed separated original trace plane", owner, v)
							}
						}
					}
					for i, uv := range got.RegionWitnesses {
						left, err := src[0].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil {
							t.Fatal(err)
						}
						right, err := src[1].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil || left.Point[2].Cmp(right.Point[2]) != want[i] {
							t.Fatal("region witness changed side on an original roof", i, err)
						}
					}
				})
			}
		}
	}
}

func TestParabolaMixedCirclesSourceRefusals(t *testing.T) {
	ctx := context.Background()
	main := mixedCirclePrimaryStock(0)
	cross := main[0]
	for _, fixture := range []struct {
		name       string
		discs      [2]twoCircleStockProbe
		changed    bool
		axis, work int
	}{
		{"changed original coefficient", main, true, 0, 0},
		{"wrong source axis", main, false, 1, 0},
		{"short proof", main, false, 0, 1},
		{"two strictly separated circles", twoCirclePrimaryStock(), false, 0, 0},
		{"extra parabola crossings", [2]twoCircleStockProbe{cross, twoCircleProbe(448, 201, 8, 512)}, false, 0, 0},
		{"disk-disk touching", [2]twoCircleStockProbe{cross, twoCircleProbe(104, 49, 8, 128)}, false, 0, 0},
		{"disk-disk overlap", [2]twoCircleStockProbe{cross, twoCircleProbe(100, 49, 8, 128)}, false, 0, 0},
		{"circle stock contact", [2]twoCircleStockProbe{cross, twoCircleProbe(15, 13, 1, 16)}, false, 0, 0},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			stock := parabolaMixedCirclesStock(t, 0, fixture.discs, fixture.changed)
			p := parabolaTwoCirclesPatches(t, stock[1], stock[2])
			out, err := geom.DiscoverSplineGraphParabolaMixedCircles(ctx, p[0], p[1], fixture.axis, geom.GraphPointContactOptions{MaxOperations: fixture.work})
			if out != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("unsupported mixed circle source was published", out, err)
			}
		})
	}
}
