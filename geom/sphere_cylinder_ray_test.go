package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSphereCylinderRationalRay(t *testing.T) {
	r := big.NewRat
	ray := [3]*big.Rat{r(3, 5), r(4, 5), r(0, 1)}
	q, e := IntersectSphereCylinderRationalRay(context.Background(), Vec3{X: 1, Z: 2}, 2, Vec3{}, Vec3{Z: 2}, 1, ray, nil, SurfacePatchEvaluationOptions{})
	// Radial distance²=(1-3/5)²+(4/5)²=4/5; t=1±sqrt(4/5).
	if e != nil || !q.Complete || len(q.Contacts) != 2 || q.Contacts[0].Radicand.Cmp(r(4, 5)) != 0 {
		t.Fatal(q, e)
	}
	for _, p := range q.Contacts {
		if p.PointBounds[0][0].Cmp(r(3, 5)) != 0 || p.PointBounds[1][1].Cmp(r(4, 5)) != 0 || p.MeridianTangent {
			t.Fatal(p)
		}
	}
	xray := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	// Center displaced tangentially to cylinder: generator double contact,
	// but sphere and cylinder normals are perpendicular, not tangent.
	q, e = IntersectSphereCylinderRationalRay(context.Background(), Vec3{X: 1, Y: 1}, 1, Vec3{}, Vec3{Z: 1}, 1, xray, nil, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Contacts) != 1 || !q.Contacts[0].MeridianTangent || q.Contacts[0].SurfaceTangent {
		t.Fatal(q, e)
	}
	q, e = IntersectSphereCylinderRationalRay(context.Background(), Vec3{X: 2}, 1, Vec3{}, Vec3{Z: 1}, 1, xray, nil, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Contacts[0].SurfaceTangent {
		t.Fatal(q, e)
	}
	w := [2]*big.Rat{r(4, 3), r(4, 3)}
	q, e = IntersectSphereCylinderRationalRay(context.Background(), Vec3{}, 5, Vec3{}, Vec3{Z: 3}, 3, xray, &w, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Contacts) != 1 || q.Contacts[0].ParameterBounds[0].Cmp(r(4, 3)) != 0 {
		t.Fatal(q, e)
	}
	q, e = IntersectSphereCylinderRationalRay(context.Background(), Vec3{X: -5}, 1, Vec3{}, Vec3{Z: 1}, 1, xray, nil, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Contacts) != 0 || !q.Complete {
		t.Fatal(q, e)
	}
	q, e = IntersectSphereCylinderRationalRay(context.Background(), Vec3{}, 2, Vec3{}, Vec3{Z: 1}, 1, ray, nil, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Contacts) != 0 {
		t.Fatal(q, e)
	}
}
