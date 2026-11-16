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

type obliqueCircleFixture struct {
	station, ordinate, slope, normal, shift, radiusSquared *big.Rat
}

func originalObliqueCircleFixture(side int) obliqueCircleFixture {
	return obliqueCircleFixture{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(3, 4), big.NewRat(int64(2*side-1), 16), new(big.Rat), big.NewRat(25, 4096)}
}

func parabolaCircleObliqueStock(t *testing.T, axis int, f obliqueCircleFixture, changed bool) [3]*geom.BSplineSurface {
	t.Helper()
	// Independently expand 9/16*(y-p(x))*Q in the original stock chart.
	// p=y0+m*(x-x0)+(x-x0)^2/8 and the circle center is
	// (x0-m*delta+shift,y0+delta).
	k := big.NewRat(1, 8)
	constant := new(big.Rat).Mul(f.station, f.station)
	constant.Mul(constant, k).Sub(constant, new(big.Rat).Mul(f.station, f.slope)).Add(constant, f.ordinate)
	linear := new(big.Rat).Sub(f.slope, new(big.Rat).Mul(f.station, big.NewRat(1, 4)))
	branch := map[[2]int]*big.Rat{
		{0, 0}: constant.Neg(constant), {1, 0}: linear.Neg(linear), {2, 0}: k.Neg(k), {0, 1}: big.NewRat(1, 1),
	}
	cx := new(big.Rat).Sub(f.station, new(big.Rat).Mul(f.slope, f.normal))
	cx.Add(cx, f.shift)
	cy := new(big.Rat).Add(f.ordinate, f.normal)
	circleConstant := new(big.Rat).Add(new(big.Rat).Mul(cx, cx), new(big.Rat).Mul(cy, cy))
	circleConstant.Sub(circleConstant, f.radiusSquared)
	circle := map[[2]int]*big.Rat{{0, 0}: circleConstant, {2, 0}: big.NewRat(1, 1), {0, 2}: big.NewRat(1, 1),
		{1, 0}: new(big.Rat).Mul(big.NewRat(-2, 1), cx), {0, 1}: new(big.Rat).Mul(big.NewRat(-2, 1), cy)}
	power := make(map[[2]int]*big.Rat)
	for i, a := range branch {
		for j, b := range circle {
			var degree [2]int
			degree[axis], degree[1-axis] = i[0]+j[0], i[1]+j[1]
			if power[degree] == nil {
				power[degree] = new(big.Rat)
			}
			term := new(big.Rat).Mul(a, b)
			power[degree].Add(power[degree], term.Mul(term, big.NewRat(9, 16)))
		}
	}
	basis := func(i, k int) *big.Rat {
		return new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(4, int64(k)))
	}
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	var out [3]*geom.BSplineSurface
	for side := range out {
		var poles []geom.Vec3
		for i := range 5 {
			for j := range 5 {
				z := 0.
				if side == 1 {
					z = 2
				} else if side == 2 {
					exact := big.NewRat(2, 1)
					for k, x := range power {
						v := new(big.Rat).Mul(basis(i, k[0]), basis(j, k[1]))
						exact.Add(exact, v.Mul(v, x))
					}
					var ok bool
					z, ok = exact.Float64()
					if !ok {
						t.Fatal("original tensor pole is not exactly representable", exact)
					}
					if changed && i == 2 && j == 3 {
						z = math.Nextafter(z, math.Inf(1))
					}
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
			}
		}
		var err error
		out[side], err = geom.NewBSplineSurface(4, 4, 5, 5, poles, knots, knots, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestParabolaCircleObliqueTangencyGeometry(t *testing.T) {
	ctx := context.Background()
	opts := geom.GraphPointContactOptions{}
	for axis := range 2 {
		for side := range 2 {
			base := originalObliqueCircleFixture(side)
			reflection := base
			reflection.slope = big.NewRat(-3, 4)
			moved := base
			moved.station = big.NewRat(7, 16)
			for profile, fixture := range []obliqueCircleFixture{base, reflection, moved} {
				stock := parabolaCircleObliqueStock(t, axis, fixture, false)
				for order := range 2 {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", axis, side, profile, order), func(t *testing.T) {
						src := parabolaTwoCirclesPatches(t, stock[1+order], stock[2-order])
						got, err := geom.DiscoverSplineGraphParabolaCircleObliqueTangency(ctx, src[0], src[1], axis, opts)
						if err != nil {
							t.Fatal(err)
						}
						signs := [3]int{1 - 2*order, 2*order - 1}
						signs[2] = -signs[side]
						wantCenter := new(big.Rat).Sub(fixture.station, new(big.Rat).Mul(fixture.slope, fixture.normal))
						if got.DiskRegion != side || got.RegionSides != signs || got.ExactOperations < 1 ||
							got.CircleCenter[axis].Cmp(wantCenter) != 0 ||
							got.CircleCenter[1-axis].Cmp(big.NewRat(int64(7+2*side), 16)) != 0 ||
							got.RadiusSquared.Cmp(big.NewRat(25, 4096)) != 0 {
							t.Fatal("changed original factor or strict occupied side", got)
						}
						if got.Parameters[axis].Cmp(fixture.station) != 0 || got.Parameters[1-axis].Cmp(big.NewRat(1, 2)) != 0 ||
							got.Point[axis].Cmp(fixture.station) != 0 || got.Point[1-axis].Cmp(big.NewRat(1, 2)) != 0 ||
							got.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
							t.Fatal("changed original rational contact", got)
						}
						if got.Slope.Cmp(fixture.slope) != 0 || got.NormalOrdinate.Cmp(fixture.normal) != 0 ||
							got.ParabolaSecondDerivative.Cmp(big.NewRat(1, 4)) != 0 ||
							got.CircleSecondDerivative.Cmp(big.NewRat(int64(50*side-25), 1)) != 0 ||
							got.SubstitutedSecondDerivative.Cmp(big.NewRat(int64(101-2*side), 32)) != 0 {
							t.Fatal("changed original unequal second derivatives", got)
						}
						for _, owner := range got.Owners {
							tangent := owner.Tangent
							if tangent[axis].Cmp(big.NewRat(1, 1)) != 0 || tangent[1-axis].Cmp(fixture.slope) != 0 || tangent[2].Sign() != 0 ||
								owner.Parameters[axis].Cmp(fixture.station) != 0 || owner.Parameters[1-axis].Cmp(big.NewRat(1, 2)) != 0 {
								t.Fatal("changed the original shared owner ray", owner)
							}
						}
					})
				}
			}
		}
	}
}

