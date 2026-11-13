package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSpherePairPlaneMaterialRationalPole(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchEvaluationOptions{}
	s, _ := NewSpherePairMaterial(Vec3{}, 1, Vec3{X: 20}, 1, SphereSetUnion)
	// x+2y+2z=3 is tangent to the unit sphere at (1/3,2/3,2/3).
	view, e := s.PlaneMaterial(ctx, Vec3{X: 3}, Vec3{X: 1, Y: 2, Z: 2}, o)
	if e != nil {
		t.Fatal(e)
	}
	uv := [2]*big.Rat{big.NewRat(-8, 3), big.NewRat(2, 3)}
	loc, mask, e := view.ClassifyRationalUV(ctx, uv, o)
	if e != nil || loc != BoreBoundary || mask != SpherePairFirst {
		t.Fatal(loc, mask, e)
	}
	rounded, _, e := s.Classify(Vec3{X: 1.0 / 3, Y: 2.0 / 3, Z: 2.0 / 3})
	if e != nil || rounded != BoreInside {
		t.Fatal("rounded point must disagree", rounded, e)
	}
	// Any nonzero rational displacement in a tangent plane is outside.
	uv[0].Add(uv[0], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	loc, mask, e = view.ClassifyRationalUV(ctx, uv, o)
	if e != nil || loc != BoreOutside || mask != 0 {
		t.Fatal(loc, mask, e)
	}
	uv[0] = big.NewRat(-8, 3)
	loc, mask, e = view.ClassifyRationalUV(ctx, uv, o)
	if e != nil || loc != BoreBoundary || mask != SpherePairFirst {
		t.Fatal("input not retained", loc, mask, e)
	}
}
func TestSpherePairPlaneMaterialLimitsAndHidden(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchEvaluationOptions{}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 1, SphereSetUnion)
	view, e := s.PlaneMaterial(ctx, Vec3{}, Vec3{Y: 1, Z: -1}, o)
	if e != nil {
		t.Fatal(e)
	}
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)}
	loc, mask, e := view.ClassifyRationalUV(ctx, uv, o)
	if e != nil || loc != BoreInside || mask != 0 {
		t.Fatal("hidden circle", loc, mask, e)
	}
	loc, mask, e = view.ClassifyRationalUV(ctx, uv, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || loc != BoreOutside || mask != 0 {
		t.Fatal("atomic", loc, mask, e)
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e = view.ClassifyRationalUV(c, uv, o); e != context.Canceled {
		t.Fatal(e)
	}
	if _, _, e = view.ClassifyRationalUV(ctx, [2]*big.Rat{}, o); e != ErrInvalid {
		t.Fatal(e)
	}
	var zero SpherePairPlaneMaterial
	if _, _, e = zero.ClassifyRationalUV(ctx, uv, o); e != ErrInvalid {
		t.Fatal(e)
	}
}
