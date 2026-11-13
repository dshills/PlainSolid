package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSphereSlabCylinderCover(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	// Nearest height1 gives section radius sqrt3; offset1/4 plus sqrt3<2.
	q, e := ClassifySphereSlabCylinderCover(ctx, Vec3{X: .25}, 2, Vec3{}, Vec3{Z: 3}, 2, [2]*big.Rat{r(1, 3), r(1, 2)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || q.Coverage != SphereSlabStrictlyCovered || q.NearestHeight.Cmp(r(1, 1)) != 0 || q.MaximumSectionRadiusSquared.Cmp(r(3, 1)) != 0 {
		t.Fatal(q, e)
	}
	for _, axis := range []Vec3{{Z: 1}, {Z: -2}} {
		q, e = ClassifySphereSlabCylinderCover(ctx, Vec3{X: 1}, 5, Vec3{}, axis, 4, [2]*big.Rat{r(4, 1), r(5, 1)}, SurfacePatchEvaluationOptions{})
		if axis.Z == 1 {
			if e != nil || q.Coverage != SphereSlabCoveredTouching || q.MaximumSectionRadiusSquared.Cmp(r(9, 1)) != 0 {
				t.Fatal(q, e)
			}
		} else if e != nil || q.Coverage != SphereSlabNoMaterial {
			t.Fatal(q, e)
		}
	}
	q, e = ClassifySphereSlabCylinderCover(ctx, Vec3{X: 1}, 5, Vec3{}, Vec3{Z: 1}, math.Nextafter(4, 0), [2]*big.Rat{r(4, 1), r(5, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || q.Coverage != SphereSlabNotCovered {
		t.Fatal(q, e)
	}
	q, e = ClassifySphereSlabCylinderCover(ctx, Vec3{}, 2, Vec3{}, Vec3{Z: -3}, 3, [2]*big.Rat{r(-1, 1), r(1, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || q.NearestParameter.Sign() != 0 || q.NearestHeight.Sign() != 0 || q.MaximumSectionRadiusSquared.Cmp(r(4, 1)) != 0 || q.Coverage != SphereSlabStrictlyCovered {
		t.Fatal(q, e)
	}
	q, e = ClassifySphereSlabCylinderCover(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(2, 1), r(3, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || q.Coverage != SphereSlabCoveredTouching || q.MaximumSectionRadiusSquared.Sign() != 0 {
		t.Fatal(q, e)
	}
	q, e = ClassifySphereSlabCylinderCover(ctx, Vec3{}, 2, Vec3{}, Vec3{Z: 1}, 3, [2]*big.Rat{r(0, 1), r(1, 1)}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete {
		t.Fatal(q, e)
	}
}
