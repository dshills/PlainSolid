package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereVectorAxisFamilies(t *testing.T) {
	ctx := context.Background()
	s, e := NewSpherePairSet(ctx, Ball{Center: geom.Vec3{X: 2, Y: 1, Z: 7}, Radius: 5}, Ball{Center: geom.Vec3{X: 2, Y: 1, Z: 1}, Radius: 5}, Difference, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	g, e := s.NearestTransverseAxisBoundaryRational(ctx, [3]*big.Rat{r(2, 1), r(1, 1), r(1, 1)}, geom.SphereFullBoundaryDistanceOptions{})
	if e != nil || len(g.TiedFaces) != 2 || g.TiedFaces[0] == g.TiedFaces[1] || g.Geometry.Families[1].Kind != geom.SphereAxisRetainedCap || !g.Geometry.Families[1].Nearest || g.Geometry.MaterialNormalSigns[1] != -1 {
		t.Fatal(g, e)
	}
	if g.Geometry.Families[1].Normal[2].Cmp(r(-6, 1)) != 0 || g.Geometry.Families[1].CapSide != -1 {
		t.Fatal("native reversed source vector")
	}
}
