package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereOverlapAlgebraicNearest(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op      BooleanOp
		x, y, z int64
	}{{Union, 0, 2, 1}, {Intersection, 0, 7, 1}, {Difference, 3, 4, 1}} {
		s, e := NewSpherePairSet(ctx, Ball{Center: geom.Vec3{X: -3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3}, Radius: 5}, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		g, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, [3]*big.Rat{r(tc.x, 1), r(tc.y, 1), r(tc.z, 1)}, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || len(g.TiedFaces) != 2 || g.TiedFaces[0] == g.TiedFaces[1] || !g.Geometry.Candidates[2].Nearest {
			t.Fatal(g, e)
		}
		f := s.SourceFaces()
		faces := s.Solid().Faces()
		if faces[f[0][0]].Reversed || faces[f[1][0]].Reversed != (tc.op == Difference) {
			t.Fatal("native material normals")
		}
	}
}
