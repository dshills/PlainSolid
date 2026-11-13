package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestCoincidentConicMobiusAlgebraicOverlap(t *testing.T) {
	ctx := context.Background()
	build := func(net [][4]*big.Rat) *ExactComposedCurveCell {
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
	a := build([][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}})
	net := [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(2, 1)}, {big.NewRat(4, 1), big.NewRat(4, 1), new(big.Rat), big.NewRat(4, 1)}}
	c := build(net)
	lower := importTestRoot(t, big.NewRat(-1, 3), true, "1/2", "2/3")
	upper := importTestRoot(t, big.NewRat(-2, 3), true, "3/4", "1")
	cut, e := a.RestrictAlgebraic(ctx, lower, upper, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a = cut.Cell
	half := importTestRoot(t, big.NewRat(-1, 2), false, "1/2", "1/2")
	threeQuarter := importTestRoot(t, big.NewRat(-3, 4), false, "3/4", "3/4")
	clipped, e := c.RestrictAlgebraic(ctx, half, threeQuarter, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntersectCoincidentComposedConics(ctx, a, clipped.Cell, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || !r.Coincident || r.PointOnly || r.Orientation != 1 {
		t.Fatal(r, e)
	}
	twoThirds := importTestRoot(t, big.NewRat(-2, 3), false, "2/3", "2/3")
	for _, pair := range [][2]*ExactAlgebraicRoot{{r.Boundary[0].A.Root(), twoThirds}, {r.Boundary[0].B.Root(), half}, {r.Boundary[1].A.Root(), upper}} {
		order, e := pair[0].Compare(ctx, pair[1], ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal(order, e)
		}
	}
	// The exact irrational mapped endpoint separates interval overlap from a
	// singleton shared model point; no interval-bound representative is used.
	one, _ := c.EndpointRoot(1)
	justAfter, e := c.RestrictAlgebraic(ctx, r.Boundary[1].B.Root(), one, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point, e := IntersectCoincidentComposedConics(ctx, a, justAfter.Cell, ExactCurvePlaneEventOptions{})
	if e != nil || !point.Complete || !point.PointOnly || point.Coincident {
		t.Fatal(point, e)
	}
	net[0], net[2] = net[2], net[0]
	reversed := build(net)
	reverse, e := IntersectCoincidentComposedConics(ctx, a, reversed, ExactCurvePlaneEventOptions{})
	if e != nil || !reverse.Complete || !reverse.Coincident || reverse.Orientation != -1 {
		t.Fatal(reverse, e)
	}
	if _, e = IntersectCoincidentComposedConics(ctx, a, c, ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = IntersectCoincidentComposedConics(cancelCtx, a, c, ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
