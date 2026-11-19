package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func authoredParabolaCircleSimilarityStock(t *testing.T, source [3]*geom.BSplineSurface) (out [3]*geom.BSplineSurface) {
	t.Helper()
	for i, raw := range source {
		d := raw.Data()
		for j, p := range d.Poles {
			// The stored world control net uses integer basis vectors.
			// Dividing those vectors by five gives a tilted 3-4-5 rotation.
			d.Poles[j] = geom.Vec3{X: 5*p.X + 17./8, Y: 3*p.Y - 4*p.Z - 3, Z: 4*p.Y + 3*p.Z + 5}
		}
		var err error
		out[i], err = geom.NewBSplineSurfaceWithExactWeights(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestPlacedParabolaCircleRejectsNonSimilarity(t *testing.T) {
	ctx := context.Background()
	local := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	world := authoredParabolaCircleSimilarityStock(t, local)
	d := world[0].Data()
	k := (d.CountU - 1) * d.CountV
	d.Poles[k].Z = math.Nextafter(d.Poles[k].Z, math.Inf(1))
	skew, err := geom.NewBSplineSurfaceWithExactWeights(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if err != nil {
		t.Fatal(err)
	}
	if cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, skew, world[1], world[2], 0, 1e-9, ParabolaCircleGraphOptions{}); cut != nil ||
		!errors.Is(err, geom.ErrConvergence) {
		t.Fatal("accepted a skewed original base frame", cut, err)
	}
}
