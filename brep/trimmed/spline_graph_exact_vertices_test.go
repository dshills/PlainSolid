package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestExactGraphVertexEvaluation(t *testing.T) {
	ctx := context.Background()
	_, _, b := polynomialGraphFixture(t, 3, 0)
	point, e := exactGraphVertexEvaluator(ctx, geom.GraphPointContactOptions{})(b, openSectionPiece{a: geom.Vec2{X: 1, Y: .375}})
	if e != nil || point != (geom.Vec3{X: 1, Y: .375, Z: 1539.0 / 512}) {
		t.Fatal(point, e)
	}
	// On affected floating evaluators the original B knot induces one ULP error;
	// correctness depends on the exact fraction, never on reproducing that error.
	limited := exactGraphVertexEvaluator(ctx, geom.GraphPointContactOptions{MaxOperations: 1})
	if _, e = limited(b, openSectionPiece{a: geom.Vec2{X: 1, Y: .375}}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("vertex budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = exactGraphVertexEvaluator(canceled, geom.GraphPointContactOptions{})(b, openSectionPiece{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestExactGraphVertexRejectsNonbinaryModelPoint(t *testing.T) {
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1, Z: 1}, {X: 1, Y: 1, Z: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, 2, 2})
	if e != nil {
		t.Fatal(e)
	}
	point, e := exactGraphVertexEvaluator(context.Background(), geom.GraphPointContactOptions{})(s, openSectionPiece{a: geom.Vec2{X: .5}})
	if e == nil || point != (geom.Vec3{}) {
		t.Fatal("rounded rational model vertex", point, e)
	}
}

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
