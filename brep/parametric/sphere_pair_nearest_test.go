package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSpherePairNearestDispatch(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		a, b   Ball
		op     BooleanOp
		p      [3]*big.Rat
		kind   geom.SphereNearestKind
		owners int
	}{
		{Ball{Radius: 2}, Ball{Center: geom.Vec3{X: 6}, Radius: 1}, Union, [3]*big.Rat{r(7, 2), r(0, 1), r(0, 1)}, geom.SphereNearestSeparatedUnion, 2},
		{Ball{Radius: 5}, Ball{Center: geom.Vec3{X: 1}, Radius: 1}, Difference, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, geom.SphereNearestCavity, 1},
		{Ball{Center: geom.Vec3{X: -1}, Radius: 2}, Ball{Center: geom.Vec3{X: 1}, Radius: 2}, Union, [3]*big.Rat{r(0, 1), r(1, 1), r(1, 1)}, geom.SphereNearestTransverseIsolated, 2},
		{Ball{Center: geom.Vec3{X: -3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3}, Radius: 5}, Intersection, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, geom.SphereNearestTransverseAxis, 2},
	} {
		s, e := NewSpherePairSet(ctx, tc.a, tc.b, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		g, e := s.NearestBoundaryRational(ctx, tc.p, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Geometry.Kind != tc.kind || len(g.TiedFaces) != tc.owners || len(g.TiedFaces)+len(g.ExcludedFaces) != 2 {
			t.Fatal(g, e)
		}
	}
}
