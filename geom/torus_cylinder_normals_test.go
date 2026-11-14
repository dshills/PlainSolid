package geom

import (
	"context"
	"math/big"
	"reflect"
	"testing"
)

func TestTorusCylinderNormals(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, op := range []TorusCylinderOperation{TorusCylinderCommon, TorusCylinderDifference, TorusCylinderUnion} {
		s, _ := NewTorusCylinderMaterial(5, 2, 5, -3, 4, op)
		p := [3]*big.Rat{r(5, 1), r(0, 1), r(2, 1)}
		v, e := s.BoundaryNormalsRational(ctx, p, SurfacePatchOptions{})
		if e != nil || v.Owners != 3 {
			t.Fatal(v, e)
		}
		if v.Directions[0][0].Sign() != 0 || v.Directions[0][1].Sign() != 0 || v.Directions[0][2].Cmp(r(100, 1)) != 0 {
			t.Fatal("top torus normal", v)
		}
		sign := int64(5)
		if op == TorusCylinderDifference {
			sign = -5
		}
		if v.Directions[1][0].Cmp(r(sign, 1)) != 0 {
			t.Fatal("wall sign")
		}
		fail, e := s.BoundaryNormalsRational(ctx, p, SurfacePatchOptions{MaxOperations: v.ExactOperations - 1})
		if e == nil || !reflect.DeepEqual(fail, TorusCylinderNormals{}) {
			t.Fatal("budget", fail, e)
		}
	}
	s, _ := NewTorusCylinderMaterial(5, 2, 6, -3, 4, TorusCylinderUnion)
	if _, e := s.BoundaryNormalsRational(ctx, [3]*big.Rat{r(6, 1), r(0, 1), r(0, 1)}, SurfacePatchOptions{}); e == nil {
		t.Fatal("buried wall")
	}
	v, e := s.BoundaryNormalsRational(ctx, [3]*big.Rat{r(6, 1), r(0, 1), r(4, 1)}, SurfacePatchOptions{})
	if e != nil || v.Owners != 10 || v.Directions[3][2].Cmp(r(1, 1)) != 0 {
		t.Fatal(v, e)
	}
}
