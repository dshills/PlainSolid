package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCircleComponentsCoincidentSeamAndIsolatedContact(t *testing.T) {
	ctx := context.Background()
	root := func(v *big.Rat) *ExactAlgebraicRoot {
		r, _, e := NewExactRationalRoot(ctx, v, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	a := directedCircleCell(t, 0, false, false, new(big.Rat))
	lo, _ := a.EndpointRoot(0)
	hi, _ := a.EndpointRoot(1)
	mid := root(big.NewRat(1, 2))
	left, e := a.RestrictAlgebraic(ctx, lo, mid, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	right, e := a.RestrictAlgebraic(ctx, mid, hi, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// From (0,1) at t=1 to (0,-2) at t=2: the start is absorbed
	// by the coincident component, but t=5/3 remains a true crossing.
	line := &ExactComposedCurveCell{power: [4]rationalPolynomial{nil, {big.NewRat(4, 1), big.NewRat(-3, 1)}, nil, {big.NewRat(1, 1)}}, endpoints: [2]*ExactGraphLineRoot{hi.root, root(big.NewRat(2, 1)).root}}
	c := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{left.Cell, right.Cell, line}}
	center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	r, e := c.ClassifyPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Components) != 2 || len(r.Components[0].Pieces) != 2 || r.Components[0].Point != nil || r.Components[0].Before != nil || r.Components[0].After == nil || r.Components[0].After.SideSigns != [2]int{0, -1} || r.Components[0].Pieces[0].Orientation != 1 || r.Components[0].Pieces[1].Orientation != 1 || r.Components[1].Point == nil || !r.Components[1].Point.Crossing || len(r.Intervals) != 2 {
		t.Fatal(r, e)
	}
	witness, e := r.Components[1].Point.Event.EvaluateRationalWitness(ctx, big.NewRat(5, 3), ExactCurvePlaneEventOptions{})
	if e != nil || witness.Point[1].Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal(witness, e)
	}
	flipped, e := c.ClassifyPlanarCircleComponents(ctx, [2]int{1, 0}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e != nil || flipped.Components[0].Pieces[0].Orientation != -1 {
		t.Fatal(flipped, e)
	}
	limited, e := c.ClassifyPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{MaxEvents: 5})
	if e == nil || limited.Complete || len(limited.Components) != 0 {
		t.Fatal(limited, e)
	}
	// Source discontinuity remains a failure even with claimed completeness.
	bad := *line
	bad.power[0] = rationalPolynomial{big.NewRat(1, 1000)}
	c.Cells[2] = &bad
	broken, e := c.ClassifyPlanarCircleComponents(ctx, [2]int{0, 1}, center, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e == nil || broken.Complete {
		t.Fatal(broken, e)
	}
}

func TestCircleComponentsStationaryCoincidenceRejected(t *testing.T) {
	ctx := context.Background()
	a := directedCircleCell(t, 0, false, false, new(big.Rat))
	constant := *a
	constant.power = [4]rationalPolynomial{{big.NewRat(1, 1)}, nil, nil, {big.NewRat(1, 1)}}
	c := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{&constant}}
	r, e := c.ClassifyPlanarCircleComponents(ctx, [2]int{0, 1}, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, big.NewRat(1, 1), ComposedPlaneContactsOptions{})
	if e == nil || r.Complete || len(r.Components) != 0 {
		t.Fatal(r, e)
	}
}
