package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestConstantCircleComponentsSeamAndNeighbors(t *testing.T) {
	ctx := context.Background()
	root := func(n int64) *ExactGraphLineRoot {
		r, _, e := NewExactRationalRoot(ctx, big.NewRat(n, 1), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return r.root
	}
	makeCell := func(lo, hi int64, x, w rationalPolynomial) *ExactComposedCurveCell {
		return &ExactComposedCurveCell{power: [4]rationalPolynomial{x, nil, nil, w}, endpoints: [2]*ExactGraphLineRoot{root(lo), root(hi)}}
	}
	one := rationalPolynomial{big.NewRat(1, 1)}
	before := makeCell(-1, 0, rationalPolynomial{big.NewRat(1, 1), big.NewRat(-1, 1)}, one)
	a := makeCell(0, 1, one, one)
	// Same point with nonconstant positive homogeneous denominator.
	b := makeCell(1, 2, rationalPolynomial{big.NewRat(1, 1), big.NewRat(1, 1)}, rationalPolynomial{big.NewRat(1, 1), big.NewRat(1, 1)})
	after := makeCell(2, 3, rationalPolynomial{big.NewRat(7, 1), big.NewRat(-3, 1)}, one)
	c := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{before, a, b, after}}
	center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	r, e := c.ClassifyAllPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Components) != 2 || len(r.Stationary) != 0 {
		t.Fatal(r, e)
	}
	component := r.Components[0]
	if component.Point != nil || len(component.Pieces) != 0 || len(component.ConstantPieces) != 2 || component.ConstantPieces[0].Cell != 1 || component.ConstantPieces[1].Cell != 2 || component.Before == nil || component.After == nil || component.Before.SideSigns != [2]int{1, 0} || component.After.SideSigns != [2]int{0, -1} || !r.Components[1].Point.Crossing {
		t.Fatal(r)
	}
	p, e := r.Components[1].Point.Event.EvaluateRationalWitness(ctx, big.NewRat(8, 3), ExactCurvePlaneEventOptions{})
	if e != nil || p.Point[0].Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal(p, e)
	}
	component.ConstantPieces[0].Point[0].SetInt64(99)
	again, e := c.ClassifyAllPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e != nil || again.Components[0].ConstantPieces[0].Point[0].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(again, e)
	}
	strict, e := c.ClassifyStationaryPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e == nil || strict.Complete {
		t.Fatal(strict, e)
	}
	limited, e := c.ClassifyAllPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{MaxEvents: 6})
	if e == nil || limited.Complete || len(limited.Components) != 0 {
		t.Fatal(limited, e)
	}
	bad := *b
	bad.power[0] = rationalPolynomial{big.NewRat(-1, 1), big.NewRat(-1, 1)}
	c.Cells[2] = &bad
	broken, e := c.ClassifyAllPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e == nil || broken.Complete {
		t.Fatal(broken, e)
	}
}
