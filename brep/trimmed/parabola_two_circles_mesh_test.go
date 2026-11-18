package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep/parametric"
)

func TestCertifiedParabolaTwoCirclesCutMeshBudgetIsAtomic(t *testing.T) {
	ctx := context.Background()
	source := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1], source[2], 0, 1e-9, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := cut.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{MaxTriangles: 4})
	if got != nil || !errors.Is(err, parametric.ErrBudget) {
		t.Fatalf("insufficient budget yielded an incomplete display: %v, %v", got, err)
	}
}
