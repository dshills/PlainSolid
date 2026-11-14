package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereOverlapAxisFamilies(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op   BooleanOp
		x    int64
		kind geom.SphereAxisMinimumKind
		slot int
	}{{Union, -3, geom.SphereAxisRetainedCap, 0}, {Union, 0, geom.SphereAxisSharedCircle, 2}, {Intersection, 0, geom.SphereAxisPole, 0}, {Difference, 3, geom.SphereAxisRetainedCap, 1}} {
		s, e := NewSpherePairSet(ctx, Ball{Center: geom.Vec3{X: -3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3}, Radius: 5}, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		g, e := s.NearestOverlapAxisBoundaryRational(ctx, [3]*big.Rat{r(tc.x, 1), r(0, 1), r(0, 1)}, geom.SurfacePatchOptions{})
		if e != nil || len(g.TiedFaces) != 2 || g.TiedFaces[0] == g.TiedFaces[1] || g.Geometry.Families[tc.slot].Kind != tc.kind || !g.Geometry.Families[tc.slot].Nearest {
			t.Fatal(g, e)
		}
		f := s.SourceFaces()
		faces := s.Solid().Faces()
		if faces[f[0][0]].Reversed || faces[f[1][0]].Reversed != (tc.op == Difference) {
			t.Fatal("native orientation")
		}
	}
}
