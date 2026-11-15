package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestTransverseLensRimPreservesObliqueSourceEquations(t *testing.T) {
	ctx := context.Background()
	s, err := NewSpherePairMaterial(Vec3{}, 4, Vec3{X: 3, Y: 4}, 3, SphereSetIntersection)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		r, err := s.TransverseLensRim(ctx, SurfacePatchOptions{})
		if err != nil || r.Center[0].Cmp(big.NewRat(48, 25)) != 0 || r.Center[1].Cmp(big.NewRat(64, 25)) != 0 || r.Center[2].Sign() != 0 ||
			r.Axis[0].Cmp(big.NewRat(3, 1)) != 0 || r.Axis[1].Cmp(big.NewRat(4, 1)) != 0 || r.AxisSquared.Cmp(big.NewRat(25, 1)) != 0 ||
			r.RadiusSquared.Cmp(big.NewRat(144, 25)) != 0 || r.CenterFraction.Cmp(big.NewRat(16, 25)) != 0 || r.NormalDot.Sign() != 0 {
			t.Fatalf("oblique rim=%+v %v", r, err)
		}
		r.Center[0].SetInt64(999)
		r.RadiusSquared.SetInt64(1)
	}
	if r, err := s.TransverseLensRim(ctx, SurfacePatchOptions{MaxOperations: 10}); !errors.Is(err, ErrConvergence) || r != (SphereLensRim{}) {
		t.Fatalf("unproved rim=%+v %v", r, err)
	}
	tangent, err := NewSpherePairMaterial(Vec3{}, 3, Vec3{Z: 7}, 4, SphereSetIntersection)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := tangent.TransverseLensRim(ctx, SurfacePatchOptions{}); !errors.Is(err, ErrDomain) || r != (SphereLensRim{}) {
		t.Fatalf("tangent rim=%+v %v", r, err)
	}
}
