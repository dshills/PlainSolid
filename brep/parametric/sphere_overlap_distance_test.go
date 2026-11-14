package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereOverlapNearest(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op   BooleanOp
		x, y *big.Rat
	}{{Union, r(0, 1), r(9, 4)}, {Intersection, r(0, 1), r(36, 5)}, {Difference, r(3, 1), r(9, 2)}} {
		s, e := NewSpherePairSet(ctx, Ball{Center: geom.Vec3{X: -3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3}, Radius: 5}, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		g, e := s.NearestOverlapBoundaryRational(ctx, [3]*big.Rat{tc.x, tc.y, r(0, 1)}, geom.SurfacePatchOptions{})
		if e != nil || len(g.TiedFaces) != 2 || g.TiedFaces[0] == g.TiedFaces[1] || !g.Geometry.Candidates[2].Nearest {
			t.Fatal(tc.op, g, e)
		}
		owners := s.SourceFaces()
		f := s.Solid().Faces()
		if f[owners[0][0]].Reversed || f[owners[1][0]].Reversed != (tc.op == Difference) {
			t.Fatal("stock/tool material orientation")
		}
	}
}
