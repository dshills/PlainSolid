package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestBoundSpherePairPlaneMaterial(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchEvaluationOptions{}
	s, e := NewSpherePairSet(ctx, Ball{Radius: 5}, Ball{Radius: 1}, Difference, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.PlaneMaterial(ctx, geom.Vec3{}, geom.Vec3{Y: 1, Z: -1}, o)
	if e != nil {
		t.Fatal(e)
	}
	loc, mask, e := v.ClassifyRationalUV(ctx, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)}, o)
	if e != nil || loc != geom.BoreBoundary || mask != geom.SpherePairSecond {
		t.Fatal(loc, mask, e)
	}
	var zero SpherePairSet
	if _, e = zero.PlaneMaterial(ctx, geom.Vec3{}, geom.Vec3{Z: 1}, o); e != geom.ErrInvalid {
		t.Fatal(e)
	}
}
