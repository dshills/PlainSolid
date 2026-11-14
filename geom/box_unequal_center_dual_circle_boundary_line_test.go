package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestBoxUnequalCenterDualCircleQueries(t *testing.T) {
	ctx := context.Background()
	q := func(n int64) *big.Rat { return big.NewRat(n, 1) }
	lo, hi := [3]*big.Rat{q(0), q(0), q(0)}, [3]*big.Rat{q(60), q(10), q(6)}
	xs, ys, rs := [2]*big.Rat{q(7), q(30)}, [2]*big.Rat{q(-3), q(-20)}, [2]*big.Rat{q(5), q(29)}
	org, dir := [3]*big.Rat{q(10), q(0), q(1)}, [3]*big.Rat{q(0), q(3), q(0)}
	opts := ToroidLineOptions{ParameterTolerance: 1e-10}
	h, e := FirstBoxUnequalCenterDualCircleBoundaryHitRational(ctx, lo, hi, xs, rs, ys, q(3), org, dir, q(0), q(1), opts)
	if e != nil || !h.HasHit || len(h.Normals) != 2 {
		t.Fatal(h, e)
	}
	root, _, e := NewExactRationalRoot(ctx, big.NewRat(1, 3), ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	same, e := h.Parameter.Compare(ctx, root, ExactCurvePlaneEventOptions{})
	if e != nil || !same.Proven || same.Order != 0 {
		t.Fatal(same, e)
	}
	for i, n := range h.Normals {
		if n.Constant[1].Cmp(ys[i]) != 0 || n.Linear[1].Cmp(q(-3)) != 0 || n.DirectionSign != -1 {
			t.Fatal(n)
		}
	}
	transitions, e := IntersectBoxUnequalCenterDualCircleBoundaryTransitionsRational(ctx, lo, hi, xs, rs, ys, q(3), org, dir, big.NewRat(1, 3), q(1), opts)
	if e != nil || len(transitions.Events) != 1 || transitions.Events[0].Before.Known || !transitions.Events[0].AtRangeStart || transitions.Events[0].After.Location != BoreInside {
		t.Fatal(transitions, e)
	}
	org[0] = q(9)
	line, e := IntersectBoxUnequalCenterDualCircleBoundaryRational(ctx, lo, hi, xs, rs, ys, q(3), org, dir, q(0), q(1), opts)
	if e != nil || len(line.Hits) != 1 || line.Hits[0].Support != BoxDualCircleFirstWall || line.Hits[0].Bounds.Lower.Cmp(line.Hits[0].Bounds.Upper) == 0 {
		t.Fatal(line, e)
	}
	bad := ys
	bad[1] = nil
	line, e = IntersectBoxUnequalCenterDualCircleBoundaryRational(ctx, lo, hi, xs, rs, bad, q(3), org, dir, q(0), q(1), opts)
	if e == nil || len(line.Hits) != 0 {
		t.Fatal(line, e)
	}
	opts.Surface.MaxOperations = 1
	h, e = FirstBoxUnequalCenterDualCircleBoundaryHitRational(ctx, lo, hi, xs, rs, ys, q(3), org, dir, q(0), q(1), opts)
	if e == nil || h.HasHit {
		t.Fatal(h, e)
	}
}
