package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func offsetCirclesStock() [2]twoCircleStockProbe {
	// The two free-coordinate intervals overlap. Only the true Euclidean
	// center distance separates these circular footprints.
	return [2]twoCircleStockProbe{twoCircleProbe(6, 9, 2, 16), twoCircleProbe(17, 25, 3, 32)}
}

func TestParabolaOffsetCirclesSourceDiscovery(t *testing.T) {
	ctx := context.Background()
	fixtures := [][2]twoCircleStockProbe{
		offsetCirclesStock(),
		{twoCircleProbe(8, 9, 1, 16), twoCircleProbe(16, 27, 3, 32)}, // equal free centers
		{twoCircleProbe(8, 3, 1, 32), twoCircleProbe(24, 5, 2, 32)},  // both below
	}
	for layout, circles := range fixtures {
		for axis := range 2 {
			stock := parabolaTwoCirclesStock(t, axis, circles, false)
			for order := range 2 {
				patches := parabolaTwoCirclesPatches(t, stock[1+order], stock[2-order])
				r, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
				if err != nil {
					t.Fatal(layout, axis, order, err)
				}
				sides := [4]int{1, -1, 1, 1}
				if layout == 2 {
					sides[2], sides[3] = -1, -1
				}
				if order == 1 {
					for j := range sides {
						sides[j] = -sides[j]
					}
				}
				if r.CirclesAbove != (layout != 2) || r.RegionSides != sides || !r.Graph.ProvenInjective {
					t.Fatal("wrong signed regions for offset circles", layout, axis, order, r)
				}
				for i, c := range circles {
					got := r.Circles[i]
					if got.Center[axis].Cmp(c.x) != 0 || got.Center[1-axis].Cmp(c.y) != 0 || got.RadiusSquared.Cmp(c.radiusSquared) != 0 {
						t.Fatal("wrong exact circle center or radius after original-source factorization", i, got)
					}
				}
				for i, y := range [3]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 4), big.NewRat(3, 8)} {
					if r.ParabolaControls[i][axis].Cmp(big.NewRat(int64(i), 2)) != 0 ||
						r.ParabolaControls[i][1-axis].Cmp(y) != 0 {
						t.Fatal("wrong exact parabola after offset circle factorization", r.ParabolaControls)
					}
				}
			}
		}
	}
}
