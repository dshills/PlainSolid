package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereSingleNearest(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		b     Ball
		ra    float64
		op    BooleanOp
		owner geom.BoreBoundaryMask
		x     int64
	}{
		{Ball{Center: geom.Vec3{X: 1}, Radius: 1}, 5, Union, 1, 5},
		{Ball{Center: geom.Vec3{X: 4}, Radius: 1}, 5, Union, 1, 5},
		{Ball{Center: geom.Vec3{X: 4}, Radius: 1}, 5, Intersection, 2, 5},
		{Ball{Radius: 5}, 5, Union, 3, 5},
		{Ball{Radius: 5}, 5, Intersection, 3, 0},
		{Ball{Center: geom.Vec3{X: 4}, Radius: 2}, 2, Difference, 1, 2},
	} {
		s, e := NewSpherePairSet(ctx, Ball{Radius: tc.ra}, tc.b, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		g, e := s.NearestBoundaryRational(ctx, [3]*big.Rat{r(tc.x, 1), r(0, 1), r(0, 1)}, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Geometry.Kind != geom.SphereNearestSingle || g.Geometry.Owners != tc.owner || len(g.TiedFaces) != 1 || len(g.ExcludedFaces) != 0 {
			t.Fatal("native reduced/aliased owners", g, e)
		}
		if tc.owner == 3 {
			ids := s.SourceFaces()
			if ids[0][0] != ids[1][0] || g.TiedFaces[0] != ids[0][0] {
				t.Fatal("coincident native face identity")
			}
		}
	}
}
