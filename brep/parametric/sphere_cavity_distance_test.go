package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereCavityNearestOwners(t *testing.T) {
	ctx := context.Background()
	s, e := NewSpherePairSet(ctx, Ball{Center: geom.Vec3{X: 2, Y: 3, Z: 4}, Radius: 5}, Ball{Center: geom.Vec3{X: 3, Y: 3, Z: 4}, Radius: 1}, Difference, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	p := [3]*big.Rat{r(11, 2), r(3, 1), r(4, 1)}
	got, e := s.NearestCavityBoundaryRational(ctx, p, geom.SphereCavityDistanceOptions{})
	if e != nil || len(got.TiedFaces) != 2 || len(got.ExcludedFaces) != 0 || got.Geometry.Lower.Cmp(r(9, 4)) != 0 {
		t.Fatal(got, e)
	}
	faces := s.SourceFaces()
	if got.TiedFaces[0] == got.TiedFaces[1] {
		t.Fatal("merged distinct sphere owners")
	}
	native := s.Solid().Faces()
	if native[faces[0][0]].Reversed || !native[faces[1][0]].Reversed {
		t.Fatal("cavity material orientation")
	}
	mass, e := s.Solid().MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Volume-496*math.Pi/3) > 1e-6 {
		t.Fatal("independent cavity volume", mass, e)
	}
	got.TiedFaces[0] = 999
	again, e := s.NearestCavityBoundaryRational(ctx, p, geom.SphereCavityDistanceOptions{})
	if e != nil || again.TiedFaces[0] == 999 {
		t.Fatal("immutable native owners")
	}
}
