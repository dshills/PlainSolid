package geom

import (
	"context"
	"errors"
	"math"
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

func TestEncloseTransverseLensRimSameSideAndIrrationalLatitudes(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		a, b, d float64 // radii and positive axial separation
	}{
		{2, 2, 2},
		{5, 2, 4},            // both latitude angles are positive: radical plane beyond B
		{2, 5, 4},            // both latitude angles are negative
		{3, 5, 4},            // one latitude is exactly zero
		{3, 2, 4.9990234375}, // near tangency exercises reciprocal angle reduction
	} {
		s, err := NewSpherePairMaterial(Vec3{}, tc.a, Vec3{Y: -tc.d}, tc.b, SphereSetIntersection)
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.EncloseTransverseLensRim(ctx, SphereLensMeasureOptions{PrecisionBits: 42})
		if err != nil || r.Work == 0 {
			t.Fatalf("%+v: %+v %v", tc, r, err)
		}
		axial := (tc.a*tc.a - tc.b*tc.b + tc.d*tc.d) / (2 * tc.d)
		radius := math.Sqrt(tc.a*tc.a - axial*axial)
		for i, iv := range []ExactParameterInterval{r.Radius, r.Latitude[0], r.Latitude[1]} {
			lo, _ := iv.Lower.Float64()
			hi, _ := iv.Upper.Float64()
			want := [3]float64{radius, math.Atan2(axial, radius), math.Atan2(axial-tc.d, radius)}[i]
			if math.Abs((lo+hi)/2-want) > 2e-13 || new(big.Rat).Sub(iv.Upper, iv.Lower).Cmp(lensMeasurePowerOfTwo(42)) > 0 {
				t.Fatalf("case %+v term %d enclosure [%g,%g] should bound %g", tc, i, lo, hi, want)
			}
		}
		if r.Latitude[0].Lower.Cmp(r.Latitude[1].Upper) <= 0 {
			t.Fatal("original seam order was not proved")
		}
	}
}
