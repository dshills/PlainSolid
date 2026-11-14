package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestBoxDualCircleFirstHit(t *testing.T) {
	q := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	min := [3]*big.Rat{q(0), q(0), q(0)}
	max := [3]*big.Rat{q(10), q(6), q(6)}
	xs := [2]*big.Rat{big.NewRat(7, 2), big.NewRat(13, 2)}
	rs := [2]*big.Rat{big.NewRat(5, 2), big.NewRat(5, 2)}
	call := func(org, dir [3]*big.Rat, lo, hi *big.Rat) (BoxDualCircleFirstHit, error) {
		return FirstBoxDualCircleBoundaryHitRational(context.Background(), min, max, xs, rs, q(3), org, dir, lo, hi, ToroidLineOptions{ParameterTolerance: 1e-10})
	}
	// Exact circle junction (5,2,1): distinct cutter normals survive.
	h, e := call([3]*big.Rat{q(5), q(0), q(1)}, [3]*big.Rat{q(0), q(2), q(0)}, q(0), q(3))
	if e != nil || !h.HasHit || h.Support != BoxDualCircleFirstWall|BoxDualCircleSecondWall || len(h.Normals) != 2 || h.SpanEnd != nil {
		t.Fatalf("junction %+v %v", h, e)
	}
	if h.Bounds.Lower.Cmp(q(1)) != 0 || h.Normals[0].Constant[0].Cmp(big.NewRat(-3, 2)) != 0 || h.Normals[1].Constant[0].Cmp(big.NewRat(3, 2)) != 0 || h.Normals[0].DirectionSign != -1 {
		t.Fatalf("normals %+v", h)
	}
	// A generator begins on the circle junction and ends on the blind floor.
	h, e = call([3]*big.Rat{q(5), q(2), q(1)}, [3]*big.Rat{q(0), q(0), q(1)}, q(0), q(4))
	if e != nil || h.SpanEnd == nil || !h.Tangent {
		t.Fatalf("generator %+v %v", h, e)
	}
	// Irrational wall height, with raw line speed three.
	h, e = call([3]*big.Rat{q(3), q(0), q(1)}, [3]*big.Rat{q(0), q(3), q(0)}, q(0), q(1))
	if e != nil || !h.HasHit || h.Bounds.Lower.Cmp(h.Bounds.Upper) == 0 || h.Support != BoxDualCircleFirstWall {
		t.Fatalf("irrational %+v %v", h, e)
	}
	// Starting at the floor inside the removed footprint is a boundary hit,
	// with outward normal toward the removed negative-Z side.
	h, e = call([3]*big.Rat{q(5), q(1), q(3)}, [3]*big.Rat{q(0), q(0), q(2)}, q(0), q(1))
	if e != nil || h.Support != BoxDualCircleBlindFloor || h.Normals[0].Constant[2].Cmp(q(-1)) != 0 {
		t.Fatalf("floor %+v %v", h, e)
	}

	h, e = call([3]*big.Rat{q(0), big.NewRat(5, 2), q(1)}, [3]*big.Rat{q(1), q(0), q(0)}, q(1), q(9))
	if e != nil || !h.Tangent || h.Support != BoxDualCircleFirstWall {
		t.Fatalf("tangent %+v %v", h, e)
	}
	h, e = FirstBoxDualCircleBoundaryHitRational(context.Background(), min, max, xs, rs, q(3), min, max, q(0), q(1), ToroidLineOptions{ParameterTolerance: 1e-8, Surface: SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || h.HasHit || h.Parameter != nil {
		t.Fatalf("budget %+v %v", h, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h, e = FirstBoxDualCircleBoundaryHitRational(ctx, min, max, xs, rs, q(3), min, max, q(0), q(1), ToroidLineOptions{ParameterTolerance: 1e-8})
	if !errors.Is(e, context.Canceled) || h.HasHit {
		t.Fatalf("cancel %+v %v", h, e)
	}
}
