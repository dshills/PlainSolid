package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSphereCylinderCapExact(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	q, e := IntersectSphereCylinderCap(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 3}, 1, r(1, 3), SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || q.EntireCircle || len(q.Points) != 2 {
		t.Fatal(q, e)
	}
	for _, p := range q.Points {
		if p.CapParameter.Cmp(r(1, 3)) != 0 || p.Base[0].Cmp(r(-1, 2)) != 0 || p.Base[2].Cmp(r(1, 1)) != 0 || p.Radicand.Cmp(r(3, 4)) != 0 {
			t.Fatal(p)
		}
		// Independent exact sphere equation for B±sqrt(D)V: constant4,
		// radical coefficient0. Cylinder radial squared similarly equals1.
		constant, cross, radial := new(big.Rat), new(big.Rat), new(big.Rat)
		for i := 0; i < 3; i++ {
			delta := new(big.Rat).Set(p.Base[i])
			if i == 0 {
				delta.Sub(delta, r(1, 1))
			}
			term := new(big.Rat).Add(new(big.Rat).Mul(delta, delta), new(big.Rat).Mul(p.Radicand, new(big.Rat).Mul(p.Vector[i], p.Vector[i])))
			constant.Add(constant, term)
			cross.Add(cross, new(big.Rat).Mul(delta, p.Vector[i]))
			if i < 2 {
				radial.Add(radial, new(big.Rat).Add(new(big.Rat).Mul(p.Base[i], p.Base[i]), new(big.Rat).Mul(p.Radicand, new(big.Rat).Mul(p.Vector[i], p.Vector[i]))))
			}
		}
		if constant.Cmp(r(4, 1)) != 0 || cross.Sign() != 0 || radial.Cmp(r(1, 1)) != 0 {
			t.Fatal(p)
		}
	}
	q, e = IntersectSphereCylinderCap(ctx, Vec3{X: 1}, 1, Vec3{}, Vec3{Z: -2}, 1, r(-1, 2), SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Points) != 1 || !q.Points[0].Tangent || q.Points[0].Base[0].Cmp(r(1, 1)) != 0 {
		t.Fatal(q, e)
	}
	q, e = IntersectSphereCylinderCap(ctx, Vec3{}, 5, Vec3{}, Vec3{Z: 3}, 3, r(4, 3), SurfacePatchEvaluationOptions{})
	if e != nil || !q.EntireCircle || !q.Complete || len(q.Points) != 0 {
		t.Fatal(q, e)
	}
	q, e = IntersectSphereCylinderCap(ctx, Vec3{}, 5, Vec3{}, Vec3{Z: 3}, 3, r(5, 3), SurfacePatchEvaluationOptions{})
	if e != nil || q.EntireCircle || len(q.Points) != 0 || !q.Complete {
		t.Fatal(q, e)
	}
	q, e = IntersectSphereCylinderCap(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 3}, 1, r(1, 3), SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || q.EntireCircle || len(q.Points) != 0 {
		t.Fatal(q, e)
	}
}
