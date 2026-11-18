//go:build research

package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestExactGraphWallEndpointStillRejectsOneULP(t *testing.T) {
	ctx := context.Background()
	base, a, b := polynomialGraphFixture(t, 3, 0)
	solid, _, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{Polynomial: &geom.GraphPointContactOptions{}})
	if e != nil {
		t.Fatal(e)
	}
	for _, face := range solid.faces {
		for _, use := range face.Outer {
			reversed, ok := use.Curve.(reverseTrim)
			if !ok {
				continue
			}
			wall, ok := reversed.differentiableTrim.(wallTrim)
			if !ok || wall.surface != b {
				continue
			}
			// Keep the corrupted local pcurve endpoint and declared UV consistent;
			// only the exact original model witness can reject this one-ULP change.
			wall.start = math.Nextafter(wall.start, math.Inf(1))
			use.UVEnd.Y = wall.start
			use.Curve = reverseTrim{wall}
			equal, e := compareSplineWallUse(ctx, solid.edges[use.Edge], face.Surface, use, edgecheck.Options{})
			if e != nil || equal {
				t.Fatal("one-ULP model mismatch accepted", equal, e)
			}
			return
		}
	}
	t.Fatal("missing original B wall")
}
