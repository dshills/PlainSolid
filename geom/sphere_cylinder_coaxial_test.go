package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestCoaxialSphereCylinderExact(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	q, e := IntersectCoaxialSphereCylinder(ctx, Vec3{Z: 2}, 2, Vec3{}, Vec3{Z: 2}, 1, nil, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || len(q.Circles) != 2 {
		t.Fatal(q, e)
	}
	for i, c := range q.Circles {
		if !c.Transverse || c.Tangent || c.RootSign != 2*i-1 || c.ParameterOffset.Cmp(r(1, 1)) != 0 || c.ParameterRadicand.Cmp(r(3, 4)) != 0 {
			t.Fatal(c)
		}
		// Independently bound sqrt(3): lower²<=3<=upper² in model Z.
		lo := new(big.Rat).Sub(c.CenterBounds[2][0], r(2, 1))
		hi := new(big.Rat).Sub(c.CenterBounds[2][1], r(2, 1))
		if i == 0 {
			lo, hi = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo)
		}
		if new(big.Rat).Mul(lo, lo).Cmp(r(3, 1)) > 0 || new(big.Rat).Mul(hi, hi).Cmp(r(3, 1)) < 0 {
			t.Fatal(c.CenterBounds)
		}
	}
	tangent, e := IntersectCoaxialSphereCylinder(ctx, Vec3{Z: 2}, 1, Vec3{}, Vec3{Z: -2}, 1, nil, SurfacePatchEvaluationOptions{})
	if e != nil || len(tangent.Circles) != 1 || !tangent.Circles[0].Tangent || tangent.Circles[0].ParameterOffset.Cmp(r(-1, 1)) != 0 {
		t.Fatal(tangent, e)
	}
	// 3-4-5 geometry: raw axis scale3 gives exact t=±4/3.
	window := [2]*big.Rat{r(4, 3), r(4, 3)}
	q, e = IntersectCoaxialSphereCylinder(ctx, Vec3{}, 5, Vec3{}, Vec3{X: 3}, 3, &window, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Circles) != 1 || q.Circles[0].ParameterBounds[0].Cmp(r(4, 3)) != 0 {
		t.Fatal(q, e)
	}
	window = [2]*big.Rat{r(-1, 2), r(1, 2)}
	q, e = IntersectCoaxialSphereCylinder(ctx, Vec3{}, 2, Vec3{}, Vec3{Z: 1}, 1, &window, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Circles) != 0 || !q.Complete {
		t.Fatal(q, e)
	}
	q, e = IntersectCoaxialSphereCylinder(ctx, Vec3{X: math.SmallestNonzeroFloat64}, 2, Vec3{}, Vec3{Z: 1}, 1, nil, SurfacePatchEvaluationOptions{})
	if e != ErrDomain || q.Complete {
		t.Fatal(q, e)
	}
	// Exact oblique raw axis, without normalized 3/5 and4/5 rounding.
	q, e = IntersectCoaxialSphereCylinder(ctx, Vec3{X: 6, Y: 8}, 5, Vec3{}, Vec3{X: 3, Y: 4}, 3, nil, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Circles) != 2 || q.Circles[1].ParameterBounds[0].Cmp(r(14, 5)) != 0 {
		t.Fatal(q, e)
	}
	q, e = IntersectCoaxialSphereCylinder(ctx, Vec3{}, 1, Vec3{}, Vec3{Z: 1}, 2, nil, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || len(q.Circles) != 0 {
		t.Fatal(q, e)
	}
	q, e = IntersectCoaxialSphereCylinder(ctx, Vec3{}, 5, Vec3{}, Vec3{Z: 1}, 3, nil, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Circles) != 0 {
		t.Fatal(q, e)
	}
}
