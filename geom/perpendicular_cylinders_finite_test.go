package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestFinitePerpendicularCylinders(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	wide := [2]*big.Rat{r(-3, 1), r(3, 1)}
	q, e := ClipPerpendicularCylinderIntersection(ctx, Vec3{}, 1, Vec3{}, 2, wide, wide, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || len(q.Components) != 2 {
		t.Fatal(q, e)
	}
	for _, c := range q.Components {
		if c.CycleRank != 1 || c.HasBoundary {
			t.Fatal(c)
		}
	}
	q, e = ClipPerpendicularCylinderIntersection(ctx, Vec3{}, 1, Vec3{}, 2, [2]*big.Rat{r(0, 1), r(3, 1)}, wide, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Components) != 1 || q.Components[0].CycleRank != 1 {
		t.Fatal(q, e)
	}
	q, e = ClipPerpendicularCylinderIntersection(ctx, Vec3{}, 1, Vec3{}, 2, [2]*big.Rat{r(2, 1), r(3, 1)}, wide, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Components) != 2 || len(q.Arcs) != 0 {
		t.Fatal(q, e)
	}
	for _, n := range q.Nodes {
		if n.Caps != PerpendicularCapXMin || n.CapTangencies != PerpendicularCapXMin {
			t.Fatal(n)
		}
	}
	for _, c := range q.Components {
		if !c.IsolatedPoint {
			t.Fatal(c)
		}
	}
	q, e = ClipPerpendicularCylinderIntersection(ctx, Vec3{}, 1, Vec3{}, 2, [2]*big.Rat{r(9, 5), r(11, 5)}, [2]*big.Rat{r(-1, 2), r(1, 2)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Arcs) != 4 || len(q.Components) != 4 {
		t.Fatal(q, e)
	}
	for _, a := range q.Arcs {
		if a.SquaredY[0].Cmp(r(3, 4)) != 0 || a.SquaredY[1].Cmp(r(19, 25)) != 0 {
			t.Fatal(a)
		}
	}
	for _, n := range q.Nodes {
		if n.Caps == 0 || n.CapTangencies != 0 {
			t.Fatal(n)
		}
	}
	q, e = ClipPerpendicularCylinderIntersection(ctx, Vec3{}, 1, Vec3{}, 1, wide, wide, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Components) != 1 || !q.Components[0].Singular || q.Components[0].CycleRank != 3 {
		t.Fatal(q, e)
	}
	count := 0
	for _, n := range q.Nodes {
		if n.SurfaceTangent {
			count++
			if len(n.IncidentArcs) != 4 {
				t.Fatal(n)
			}
		}
	}
	if count != 2 {
		t.Fatal(q.Nodes)
	}
	// Translate crossing: cap ranges remain model coordinates.
	q, e = ClipPerpendicularCylinderIntersection(ctx, Vec3{Y: 2, Z: 3}, 1, Vec3{X: 5, Y: 2}, 2, [2]*big.Rat{r(7, 1), r(8, 1)}, [2]*big.Rat{r(0, 1), r(6, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Nodes) != 2 || q.Nodes[0].Bounds[0][0].Cmp(r(7, 1)) != 0 {
		t.Fatal(q, e)
	}
	q, e = ClipPerpendicularCylinderIntersection(ctx, Vec3{}, 1, Vec3{}, 2, wide, wide, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Nodes) != 0 {
		t.Fatal(q, e)
	}
}
