package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSphereCylinderAxialPlan(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	q, e := PlanSphereCylinderAxialCaps(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 1}, 2, [2]*big.Rat{r(-3, 1), r(3, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || len(q.Cells) != 6 {
		t.Fatal(q, e)
	}
	want := []SphereCylinderCapMaterialKind{SphereCylinderCapEmpty, SphereCylinderCapSphereDisk, SphereCylinderCapLens, SphereCylinderCapLens, SphereCylinderCapSphereDisk, SphereCylinderCapEmpty}
	for i, c := range q.Cells {
		if c.Material != want[i] || c.WitnessParameter.Cmp(q.Events[c.Lower].Bounds[1]) <= 0 || c.WitnessParameter.Cmp(q.Events[c.Upper].Bounds[0]) >= 0 {
			t.Fatal(i, c, q.Events)
		}
	}
	// Exact nested radical event: A=-1, sqrt(B)=4, hence height²3.
	ev := q.Events[4]
	if ev.HeightSign != 1 || ev.SquaredBase.Cmp(r(-1, 1)) != 0 || ev.SquaredRadicand.Cmp(r(16, 1)) != 0 || ev.RadicalSign != 1 {
		t.Fatal(ev)
	}
	q, e = PlanSphereCylinderAxialCaps(ctx, Vec3{}, 3, Vec3{}, Vec3{Z: -3}, 1, [2]*big.Rat{r(-4, 3), r(4, 3)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete {
		t.Fatal(q, e)
	}
	found := false
	for _, c := range q.Cells {
		if c.Material == SphereCylinderCapCylinderDisk {
			found = true
		}
	}
	if !found {
		t.Fatal(q.Cells)
	}
	// d=r makes the outer tangency equal a sphere pole: canonical deduplication.
	q, e = PlanSphereCylinderAxialCaps(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-3, 1), r(3, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || len(q.Events) != 5 {
		t.Fatal(q, e)
	}
	// Window endpoints exactly equal rational critical heights are not duplicated.
	q, e = PlanSphereCylinderAxialCaps(ctx, Vec3{}, 5, Vec3{}, Vec3{Z: 3}, 3, [2]*big.Rat{r(-4, 3), r(4, 3)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Events) != 3 {
		t.Fatal(q, e)
	}
	q, e = PlanSphereCylinderAxialCaps(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 1}, 2, [2]*big.Rat{r(-3, 1), r(3, 1)}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Events) != 0 {
		t.Fatal(q, e)
	}
}
