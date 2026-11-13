package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestComposedIndependentQuadraticAffineProjection(t *testing.T) {
	ctx := context.Background()
	makeCell := func(reverse, coincident bool) *ExactComposedCurveCell {
		x := []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(1, 1)}
		y := []*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}
		if reverse {
			x = []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(0, 1)}
			y = []*big.Rat{big.NewRat(3, 2), big.NewRat(-1, 2), big.NewRat(-1, 2)}
			if coincident {
				y = []*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)}
			}
		}
		net := make([][4]*big.Rat, 3)
		for i := range net {
			net[i] = [4]*big.Rat{x[i], y[i], new(big.Rat), big.NewRat(1, 1)}
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
	a, c := makeCell(false, false), makeCell(true, false)
	lower := importTestRoot(t, big.NewRat(-1, 3), true, "1/2", "2/3")
	upper := importTestRoot(t, big.NewRat(-2, 3), true, "3/4", "1")
	cut, e := a.RestrictAlgebraic(ctx, lower, upper, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a = cut.Cell
	bl := importTestRoot(t, big.NewRat(-1, 5), false, "1/5", "1/5")
	bh := importTestRoot(t, big.NewRat(-1, 3), false, "1/3", "1/3")
	cut, e = c.RestrictAlgebraic(ctx, bl, bh, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c = cut.Cell
	r, e := IntersectComposedCurvesAffineProjection(ctx, a, c, 0, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || r.Coincident || len(r.Hits) != 1 || r.Scale.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal(r, e)
	}
	rootA := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	rootB, _, e := rootA.AffineMap(ctx, big.NewRat(1, 1), big.NewRat(-1, 1), ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]*ExactAlgebraicRoot{{r.Hits[0].A.Root(), rootA}, {r.Hits[0].B.Root(), rootB}} {
		order, e := pair[0].Compare(ctx, pair[1], ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal(order, e)
		}
	}
	for _, event := range []*ExactComposedCurveEvent{r.Hits[0].A, r.Hits[0].B} {
		q, e := event.Enclose(ctx, ComposedCurveEndpointOptions{})
		if e != nil || q.Bounds.Min.Y > .5 || q.Bounds.Max.Y < .5 {
			t.Fatal(q, e)
		}
	}
	swapped, e := IntersectComposedCurvesAffineProjection(ctx, c, a, 0, ExactCurvePlaneEventOptions{})
	if e != nil || !swapped.Complete || len(swapped.Hits) != 1 {
		t.Fatal(swapped, e)
	}
	order, e := swapped.Hits[0].A.Root().Compare(ctx, rootB, ExactCurvePlaneEventOptions{})
	if e != nil || order.Order != 0 {
		t.Fatal(order, e)
	}
	near := *c
	near.power = c.power
	near.power[2] = rationalPolynomial{new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))}
	miss, e := IntersectComposedCurvesAffineProjection(ctx, a, &near, 0, ExactCurvePlaneEventOptions{})
	if e != nil || !miss.Complete || len(miss.Hits) != 0 || miss.Coincident {
		t.Fatal(miss, e)
	}

	same := makeCell(true, true)
	overlap, e := IntersectComposedCurvesAffineProjection(ctx, a, same, 0, ExactCurvePlaneEventOptions{})
	if e != nil || !overlap.Complete || !overlap.Coincident || overlap.PointOnly {
		t.Fatal(overlap, e)
	}
	// Disjoint parameter domains whose projections meet at exactly one irrational
	// endpoint must retain a point contact, not a positive-length coincidence.
	aEnd, e := a.RestrictAlgebraic(ctx, lower, rootA, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	zero, _ := same.EndpointRoot(0)
	bEnd, e := same.RestrictAlgebraic(ctx, zero, rootB, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point, e := IntersectComposedCurvesAffineProjection(ctx, aEnd.Cell, bEnd.Cell, 0, ExactCurvePlaneEventOptions{})
	if e != nil || !point.Complete || !point.PointOnly || point.Coincident || len(point.Hits) != 1 {
		t.Fatal(point, e)
	}
	if _, e = IntersectComposedCurvesAffineProjection(ctx, a, c, 1, ExactCurvePlaneEventOptions{}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	if _, e = IntersectComposedCurvesAffineProjection(ctx, a, c, 0, ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = IntersectComposedCurvesAffineProjection(cancelCtx, a, c, 0, ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
