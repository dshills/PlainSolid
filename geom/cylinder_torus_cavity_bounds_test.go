package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCylinderTorusCavityAffineRange(t *testing.T) {
	r := big.NewRat
	q, _ := NewCylinderTorusCavityMaterial(5, 2, 8, -3, 4)
	m := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}
	v, e := q.BoundAffineRange(context.Background(), m, 64, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lows := []*big.Rat{r(-71, 10), r(-42, 5), r(-313, 40)}
	highs := []*big.Rat{r(81, 10), r(38, 5), r(367, 40)}
	for i := 0; i < 3; i++ {
		if v.Minimum[i][0].Cmp(lows[i]) != 0 || v.Minimum[i][1].Cmp(lows[i]) != 0 || v.Maximum[i][0].Cmp(highs[i]) != 0 || v.Maximum[i][1].Cmp(highs[i]) != 0 {
			t.Fatal(i, v.Minimum[i], v.Maximum[i])
		}
		if v.Extrema[i][0].Owners&TorusCylinderTorus != 0 {
			t.Fatal("buried")
		}
	}
	m[0][0] = r(1, 1)
	m[0][1] = r(1, 1)
	v, e = q.BoundAffineRange(context.Background(), m, 64, SurfacePatchOptions{})
	if e != nil || v.Minimum[0][0].Cmp(v.Minimum[0][1]) >= 0 || v.Extrema[0][0].RadialSquared.Cmp(r(2, 1)) != 0 {
		t.Fatal("irrational", e)
	}
	if _, e = q.BoundAffineRange(context.Background(), m, 64, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
	m[2] = m[1]
	if _, e = q.BoundAffineRange(context.Background(), m, 64, SurfacePatchOptions{}); e == nil {
		t.Fatal("singular")
	}
}
