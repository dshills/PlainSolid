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

type parabolaCircleOsculationFixture struct {
	station, ordinate, bend, radiusOffset *big.Rat
}

func originalOsculationFixture(side int) parabolaCircleOsculationFixture {
	sign := int64(2*side - 1)
	return parabolaCircleOsculationFixture{
		station: big.NewRat(1, 2), ordinate: big.NewRat(4-3*sign, 8), bend: big.NewRat(2*sign, 1), radiusOffset: big.NewRat(sign, 4),
	}
}

func parabolaCircleOsculationStock(t *testing.T, axis int, f parabolaCircleOsculationFixture, changed bool) [3]*geom.BSplineSurface {
	t.Helper()
	// Author the original variable roof as a tensor expansion of
	// 9/16*(v−s−k*(u−x0)²)*((u−x0)²+(v−s−δ)²−δ²).
	cy := new(big.Rat).Add(f.ordinate, f.radiusOffset)
	factor := map[[2]int]*big.Rat{
		{0, 0}: new(big.Rat).Neg(new(big.Rat).Add(f.ordinate, new(big.Rat).Mul(f.bend, new(big.Rat).Mul(f.station, f.station)))),
		{1, 0}: new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(f.bend, f.station)),
		{2, 0}: new(big.Rat).Neg(f.bend), {0, 1}: big.NewRat(1, 1),
	}
	radiusSquared := new(big.Rat).Mul(f.radiusOffset, f.radiusOffset)
	constant := new(big.Rat).Add(new(big.Rat).Mul(f.station, f.station), new(big.Rat).Mul(cy, cy))
	circle := map[[2]int]*big.Rat{
		{0, 0}: constant.Sub(constant, radiusSquared), {2, 0}: big.NewRat(1, 1), {0, 2}: big.NewRat(1, 1),
		{1, 0}: new(big.Rat).Mul(big.NewRat(-2, 1), f.station), {0, 1}: new(big.Rat).Mul(big.NewRat(-2, 1), cy),
	}
	coeff := make(map[[2]int]*big.Rat)
	for i, a := range factor {
		for j, b := range circle {
			var degree [2]int
			degree[axis], degree[1-axis] = i[0]+j[0], i[1]+j[1]
			if coeff[degree] == nil {
				coeff[degree] = new(big.Rat)
			}
			term := new(big.Rat).Mul(a, b)
			coeff[degree].Add(coeff[degree], term.Mul(term, big.NewRat(9, 16)))
		}
	}
	basis := func(i, k int) *big.Rat {
		return new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(4, int64(k)))
	}
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	var out [3]*geom.BSplineSurface
	for roof := range out {
		var poles []geom.Vec3
		for i := range 5 {
			for j := range 5 {
				z := 0.
				if roof > 0 {
					z = 2
				}
				if roof == 2 {
					exact := big.NewRat(2, 1)
					for k, x := range coeff {
						term := new(big.Rat).Mul(basis(i, k[0]), basis(j, k[1]))
						exact.Add(exact, term.Mul(term, x))
					}
					var ok bool
					z, ok = exact.Float64()
					if !ok {
						t.Fatal("fixture source pole must be exactly representable", exact)
					}
					if changed && i == 2 && j == 3 {
						z = math.Nextafter(z, math.Inf(1))
					}
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
			}
		}
		var err error
		out[roof], err = geom.NewBSplineSurface(4, 4, 5, 5, poles, knots, knots, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestParabolaCircleOsculationGeometry(t *testing.T) {
	ctx := context.Background()
	one := big.NewRat(1, 1)
	for axis := range 2 {
		for side := range 2 {
			for shifted := range 2 {
				f := originalOsculationFixture(side)
				if shifted == 1 {
					f.station = big.NewRat(7, 16)
				}
				stock := parabolaCircleOsculationStock(t, axis, f, false)
				for order := range 2 {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", axis, side, shifted, order), func(t *testing.T) {
						patch := parabolaTwoCirclesPatches(t, stock[1+order], stock[2-order])
						p, err := geom.DiscoverSplineGraphParabolaCircleOsculation(ctx, patch[0], patch[1], axis, geom.GraphPointContactOptions{})
						if err != nil {
							t.Fatal(err)
						}
						sign := int64(2*side - 1)
						expectedSides := [3]int{1 - 2*order, 2*order - 1}
						expectedSides[2] = -expectedSides[side]
						if p.DiskRegion != side || p.RegionSides != expectedSides || p.ExactOperations < 1 ||
							p.Point[axis].Cmp(f.station) != 0 || p.Point[1-axis].Cmp(f.ordinate) != 0 ||
							p.Point[2].Cmp(big.NewRat(2, 1)) != 0 || p.CircleCenter[axis].Cmp(f.station) != 0 ||
							p.CircleCenter[1-axis].Cmp(new(big.Rat).Add(f.ordinate, f.radiusOffset)) != 0 ||
							p.RadiusSquared.Cmp(big.NewRat(1, 16)) != 0 {
							t.Fatal("changed the original exact wall circle relationship", p)
						}
						mid := p.ParabolaControls[1][1-axis]
						if mid.Sign() >= 0 && mid.Cmp(one) <= 0 {
							t.Fatal("fixture did not exercise outlying Bernstein ordinate", mid)
						}
						if p.SharedSecondDerivative.Cmp(big.NewRat(4*sign, 1)) != 0 ||
							p.CircleFourthDerivative.Cmp(big.NewRat(192*sign, 1)) != 0 ||
							p.SubstitutedFourthDerivative.Cmp(big.NewRat(96, 1)) != 0 {
							t.Fatal("changed exact matched curvature or first unequal fourth derivative", p)
						}
						for wall, exit := range p.ParabolaExits {
							d := new(big.Rat).Sub(big.NewRat(int64(wall), 1), f.station)
							y := new(big.Rat).Add(f.ordinate, new(big.Rat).Mul(f.bend, new(big.Rat).Mul(d, d)))
							if exit[axis].Cmp(big.NewRat(int64(wall), 1)) != 0 || exit[1-axis].Cmp(y) != 0 || exit[2].Cmp(big.NewRat(2, 1)) != 0 {
								t.Fatal("changed verified stock exit", exit)
							}
						}
						for _, owner := range p.Owners {
							if owner.FreeTangent[axis].Cmp(one) != 0 || owner.FreeTangent[1-axis].Sign() != 0 ||
								owner.FreeTangent[2].Sign() != 0 || owner.FourthJetGap[axis].Sign() != 0 ||
								owner.FourthJetGap[1-axis].Cmp(big.NewRat(192*sign, 1)) != 0 || owner.FourthJetGap[2].Sign() != 0 ||
								owner.Parameters[axis].Cmp(f.station) != 0 || owner.Parameters[1-axis].Cmp(f.ordinate) != 0 {
								t.Fatal("changed original source owner jet", owner)
							}
						}
						if double, err := geom.DiscoverSplineGraphParabolaCircleApexTangency(ctx, patch[0], patch[1], axis, geom.GraphPointContactOptions{}); double != nil || !errors.Is(err, geom.ErrConvergence) {
							t.Fatal("legacy apex entry admitted order four", double, err)
						}
					})
				}
			}
		}
	}
}

func TestParabolaCircleOsculationRefusals(t *testing.T) {
	ctx := context.Background()
	good := originalOsculationFixture(1)
	double, crossing, corner, diskWall := good, good, good, good
	double.bend, crossing.bend = big.NewRat(7, 4), big.NewRat(9, 4)
	corner.station, corner.ordinate = big.NewRat(3, 8), big.NewRat(7, 32)
	diskWall.ordinate = big.NewRat(1, 2)
	for _, c := range []struct {
		name       string
		original   parabolaCircleOsculationFixture
		changed    bool
		axis, work int
	}{
		{"apex double root", double, false, 0, 0},
		{"other real crossings", crossing, false, 0, 0},
		{"wall exit at corner", corner, false, 0, 0},
		{"circle touches stock wall", diskWall, false, 0, 0},
		{"changed original coefficient", good, true, 0, 0},
		{"wrong free axis", good, false, 1, 0},
		{"short proof", good, false, 0, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			stock := parabolaCircleOsculationStock(t, 0, c.original, c.changed)
			patch := parabolaTwoCirclesPatches(t, stock[1], stock[2])
			out, err := geom.DiscoverSplineGraphParabolaCircleOsculation(ctx, patch[0], patch[1], c.axis, geom.GraphPointContactOptions{MaxOperations: c.work})
			if out != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("unqualified original source published fourth-order contact", out, err)
			}
		})
	}
}
