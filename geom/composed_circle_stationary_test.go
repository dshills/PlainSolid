package geom

import (
	"context"
	"math/big"
	"testing"
)

func stationaryCircleFixture(t *testing.T) *ExactComposedCurveCell {
	ctx := context.Background()
	net := make([][4]*big.Rat, 5)
	// x=(1-s²)/(1+s²), y=2s/(1+s²), s=(t-1/2)².
	ys := []*big.Rat{big.NewRat(1, 2), new(big.Rat), big.NewRat(-1, 6), new(big.Rat), big.NewRat(1, 2)}
	for i := range net {
		q := big.NewRat(1, 16)
		if i%2 == 1 {
			q.Neg(q)
		}
		net[i] = [4]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), q), ys[i], new(big.Rat), new(big.Rat).Add(big.NewRat(1, 1), q)}
	}
	curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return cell
}

func TestCoincidentCircleStationaryReversal(t *testing.T) {
	ctx := context.Background()
	cell := stationaryCircleFixture(t)
	center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	r, e := cell.SplitCoincidentCircleOrientation(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || len(r.Pieces) != 2 || r.Pieces[0].Orientation != -1 || r.Pieces[1].Orientation != 1 || len(r.Stationary) != 1 || !r.Stationary[0].Reverses || r.Stationary[0].Order != 1 {
		t.Fatal(r, e)
	}
	p, e := r.Stationary[0].Event.EvaluateRationalWitness(ctx, big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	if e != nil || p.Point[0].Cmp(big.NewRat(1, 1)) != 0 || p.Point[1].Sign() != 0 {
		t.Fatal(p, e)
	}
	cut, e := cell.RestrictAlgebraic(ctx, r.Stationary[0].Event.Root(), r.Domain[1], ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	half, e := cut.Cell.SplitCoincidentCircleOrientation(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e != nil || !half.Complete || len(half.Pieces) != 1 || len(half.Stationary) != 1 || half.Stationary[0].Reverses || half.Stationary[0].SideOrientations != [2]int{0, 1} {
		t.Fatal(half, e)
	}
	strict, e := (SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{cell}}).ClassifyPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e == nil || strict.Complete {
		t.Fatal(strict, e)
	}
	limited, e := cell.SplitCoincidentCircleOrientation(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || limited.Complete || len(limited.Pieces) != 0 {
		t.Fatal(limited, e)
	}
}
