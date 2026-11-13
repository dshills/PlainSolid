package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestFiniteSphereCylinderSingularGraph(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	q, e := BuildFiniteSphereCylinderWallGraph(ctx, Vec3{X: 2}, 3, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-4, 1), r(4, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || len(q.Components) != 1 || !q.Components[0].Singular || q.Components[0].CycleRank != 2 || q.Components[0].HasBoundary {
		t.Fatal(q, e)
	}
	nodes := 0
	for _, v := range q.Events {
		if len(v.IncidentArcs) == 4 {
			nodes++
			if v.HeightSign != 0 || v.ProjectionExtreme != -1 {
				t.Fatal(v)
			}
		}
	}
	if nodes != 1 {
		t.Fatal(q.Events)
	}
	// Clip exactly at the singular equator: only the upper loop survives;
	// the zero-width lower sheet must not create a spurious component.
	q, e = BuildFiniteSphereCylinderWallGraph(ctx, Vec3{X: 2}, 3, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(0, 1), r(4, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Components) != 1 || q.Components[0].CycleRank != 1 || q.Components[0].Singular {
		t.Fatal(q, e)
	}
	found := false
	for _, v := range q.Events {
		if v.HeightSign == 0 {
			found = true
			if v.CapMask != 1 || len(v.IncidentArcs) != 2 {
				t.Fatal(v)
			}
		}
	}
	if !found {
		t.Fatal(q)
	}
	q, e = BuildFiniteSphereCylinderWallGraph(ctx, Vec3{X: 2}, 1, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-1, 1), r(1, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Components) != 1 || !q.Components[0].IsolatedPoint || len(q.Events) != 1 {
		t.Fatal(q, e)
	}
	// A regular fold on a cap joins neither nonexistent negative-height edge
	// nor another half: it is a terminal of the single retained positive arc.
	q, e = BuildFiniteSphereCylinderWallGraph(ctx, Vec3{X: 2}, 2, Vec3{}, Vec3{Z: -2}, 1, [2]*big.Rat{r(-1, 1), r(0, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Components) != 1 || !q.Components[0].HasBoundary || q.Components[0].CycleRank != 0 {
		t.Fatal(q, e)
	}
	terminals := 0
	for _, v := range q.Events {
		if len(v.IncidentArcs) == 1 {
			terminals++
			if v.CapMask != 2 || v.HeightSign != 0 {
				t.Fatal(v)
			}
		}
	}
	if terminals != 2 {
		t.Fatal(q.Events)
	}
	q, e = BuildFiniteSphereCylinderWallGraph(ctx, Vec3{X: 2}, 3, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-4, 1), r(4, 1)}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Events) != 0 {
		t.Fatal(q, e)
	}
}
