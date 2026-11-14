package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereTransverseNearest(t *testing.T) {
	ctx := context.Background()
	s, e := NewSpherePairSet(ctx, Ball{Center: geom.Vec3{X: -1}, Radius: 2}, Ball{Center: geom.Vec3{X: 1}, Radius: 2}, Union, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	p := [3]*big.Rat{r(0, 1), r(1, 1), r(1, 1)}
	g, e := s.NearestTransverseBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{})
	if e != nil || len(g.TiedFaces) != 2 || g.TiedFaces[0] == g.TiedFaces[1] || !g.Geometry.Candidates[2].Nearest {
		t.Fatal(g, e)
	}
	w := g.Geometry.Candidates[2]
	if w.Radicand.Cmp(r(6, 1)) != 0 || w.Constant.Cmp(r(5, 1)) != 0 || w.Coefficient.Cmp(r(2, 1)) != 0 || w.Numerator[1].Cmp(r(3, 1)) != 0 || w.Numerator[2].Cmp(r(3, 1)) != 0 {
		t.Fatal("exact irrational radius foot", w)
	}
	if _, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{}); e == nil {
		t.Fatal("legacy gate")
	}
}
