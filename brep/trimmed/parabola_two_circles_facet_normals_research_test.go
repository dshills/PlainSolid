//go:build research

package trimmed

import (
	"context"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestCertifiedPlacedParabolaTwoCirclesFacetAngle(t *testing.T) {
	slowtest.Skip(t)
	const axis = 1
	source := placeParabolaTwoCirclesTestStock(t, parabolaTwoCirclesStock(t, axis, oppositeCircleStock(), false))
	ctx := context.Background()
	for order := range 2 {
		cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1+order], source[2-order], axis, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if order == 1 {
			cut = reopenPlacedParabolaTwoCircles(t, cut)
		}
		result, err := cut.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{NormalAngle: .1, NormalTurn: .2})
		if err != nil || len(result) != 2 {
			t.Fatal("facet-certified placed assembly", len(result), err)
		}
		uv := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.Y + 3, Y: 17.0/8 - p.Z} }
		checkOppositeCirclesSourceDisplay(t, cut.Components(), cut.History(), result, order, uv)
		for i, m := range result {
			checkParabolaCircleFacetDisplay(t, cut.world[i], m, .1, uv)
		}
	}
}
