package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSpherePairPlaneFirstHitRational(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	s, _ := NewSpherePairMaterial(Vec3{}, 1, Vec3{X: 20}, 1, SphereSetUnion)
	v, _ := s.PlaneMaterial(ctx, Vec3{}, Vec3{Y: 1, Z: -1}, SurfacePatchEvaluationOptions{})
	h, e := v.FirstBoundaryHitRationalUV(ctx, [2]*big.Rat{new(big.Rat), big.NewRat(1, 3)}, [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}, big.NewRat(4, 3), o)
	if e != nil || !h.HasHit || h.Tangent || h.Isolated || h.Boundary != SpherePairFirst || len(h.Normals) != 1 {
		t.Fatal(h, e)
	}
	q, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-7", "0", "9"}, Lower: "0", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cmp, e := h.Parameter.Compare(ctx, q, ExactCurvePlaneEventOptions{})
	if e != nil || !cmp.Proven || cmp.Order != 0 {
		t.Fatal(cmp, e)
	}
	if h.Normals[0].DirectionSign != 1 || h.Normals[0].Constant[1].Cmp(big.NewRat(2, 3)) != 0 || h.Normals[0].Constant[2].Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal(h.Normals)
	}
	// A short interval entirely inside material must not invent an endpoint hit.
	h, e = v.FirstBoundaryHitRationalUV(ctx, [2]*big.Rat{new(big.Rat), big.NewRat(1, 3)}, [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}, big.NewRat(1, 3), o)
	if e != nil || h.HasHit {
		t.Fatal(h, e)
	}
	// Boundary start directed outward: isolated in this finite ray, not tangent.
	h, e = v.FirstBoundaryHitRationalUV(ctx, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)}, [2]*big.Rat{big.NewRat(1, 3), new(big.Rat)}, big.NewRat(1, 3), o)
	if e != nil || !h.HasHit || !h.Isolated || h.Tangent || h.Bounds.Lower.Sign() != 0 {
		t.Fatal(h, e)
	}
}
func TestSpherePairPlaneFirstHitPoleAndCavity(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	s, _ := NewSpherePairMaterial(Vec3{}, 1, Vec3{X: 20}, 1, SphereSetUnion)
	v, _ := s.PlaneMaterial(ctx, Vec3{X: 3}, Vec3{X: 1, Y: 2, Z: 2}, SurfacePatchEvaluationOptions{})
	org := [2]*big.Rat{big.NewRat(-8, 3), big.NewRat(2, 3)}
	dir := [2]*big.Rat{big.NewRat(1, 3), new(big.Rat)}
	h, e := v.FirstBoundaryHitRationalUV(ctx, org, dir, big.NewRat(1, 3), o)
	if e != nil || !h.HasHit || !h.Isolated || !h.Tangent || h.Normals[0].DirectionSign != 0 {
		t.Fatal(h, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 1, SphereSetDifference)
	v, _ = s.PlaneMaterial(ctx, Vec3{}, Vec3{Y: 1, Z: -1}, SurfacePatchEvaluationOptions{})
	h, e = v.FirstBoundaryHitRationalUV(ctx, [2]*big.Rat{new(big.Rat), big.NewRat(1, 3)}, dir, big.NewRat(4, 1), o)
	if e != nil || !h.HasHit || h.Boundary != SpherePairSecond || h.Normals[0].DirectionSign != -1 || h.Normals[0].Constant[1].Cmp(big.NewRat(-2, 3)) != 0 {
		t.Fatal(h, e)
	}
	o.Surface.MaxOperations = 1
	h, e = v.FirstBoundaryHitRationalUV(ctx, org, dir, big.NewRat(1, 3), o)
	if e == nil || h.HasHit || h.Parameter != nil {
		t.Fatal("atomic", h, e)
	}
}
