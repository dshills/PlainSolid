package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSphereCylinderCapMaterial(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	for _, c := range []struct {
		d, R, tool, z float64
		kind          SphereCylinderCapMaterialKind
		tangent       bool
	}{
		{3, 1, 1, 0, SphereCylinderCapEmpty, false}, {2, 1, 1, 0, SphereCylinderCapPointMaterial, true},
		{1, 1, 1, 0, SphereCylinderCapLens, false}, {0, 1, 1, 0, SphereCylinderCapCoincidentDisk, false},
		{0, 2, 1, 0, SphereCylinderCapCylinderDisk, false}, {0, 1, 2, 0, SphereCylinderCapSphereDisk, false},
		{1, 2, 1, 0, SphereCylinderCapCylinderDisk, true}, {1, 1, 2, 0, SphereCylinderCapSphereDisk, true},
		{.5, 3, 1, 0, SphereCylinderCapCylinderDisk, false}, {.5, 1, 3, 0, SphereCylinderCapSphereDisk, false},
		{.5, 1, 1, 1, SphereCylinderCapPointMaterial, true}, {2, 1, 1, 1, SphereCylinderCapEmpty, false}, {0, 1, 1, 2, SphereCylinderCapEmpty, false},
	} {
		q, e := ClassifySphereCylinderCapMaterial(ctx, Vec3{X: c.d}, c.R, Vec3{}, Vec3{Z: 1}, c.tool, new(big.Rat).SetFloat64(c.z), SurfacePatchEvaluationOptions{})
		if e != nil || !q.Complete || q.Kind != c.kind || q.Tangent != c.tangent {
			t.Fatal(c, q, e)
		}
	}
	// Unit lens: cylinder arc from negative-Y to positive-Y through +X,
	// sphere arc returns through the opposite side, both CCW/material-left.
	q, e := ClassifySphereCylinderCapMaterial(ctx, Vec3{X: 1}, 1, Vec3{}, Vec3{Z: 1}, 1, r(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Boundary) != 2 || q.RadicalAxisValue.Cmp(r(1, 2)) != 0 {
		t.Fatal(q, e)
	}
	if q.Boundary[0].StartEvent != 0 || q.Boundary[0].EndEvent != 1 || q.Boundary[1].StartEvent != 1 || q.Boundary[1].EndEvent != 0 || q.Events.Points[0].Bounds[1][1].Sign() >= 0 || q.Events.Points[1].Bounds[1][0].Sign() <= 0 {
		t.Fatal(q)
	}
	reversed, e := ClassifySphereCylinderCapMaterial(ctx, Vec3{X: 1}, 1, Vec3{}, Vec3{Z: -2}, 1, r(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil || reversed.Events.Points[0].Bounds[1][0].Sign() <= 0 {
		t.Fatal(reversed, e)
	}
	// Nonbinary cap maps exactly to z=1; section radius²3 stays rational.
	q, e = ClassifySphereCylinderCapMaterial(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 3}, 1, r(1, 3), SurfacePatchEvaluationOptions{})
	if e != nil || q.Kind != SphereCylinderCapLens || q.SphereRadiusSquared.Cmp(r(3, 1)) != 0 || q.CylinderCenter[2].Cmp(r(1, 1)) != 0 {
		t.Fatal(q, e)
	}
	q, e = ClassifySphereCylinderCapMaterial(ctx, Vec3{X: 1}, 1, Vec3{}, Vec3{Z: 1}, 1, r(0, 1), SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Boundary) != 0 {
		t.Fatal(q, e)
	}
}
