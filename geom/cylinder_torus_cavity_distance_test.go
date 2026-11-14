package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCylinderTorusCavityDistance(t *testing.T) {
	q, _ := NewCylinderTorusCavityMaterial(5, 2, 8, -3, 4)
	r := big.NewRat
	ctx := context.Background()
	o := TorusCylinderDistanceOptions{SquaredTolerance: r(1, 100)}
	for _, tc := range []struct {
		x, z, d *big.Rat
		owner   BoreBoundaryMask
	}{{r(15, 2), r(0, 1), r(1, 4), TorusCylinderTorus | TorusCylinderWall}, {r(0, 1), r(5, 1), r(1, 1), TorusCylinderUpperCap}, {r(9, 1), r(5, 1), r(2, 1), TorusCylinderWall | TorusCylinderUpperCap}, {r(5, 1), r(0, 1), r(4, 1), TorusCylinderTorus}} {
		v, e := q.BoundBoundaryDistance(ctx, [3]*big.Rat{tc.x, r(0, 1), tc.z}, o)
		if e != nil || v.Lower.Cmp(tc.d) > 0 || v.Upper.Cmp(tc.d) < 0 || v.PossibleOwners != tc.owner {
			t.Fatal(v, e, tc)
		}
		if v.ExactOwners != tc.owner {
			t.Fatal("exact owner tie", v.ExactOwners, tc.owner)
		}
	}
	o.MaxCells = 1
	if _, e := q.BoundBoundaryDistance(ctx, [3]*big.Rat{r(1, 1), r(1, 1), r(1, 1)}, o); e == nil {
		t.Fatal("budget")
	}
}
