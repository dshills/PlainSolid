package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestBoxDualCircleTransitions(t *testing.T) {
	q := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	min, max := [3]*big.Rat{q(0), q(0), q(0)}, [3]*big.Rat{q(10), q(6), q(6)}
	xs, rs := [2]*big.Rat{big.NewRat(7, 2), big.NewRat(13, 2)}, [2]*big.Rat{big.NewRat(5, 2), big.NewRat(5, 2)}
	opts := ToroidLineOptions{ParameterTolerance: 1e-9}
	call := func(org, dir [3]*big.Rat, lo, hi *big.Rat) (BoxDualCircleBoundaryTransitions, error) {
		return IntersectBoxDualCircleBoundaryTransitionsRational(context.Background(), min, max, xs, rs, q(3), org, dir, lo, hi, opts)
	}
	r, e := call([3]*big.Rat{q(5), q(0), q(1)}, [3]*big.Rat{q(0), q(2), q(0)}, q(0), q(2))
	if e != nil || len(r.Events) != 1 {
		t.Fatalf("cross %+v %v", r, e)
	}
	event := r.Events[0]
	if event.Support != BoxDualCircleFirstWall|BoxDualCircleSecondWall || !event.Before.Known || !event.After.Known || event.Before.Location != BoreOutside || event.After.Location != BoreInside {
		t.Fatalf("cross sides %+v", event)
	}
	// Tangency to either cutter from retained material: both open germs inside.
	r, e = call([3]*big.Rat{q(0), big.NewRat(5, 2), q(1)}, [3]*big.Rat{q(1), q(0), q(0)}, q(1), q(9))
	if e != nil || len(r.Events) != 2 {
		t.Fatalf("touch %+v %v", r, e)
	}
	for _, v := range r.Events {
		if v.Before.Location != BoreInside || v.After.Location != BoreInside {
			t.Fatalf("touch sides %+v", v)
		}
	}
	// At stock X=0, a quadratic touch is unavailable for a straight ray;
	// the continuous boundary generator instead has distinct endpoint germs.
	r, e = call([3]*big.Rat{q(5), q(2), q(1)}, [3]*big.Rat{q(0), q(0), q(1)}, q(0), q(4))
	if e != nil || len(r.Boundary.Intervals) != 1 || len(r.Events) != 2 {
		t.Fatalf("span %+v %v", r, e)
	}
	if !r.Events[0].AtRangeStart || r.Events[0].Before.Known || r.Events[0].After.Location != BoreBoundary || r.Events[1].Before.Location != BoreBoundary || r.Events[1].After.Location != BoreInside || r.Events[1].Support&BoxDualCircleBlindFloor == 0 {
		t.Fatalf("span sides %+v", r.Events)
	}

	// A stock corner touch has exterior on both sides, unlike a cutter kiss.
	r, e = call([3]*big.Rat{q(0), q(0), q(4)}, [3]*big.Rat{q(1), q(-1), q(0)}, q(-1), q(1))
	if e != nil || len(r.Events) != 1 || r.Events[0].Support != BoxDualCircleStockX|BoxDualCircleStockY || r.Events[0].Before.Location != BoreOutside || r.Events[0].After.Location != BoreOutside || r.Events[0].Before.Support != 0 || r.Events[0].After.Support != 0 {
		t.Fatalf("exterior touch %+v %v", r, e)
	}
	// Clip exactly at crossing: the absent side is not extrapolated.
	r, e = call([3]*big.Rat{q(5), q(0), q(1)}, [3]*big.Rat{q(0), q(2), q(0)}, q(1), q(1))
	if e != nil || len(r.Events) != 1 || r.Events[0].Before.Known || r.Events[0].After.Known || !r.Events[0].AtRangeStart || !r.Events[0].AtRangeEnd {
		t.Fatalf("singleton %+v %v", r, e)
	}
	// A rational offset chosen at 2^-80 cannot be confused with the tangent.
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	y := new(big.Rat).Sub(big.NewRat(5, 2), eps)
	r, e = call([3]*big.Rat{q(0), y, q(1)}, [3]*big.Rat{q(1), q(0), q(0)}, q(3), q(4))
	if e != nil || len(r.Events) != 2 || r.Events[0].Before.Location != BoreInside || r.Events[0].After.Location != BoreOutside || r.Events[1].Before.Location != BoreOutside || r.Events[1].After.Location != BoreInside {
		t.Fatalf("near tangent %+v %v", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = IntersectBoxDualCircleBoundaryTransitionsRational(ctx, min, max, xs, rs, q(3), min, max, q(0), q(1), opts)
	if !errors.Is(e, context.Canceled) || len(r.Events) != 0 {
		t.Fatalf("cancel %+v %v", r, e)
	}
	opts.Surface.MaxOperations = 1
	r, e = IntersectBoxDualCircleBoundaryTransitionsRational(context.Background(), min, max, xs, rs, q(3), min, max, q(0), q(1), opts)
	if e == nil || len(r.Events) != 0 || len(r.Boundary.Hits) != 0 {
		t.Fatalf("budget %+v %v", r, e)
	}
}
