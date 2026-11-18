package trimmed

import (
	"context"
	"testing"

	"plainsolid/geom"
)

func oppositeCircleStock() [2]twoCircleStockProbe {
	// The upper circle comes first in free-coordinate order. Native
	// assembly visits the lower enclosing roof before the upper roof.
	return [2]twoCircleStockProbe{twoCircleProbe(2, 6, 1, 8), twoCircleProbe(12, 2, 1, 16)}
}

func TestParabolaOppositeCirclesSourceDiscovery(t *testing.T) {
	ctx := context.Background()
	circles := oppositeCircleStock()
	for axis := range 2 {
		stock := parabolaTwoCirclesStock(t, axis, circles, false)
		for order := range 2 {
			p := parabolaTwoCirclesPatches(t, stock[1+order], stock[2-order])
			proof, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, p[0], p[1], axis, geom.GraphPointContactOptions{})
			if err != nil {
				t.Fatal(axis, order, err)
			}
			signs := [2][4]int{{1, -1, 1, -1}, {-1, 1, -1, 1}}[order]
			if proof.CircleRegions != ([2]int{1, 0}) || proof.CirclesAbove || proof.RegionSides != signs || !proof.Graph.ProvenInjective {
				t.Fatal("wrong source regions for opposite circles", proof)
			}
			for i, circle := range circles {
				c := proof.Circles[i]
				if c.Center[axis].Cmp(circle.x) != 0 || c.Center[1-axis].Cmp(circle.y) != 0 || c.RadiusSquared.Cmp(circle.radiusSquared) != 0 {
					t.Fatal("source proof swapped the circle factors", i, c)
				}
			}
		}
	}
}
