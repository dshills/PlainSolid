package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func localOppositeCircleStock() [2]twoCircleStockProbe {
	return [2]twoCircleStockProbe{twoCircleProbe(4, 10, 1, 32), twoCircleProbe(56, 20, 1, 64)}
}

func TestParabolaCircleLocalSeparationGeometry(t *testing.T) {
	ctx := context.Background()
	opts := geom.GraphPointContactOptions{}
	opposite := localOppositeCircleStock()
	// For the second layout the right circle's lower extent is below
	// p(13/16), its local maximum. The intersecting ordinate ranges alone
	// cannot prove separation. With L=1/4 bounding p' on the whole stock,
	// the squared distance is at least gap²/(1+L²)>r².
	overlappingLocalBox := [2]twoCircleStockProbe{opposite[0], twoCircleProbe(192, 99, 16, 256)}
	gap := big.NewRat(17, 256)
	distanceLower := new(big.Rat).Mul(gap, gap)
	distanceLower.Quo(distanceLower, big.NewRat(17, 16))
	if distanceLower.Cmp(overlappingLocalBox[1].radiusSquared) <= 0 {
		t.Fatal("test must independently provide strict circle separation")
	}
	irrationalRadius := opposite
	irrationalRadius[0].radiusSquared = big.NewRat(65, 65536)
	for _, c := range []struct {
		name    string
		circles [2]twoCircleStockProbe
		regions [2]int
	}{
		{"equal ordinate, opposite sides", opposite, [2]int{1, 0}},
		{"overlapping local ordinate bounds", overlappingLocalBox, [2]int{1, 1}},
		{"irrational radius, opposite sides", irrationalRadius, [2]int{1, 0}},
	} {
		t.Run(c.name, func(t *testing.T) {
			for axis := range 2 {
				src := parabolaTwoCirclesStock(t, axis, c.circles, false)
				for order := range 2 {
					patches := parabolaTwoCirclesPatches(t, src[1+order], src[2-order])
					got, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, patches[0], patches[1], axis, opts)
					if err != nil || got.CircleRegions != c.regions || !got.Graph.ProvenInjective {
						t.Fatalf("axis %d order %d lost exact open region binding: %+v, %v", axis, order, got, err)
					}
					wantSides := [4]int{1 - 2*order, 2*order - 1}
					for i, circle := range got.Circles {
						original := c.circles[i]
						wantSides[2+i] = -wantSides[c.regions[i]]
						if circle.Center[axis].Cmp(original.x) != 0 || circle.Center[1-axis].Cmp(original.y) != 0 ||
							circle.RadiusSquared.Cmp(original.radiusSquared) != 0 {
							t.Fatal("changed circle factor or order", circle)
						}
					}
					if got.RegionSides != wantSides {
						t.Fatal("wrong certified original height signs", got.RegionSides, wantSides)
					}
				}
			}
		})
	}
}

func TestParabolaSingleCircleLocalSeparationGeometry(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		ordinate *big.Rat
		above    bool
	}{
		{big.NewRat(43, 128), true},
		{big.NewRat(31, 128), false},
	} {
		for axis := range 2 {
			src := parabolaCircleGraphStock(t, axis, c.ordinate, big.NewRat(1, 1024), false)
			patches := parabolaTwoCirclesPatches(t, src[1], src[2])
			got, err := geom.DiscoverSplineGraphParabolaCircle(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
			if err != nil || got.CircleAbove != c.above || got.CircleCenter[1-axis].Cmp(c.ordinate) != 0 {
				t.Fatal("single-circle source did not bind its original open side", c, got, err)
			}
		}
	}
}

func TestParabolaCircleLocalSeparationRefusals(t *testing.T) {
	ctx := context.Background()
	stock := localOppositeCircleStock()
	// The first circle is normal to the parabola at (1/2,9/32).
	// Its center is (1/2,9/32)+(−1/8,1)/32, so the exact substituted
	// quartic has an interior double root with positive endpoint values.
	tangent := twoCircleStockProbe{big.NewRat(127, 256), big.NewRat(5, 16), big.NewRat(65, 65536)}
	for _, c := range []struct {
		name    string
		first   twoCircleStockProbe
		changed bool
		budget  int
	}{
		{"interior tangent quartic", tangent, false, 0},
		{"transverse quartic", twoCircleProbe(16, 9, 2, 32), false, 0},
		{"stock boundary", twoCircleProbe(1, 10, 1, 32), false, 0},
		{"changed original coefficients", stock[0], true, 0},
		{"exhausted proof", stock[0], false, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := parabolaTwoCirclesStock(t, 0, [2]twoCircleStockProbe{c.first, stock[1]}, c.changed)
			patches := parabolaTwoCirclesPatches(t, src[1], src[2])
			if got, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, patches[0], patches[1], 0, geom.GraphPointContactOptions{MaxOperations: c.budget}); got != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("unsupported original circle published a separation", got, err)
			}
		})
	}
}
