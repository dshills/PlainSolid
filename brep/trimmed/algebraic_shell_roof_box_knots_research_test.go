//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOriginalWeightedRoofKnotRefinement(t *testing.T) {
	ctx := context.Background()
	a, b := weightedIdentityRoofStockWithKnots(t, false, 3, true), weightedIdentityRoofStockWithKnots(t, true, 2, true)
	sec, err := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{})
	if err != nil || !sec.Complete {
		t.Fatal("knot-refined original shells", sec.Coverage, err)
	}
	m, err := sec.CombineAlgebraic(ctx, parametric.Difference, AlgebraicShellBooleanOptions{})
	if err != nil {
		t.Fatal("knot-refined cavity", err)
	}
	q, err := m.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
	if err != nil {
		t.Fatal("prepare knot-refined roofs", err)
	}
	if limited, err := m.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{MaxPreparationOperations: q.PreparationOperations() - 1}); !errors.Is(err, geom.ErrConvergence) || limited != nil {
		t.Fatal("knot proof did not charge every span", limited, err)
	}
	r := big.NewRat
	line, err := q.IntersectLineRational(ctx,
		[3]*big.Rat{r(0, 1), r(0, 1), r(3, 2)}, [3]*big.Rat{r(1, 1), r(1, 1), r(1, 4)},
		r(1, 8), r(7, 8), MixedRoofLineOptions{})
	if err != nil || len(line.Events) != 1 || len(line.Events[0].Owners) != 1 || line.Events[0].Owners[0] != (MixedRoofFaceOwner{0, 1}) {
		t.Fatal("knot charts changed original line ownership", line, err)
	}
	sign, err := line.Events[0].Parameter.SignPolynomial(ctx, []*big.Rat{r(32, 1), r(-112, 1), r(48, 1), r(0, 1), r(1, 1), r(-2, 1), r(1, 1)}, geom.ExactCurvePlaneEventOptions{})
	if err != nil || sign.Sign != 0 {
		t.Fatal("station no longer satisfies the original roof", sign, err)
	}
	// This original inserted U knot lies on an exposed vertical section.
	xy := [2]*big.Rat{r(1, 4), r(1, 2)}
	z := r(0, 1)
	vertical, err := q.IntersectVerticalRational(ctx, xy, z, r(1, 1), r(0, 1), r(3, 1), geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(vertical.Spans) != 2 || len(vertical.Events) != 4 {
		t.Fatal("exact vertical cavity section on original knot", vertical, err)
	}
	point := [3]*big.Rat{xy[0], xy[1], r(0, 1)}
	normal, err := q.BoundaryNormalsRational(ctx, point, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(normal.Normals) != 1 || normal.Normals[0].Owner != (MixedRoofFaceOwner{0, 0}) || normal.Normals[0].Direction[2].Cmp(r(-1, 1)) != 0 {
		t.Fatal("knot split changed original floor ownership", normal, err)
	}
	mass, err := q.BoundMass(ctx, MixedRoofMassOptions{MaxDepth: 6})
	if err != nil || mass.Volume.Lower.Cmp(r(115, 100)) > 0 || mass.Volume.Upper.Cmp(r(116, 100)) < 0 {
		t.Fatal("knot-refined cavity mass enclosure", mass, err)
	}
}
