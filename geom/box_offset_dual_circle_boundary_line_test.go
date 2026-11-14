package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestBoxOffsetDualCircleBoundaryLine(t *testing.T) {
	q := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	min, max := [3]*big.Rat{q(0, 1), q(0, 1), q(0, 1)}, [3]*big.Rat{q(40, 1), q(10, 1), q(6, 1)}
	xs, rs := [2]*big.Rat{q(25, 2), q(22, 1)}, [2]*big.Rat{q(65, 8), q(87, 8)}
	org, dir := [3]*big.Rat{q(14, 1), q(0, 1), q(1, 1)}, [3]*big.Rat{q(0, 1), q(3, 1), q(0, 1)}
	opts := ToroidLineOptions{ParameterTolerance: 1e-10}
	r, e := IntersectBoxOffsetDualCircleBoundaryRational(context.Background(), min, max, xs, rs, q(-15, 2), q(3, 1), org, dir, q(0, 1), q(1, 1), opts)
	if e != nil || len(r.Hits) != 1 || r.Hits[0].Support != BoxDualCircleFirstWall || r.Hits[0].Bounds.Lower.Cmp(r.Hits[0].Bounds.Upper) == 0 {
		t.Fatal("irrational offset", r, e)
	}
	for _, cy := range []*big.Rat{nil, q(1, 1), q(-100, 1)} {
		r, e = IntersectBoxOffsetDualCircleBoundaryRational(context.Background(), min, max, xs, rs, cy, q(3, 1), org, dir, q(0, 1), q(1, 1), opts)
		if e == nil || len(r.Hits) != 0 {
			t.Fatal("invalid center", r, e)
		}
	}
	opts.Surface.MaxOperations = 1
	r, e = IntersectBoxOffsetDualCircleBoundaryRational(context.Background(), min, max, xs, rs, q(-15, 2), q(3, 1), org, dir, q(0, 1), q(1, 1), opts)
	if e == nil || len(r.Hits) != 0 {
		t.Fatal("budget", r, e)
	}
}
