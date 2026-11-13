package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSpherePairHorizontalSection(t *testing.T) {
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetIntersection)
	r, e := s.HorizontalSection(context.Background(), 1, SurfacePatchEvaluationOptions{})
	if e != nil || r.CoincidentCircle || len(r.Contacts) != 2 || r.Supports[0].RadiusSquared.Cmp(big.NewRat(24, 1)) != 0 || r.Supports[1].RadiusSquared.Cmp(big.NewRat(8, 1)) != 0 {
		t.Fatal(r, e)
	}
	for i, p := range r.Contacts {
		if p.Sign != 2*i-1 || p.Base[0].Cmp(big.NewRat(4, 1)) != 0 || p.Bounds[2].Lower.Cmp(big.NewRat(1, 1)) != 0 || p.Location != BoreBoundary || p.Boundary != SpherePairFirst|SpherePairSecond {
			t.Fatal(p)
		}
		// x=4, z=1 gives y²=8 on each sphere. Verify the exact radical identity.
		square := new(big.Rat).Mul(p.Vector[1], p.Vector[1])
		square.Mul(square, p.Radicand)
		if square.Cmp(big.NewRat(8, 1)) != 0 {
			t.Fatal("independent support residual", p)
		}
	}
	r, e = s.HorizontalSection(context.Background(), 3, SurfacePatchEvaluationOptions{})
	if e != nil || len(r.Contacts) != 1 || r.Contacts[0].Base[0].Cmp(big.NewRat(4, 1)) != 0 || r.Contacts[0].Incidence != SpherePairFirst|SpherePairSecond {
		t.Fatal("isolated pole on other circle", r, e)
	}
	r.Supports[0].Center[0].SetInt64(99)
	r, e = s.HorizontalSection(context.Background(), 1, SurfacePatchEvaluationOptions{})
	if e != nil || r.Supports[0].Center[0].Sign() != 0 {
		t.Fatal("aliased source", r, e)
	}
}
func TestSpherePairSectionSingularAndCoincident(t *testing.T) {
	ctx := context.Background()
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 8}, 3, SphereSetIntersection)
	r, e := s.HorizontalSection(ctx, 0, SurfacePatchEvaluationOptions{})
	if e != nil || len(r.Contacts) != 1 || r.Contacts[0].Radicand.Sign() != 0 || r.Contacts[0].Location != BoreOutside || r.Contacts[0].Boundary != 0 || r.Contacts[0].Incidence != SpherePairFirst|SpherePairSecond {
		t.Fatal("empty material but exact tangent support", r, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{Z: -1}, 5, Vec3{Z: 1}, 5, SphereSetUnion)
	r, e = s.HorizontalSection(ctx, 0, SurfacePatchEvaluationOptions{})
	if e != nil || !r.CoincidentCircle || r.CoincidentLocation != BoreBoundary || r.CoincidentBoundary != SpherePairFirst|SpherePairSecond || r.Supports[0].RadiusSquared.Cmp(big.NewRat(24, 1)) != 0 {
		t.Fatal("common irrational radius circle", r, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 5, SphereSetDifference)
	r, e = s.HorizontalSection(ctx, 0, SurfacePatchEvaluationOptions{})
	if e != nil || !r.CoincidentCircle || r.CoincidentLocation != BoreOutside {
		t.Fatal("coincident removed support", r, e)
	}
	r, e = s.HorizontalSection(ctx, 5, SurfacePatchEvaluationOptions{})
	if e != nil || len(r.Contacts) != 1 || r.Contacts[0].Location != BoreOutside {
		t.Fatal("coincident removed poles", r, e)
	}
	r, e = s.HorizontalSection(ctx, 6, SurfacePatchEvaluationOptions{})
	if e != nil || len(r.Contacts) != 0 || r.Supports[0].RadiusSquared.Sign() >= 0 {
		t.Fatal("empty support sections", r, e)
	}
}
func TestSpherePairSectionAtomicLimits(t *testing.T) {
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetUnion)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := s.HorizontalSection(ctx, 0, SurfacePatchEvaluationOptions{})
	if e != context.Canceled || r.Height != nil || len(r.Contacts) != 0 {
		t.Fatal(r, e)
	}
	r, e = s.HorizontalSection(context.Background(), 1, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || r.Height != nil || len(r.Contacts) != 0 {
		t.Fatal("atomic budget", r, e)
	}
}
