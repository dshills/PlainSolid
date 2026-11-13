package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestIrrationalSphereBlindBore(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, floor := range []float64{-1, 0, 2} {
		r, e := SphereBlindBoreAlgebraic(context.Background(), f, 5, 2, floor, 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		lo, hi := r.CrossingInterval.Lower, r.CrossingInterval.Upper
		if new(big.Rat).Mul(lo, lo).Cmp(big.NewRat(21, 1)) > 0 || new(big.Rat).Mul(hi, hi).Cmp(big.NewRat(21, 1)) < 0 {
			t.Fatal("root bracket")
		}
		if r.SeamSquaredBound.Cmp(big.NewRat(1, 10000000000000000)) > 0 {
			t.Fatal("seam")
		}
		z := math.Sqrt(21)
		h := 5 - z
		want := 500.0/3 - 4*(z-floor) - h*h*(5-h/3)
		a, _ := r.VolumePiInterval[0].Float64()
		b, _ := r.VolumePiInterval[1].Float64()
		if want < a-1e-12 || want > b+1e-12 {
			t.Fatal("volume bounds", a, want, b)
		}
		mass, e := r.Solid.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-math.Pi*want) > 1e-6 {
			t.Fatal("native mass", e, mass)
		}
		if _, e := SphereBlindBore(context.Background(), f, 5, 2, floor, 1e-8); e == nil {
			t.Fatal("strict API weakened")
		}
	}
}
func TestAlgebraicBoreLimits(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	r, e := SphereBlindBoreAlgebraic(context.Background(), f, 5, 2, 0, 1e-8, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("partial")
	}
	for _, floor := range []float64{5, -5} {
		if _, e := SphereBlindBoreAlgebraic(context.Background(), f, 5, 2, floor, 1e-8, geom.ExactCurvePlaneEventOptions{}); e == nil {
			t.Fatal("floor")
		}
	}
}
