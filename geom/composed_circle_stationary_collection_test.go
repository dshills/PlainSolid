package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestStationaryCircleCollectionSeamAndDeparture(t *testing.T) {
	ctx := context.Background()
	cell := stationaryCircleFixture(t)
	root := func(v *big.Rat) *ExactAlgebraicRoot {
		r, _, e := NewExactRationalRoot(ctx, v, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	lo, _ := cell.EndpointRoot(0)
	hi, _ := cell.EndpointRoot(1)
	mid := root(big.NewRat(1, 2))
	a, e := cell.RestrictAlgebraic(ctx, lo, mid, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := cell.RestrictAlgebraic(ctx, mid, hi, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	line := &ExactComposedCurveCell{power: [4]rationalPolynomial{{big.NewRat(60, 17), big.NewRat(-45, 17)}, {big.NewRat(32, 17), big.NewRat(-24, 17)}, nil, {big.NewRat(1, 1)}}, endpoints: [2]*ExactGraphLineRoot{hi.root, root(big.NewRat(2, 1)).root}}
	c := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{a.Cell, b.Cell, line}}
	center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	r, e := c.ClassifyStationaryPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Stationary) != 1 || len(r.Stationary[0].Cells) != 2 || len(r.Stationary[0].Local) != 2 || !r.Stationary[0].Reverses || r.Stationary[0].SideOrientations != [2]int{-1, 1} || len(r.Components) != 2 || len(r.Components[0].Pieces) != 2 || r.Components[0].Pieces[0].Cell != 0 || r.Components[0].Pieces[1].Cell != 1 || r.Components[0].After == nil || r.Components[1].Point == nil || !r.Components[1].Point.Crossing {
		t.Fatal(r, e)
	}
	p, e := r.Components[1].Point.Event.EvaluateRationalWitness(ctx, big.NewRat(5, 3), ExactCurvePlaneEventOptions{})
	if e != nil || p.Point[0].Cmp(big.NewRat(-15, 17)) != 0 {
		t.Fatal(p, e)
	}
	// Raw stationary roots are charged before the shared endpoint is merged.
	limited, e := c.ClassifyStationaryPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{MaxEvents: 7})
	if e == nil || limited.Complete || len(limited.Stationary) != 0 {
		t.Fatal(limited, e)
	}
	strict, e := c.ClassifyPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e == nil || strict.Complete {
		t.Fatal(strict, e)
	}
	// A single folding cell produces the same material component ordering.
	c.Cells = []*ExactComposedCurveCell{cell, line}
	whole, e := c.ClassifyStationaryPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e != nil || !whole.Complete || len(whole.Stationary) != 1 || len(whole.Stationary[0].Cells) != 1 || len(whole.Components[0].Pieces) != 2 {
		t.Fatal(whole, e)
	}
}
