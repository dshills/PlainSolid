package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func authoredPlacedParabolaTwoCirclesStock(t *testing.T) [3]*geom.BSplineSurface {
	t.Helper()
	return placeParabolaTwoCirclesTestStock(t, parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false))
}

func placeParabolaTwoCirclesTestStock(t *testing.T, contacts [3]*geom.BSplineSurface) [3]*geom.BSplineSurface {
	t.Helper()
	var placed [3]*geom.BSplineSurface
	for i, source := range contacts {
		data := source.Data()
		for j, p := range data.Poles {
			// World poles are authored directly in the rotated and translated stock.
			data.Poles[j] = geom.Vec3{X: 5 - p.Z, Y: p.X - 3, Z: 17.0/8 - p.Y}
		}
		var err error
		placed[i], err = geom.NewBSplineSurfaceWithExactWeights(data.DegreeU, data.DegreeV, data.CountU, data.CountV,
			data.Poles, data.KnotsU, data.KnotsV, data.Weights)
		if err != nil {
			t.Fatal(err)
		}
	}
	return placed
}

func TestPlacedParabolaTwoCirclesRejectsPerturbedOwnerChart(t *testing.T) {
	ctx := context.Background()
	source := authoredPlacedParabolaTwoCirclesStock(t)
	d := source[1].Data()
	k := 4*d.CountV + 4
	d.Poles[k].Y = math.Nextafter(d.Poles[k].Y, math.Inf(1))
	moved, err := geom.NewBSplineSurfaceWithExactWeights(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if err != nil {
		t.Fatal(err)
	}
	if cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, source[0], moved, source[2], 0, 1e-9, ParabolaCircleGraphOptions{}); cut != nil ||
		!errors.Is(err, geom.ErrInvalid) {
		t.Fatal("accepted a world owner with a nonshared chart", cut, err)
	}
}
