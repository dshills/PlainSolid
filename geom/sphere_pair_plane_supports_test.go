package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSpherePairPlaneSupportsOblique(t *testing.T) {
	s, _ := NewSpherePairMaterial(Vec3{X: 1}, 2, Vec3{Z: 2}, 1, SphereSetDifference)
	r, e := s.PlaneSupports(context.Background(), Vec3{}, Vec3{X: 1, Y: 1, Z: 1}, SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Supports[0].RadiusSquared.Cmp(big.NewRat(11, 3)) != 0 || r.Supports[1].RadiusSquared.Cmp(big.NewRat(-1, 3)) != 0 || r.Supports[1].Sign != -1 {
		t.Fatal(r.Supports)
	}
	if r.Gram[0][0].Cmp(big.NewRat(2, 1)) != 0 || r.Gram[0][1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r.Gram)
	}
	for _, v := range r.Supports {
		residual := new(big.Rat)
		for j := 0; j < 3; j++ {
			p := new(big.Rat).Set(r.Origin[j])
			for k := 0; k < 2; k++ {
				p.Add(p, new(big.Rat).Mul(r.Basis[k][j], v.Center[k]))
			}
			if p.Cmp(v.WorldCenter[j]) != 0 {
				t.Fatal("chart reconstruction")
			}
			residual.Add(residual, new(big.Rat).Mul(r.Normal[j], new(big.Rat).Sub(p, r.Origin[j])))
		}
		if residual.Sign() != 0 {
			t.Fatal("plane projection")
		}
	}
	// Raw normal reversal/scaling preserves centers/radii and scales residual.
	q, e := s.PlaneSupports(context.Background(), Vec3{}, Vec3{X: -3, Y: -3, Z: -3}, SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if q.Supports[i].RadiusSquared.Cmp(r.Supports[i].RadiusSquared) != 0 || q.Supports[i].SignedCenterResidual.Cmp(new(big.Rat).Mul(big.NewRat(-3, 1), r.Supports[i].SignedCenterResidual)) != 0 {
			t.Fatal("scale")
		}
	}
	r.Supports[0].RadiusSquared.SetInt64(999)
	fresh, _ := s.PlaneSupports(context.Background(), Vec3{}, Vec3{X: 1, Y: 1, Z: 1}, SurfacePatchEvaluationOptions{})
	if fresh.Supports[0].RadiusSquared.Cmp(big.NewRat(11, 3)) != 0 {
		t.Fatal("alias")
	}
}
func TestSpherePairPlaneSupportsContactAndLimits(t *testing.T) {
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 20}, 2, SphereSetUnion)
	for _, h := range []float64{math.Nextafter(5, 0), 5, math.Nextafter(5, 6)} {
		r, e := s.PlaneSupports(context.Background(), Vec3{Z: h}, Vec3{Z: 7}, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := 0
		if h < 5 {
			want = 1
		} else if h > 5 {
			want = -1
		}
		if r.Supports[0].Sign != want {
			t.Fatal(h, r.Supports[0])
		}
	}
	for _, o := range []SurfacePatchEvaluationOptions{{MaxOperations: 1}, {MaxCoefficientBits: 1}} {
		r, e := s.PlaneSupports(context.Background(), Vec3{}, Vec3{X: 1, Y: 2, Z: 3}, o)
		if e == nil || r.NormalSquared != nil {
			t.Fatal("atomic limit", r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := s.PlaneSupports(ctx, Vec3{}, Vec3{Z: 1}, SurfacePatchEvaluationOptions{}); e != context.Canceled || r.NormalSquared != nil {
		t.Fatal(r, e)
	}
	if _, e := s.PlaneSupports(context.Background(), Vec3{}, Vec3{}, SurfacePatchEvaluationOptions{}); e != ErrDegenerate {
		t.Fatal(e)
	}
}
