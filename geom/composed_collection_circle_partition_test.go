package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestCollectionCircleIrrationalSeam(t *testing.T) {
	ctx := context.Background()
	center := [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}
	radius := big.NewRat(1, 1)
	a := directedCircleCell(t, 0, false, false, new(big.Rat))
	single, e := a.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, radius, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cut := single.Contacts[0].Event.Root()
	left, e := a.RestrictAlgebraic(ctx, single.Domain[0], cut, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	right, e := a.RestrictAlgebraic(ctx, cut, single.Domain[1], ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	collection := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{left.Cell, right.Cell}}
	r, e := collection.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, radius, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || r.CellCount != 2 || len(r.Contacts) != 1 || len(r.Contacts[0].Cells) != 2 || len(r.Contacts[0].Local) != 2 || !r.Contacts[0].Crossing || r.Contacts[0].Touching || r.Contacts[0].SideSigns != [2]int{-1, 1} || len(r.Intervals) != 2 || r.Intervals[0].Cell != 0 || r.Intervals[1].Cell != 1 {
		t.Fatal(r, e)
	}
	limited, e := collection.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, radius, ComposedPlaneContactsOptions{MaxEvents: 1})
	if !errors.Is(e, ErrConvergence) || limited.Complete || len(limited.Intervals) != 0 || len(limited.Contacts) != 0 {
		t.Fatal(limited, e)
	}
	// Forged public Complete does not excuse an actual source/model discontinuity.
	wrong := *right.Cell
	wrong.power[2] = rationalPolynomial{big.NewRat(1, 1000)}
	collection.Cells[1] = &wrong
	r, e = collection.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, radius, ComposedPlaneContactsOptions{})
	if e == nil || r.Complete || len(r.Contacts) != 0 {
		t.Fatal(r, e)
	}
}

func TestCollectionCircleCornerKissAndScope(t *testing.T) {
	ctx := context.Background()
	root := func(x *big.Rat) *ExactGraphLineRoot {
		r, _, e := NewExactRationalRoot(ctx, x, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return r.root
	}
	zero, half, one := root(new(big.Rat)), root(big.NewRat(1, 2)), root(big.NewRat(1, 1))
	// C0 V-shaped model curve touches the disk at a sharp corner. Both local
	// residuals are simple endpoint roots; only their combined signs prove a kiss.
	a := &ExactComposedCurveCell{power: [4]rationalPolynomial{{big.NewRat(3, 2), big.NewRat(-1, 1)}, nil, nil, {big.NewRat(1, 1)}}, endpoints: [2]*ExactGraphLineRoot{zero, half}}
	b := &ExactComposedCurveCell{power: [4]rationalPolynomial{{big.NewRat(1, 2), big.NewRat(1, 1)}, nil, nil, {big.NewRat(1, 1)}}, endpoints: [2]*ExactGraphLineRoot{half, one}}
	c := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{a, b}}
	center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	r, e := c.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Contacts) != 1 || !r.Contacts[0].Touching || r.Contacts[0].Crossing || r.Contacts[0].SideSigns != [2]int{1, 1} || r.Contacts[0].Local[0].Order != 1 || r.Contacts[0].Local[1].Order != 1 {
		t.Fatal(r, e)
	}
	c.Cells = c.Cells[1:]
	r, e = c.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || r.CellCount != 1 || r.Contacts[0].Touching || r.Contacts[0].Crossing || r.Contacts[0].SideSigns != [2]int{0, 1} {
		t.Fatal(r, e)
	}
	arc := directedCircleCell(t, 0, false, false, new(big.Rat))
	c.Cells = []*ExactComposedCurveCell{arc}
	r, e = c.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Contacts) != 0 {
		t.Fatal(r, e)
	}
}