func TestParabolaCircleObliqueTangencyRefusals(t *testing.T) {
	ctx := context.Background()
	good := originalObliqueCircleFixture(1)
	crossed, separate, apex := good, good, good
	crossed.radiusSquared = big.NewRat(26, 4096)
	separate.radiusSquared = big.NewRat(24, 4096)
	apex.slope, apex.radiusSquared = new(big.Rat), big.NewRat(1, 256)
	for _, c := range []struct {
		name       string
		source     obliqueCircleFixture
		changed    bool
		axis, work int
	}{
		{"transverse crossings", crossed, false, 0, 0},
		{"strict separation", separate, false, 0, 0},
		{"apex tangency", apex, false, 0, 0},
		{"changed original coefficients", good, true, 0, 0},
		{"wrong chart axis", good, false, 1, 0},
		{"short proof", good, false, 0, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			stock := parabolaCircleObliqueStock(t, 0, c.source, c.changed)
			src := parabolaTwoCirclesPatches(t, stock[1], stock[2])
			got, err := geom.DiscoverSplineGraphParabolaCircleObliqueTangency(ctx, src[0], src[1], c.axis, geom.GraphPointContactOptions{MaxOperations: c.work})
			if got != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("unsupported or incomplete original proof published a tangent", got, err)
			}
		})
	}
	stock := parabolaCircleObliqueStock(t, 0, good, false)
	p := parabolaTwoCirclesPatches(t, stock[1], stock[2])
	if got, err := geom.DiscoverSplineGraphParabolaCircleApexTangency(ctx, p[0], p[1], 0, geom.GraphPointContactOptions{}); got != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("original apex entry admitted a nonapex tangency", got, err)
	}
}
