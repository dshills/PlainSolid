package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestNonlinearQuadraticConicPairing(t *testing.T) {
	ctx := context.Background()
	makeCell := func(swap bool, z *big.Rat) *ExactComposedCurveCell {
		x := []*big.Rat{big.NewRat(-1, 1), new(big.Rat), big.NewRat(1, 1)}
		y := []*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 1), big.NewRat(1, 1)}
		if swap {
			x, y = y, x
		}
		net := make([][4]*big.Rat, 3)
		for i := range net {
			net[i] = [4]*big.Rat{x[i], y[i], z, big.NewRat(1, 1)}
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
	a, c := makeCell(false, new(big.Rat)), makeCell(true, new(big.Rat))
	zero, _ := a.EndpointRoot(0)
	cap := importTestRoot(t, big.NewRat(-9, 10), false, "9/10", "9/10")
	cut, e := a.RestrictAlgebraic(ctx, zero, cap, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a = cut.Cell
	r, e := IntersectComposedQuadraticConic(ctx, a, c, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.CoincidentSupport {
		t.Fatal(r, e)
	}
	half := importTestRoot(t, big.NewRat(-1, 2), false, "1/2", "1/2")
	for _, event := range []*ExactComposedCurveEvent{r.Hits[0].A, r.Hits[0].B} {
		order, e := event.Root().Compare(ctx, half, ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal(order, e)
		}
		point, e := event.Enclose(ctx, ComposedCurveEndpointOptions{})
		if e != nil || point.Bounds.Min.X > 0 || point.Bounds.Max.X < 0 || point.Bounds.Min.Y > 0 || point.Bounds.Max.Y < 0 {
			t.Fatal(point, e)
		}
	}
	// Parameter relation is nonlinear: s=((2t-1)^2+1)/2. The affine
	// projection path correctly cannot certify this owner pair.
	if _, e = IntersectComposedCurvesAffineProjection(ctx, a, c, 0, ExactCurvePlaneEventOptions{}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	near := makeCell(true, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	miss, e := IntersectComposedQuadraticConic(ctx, a, near, ExactCurvePlaneEventOptions{})
	if e != nil || !miss.Complete || len(miss.Hits) != 0 {
		t.Fatal(miss, e)
	}
	same, e := IntersectComposedQuadraticConic(ctx, a, a, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrConvergence) || same.Complete || !same.CoincidentSupport {
		t.Fatal(same, e)
	}
	if _, e = IntersectComposedQuadraticConic(ctx, a, c, ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = IntersectComposedQuadraticConic(cancelCtx, a, c, ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestNonlinearRationalConicAlgebraicPair(t *testing.T) {
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
	c := build([][4]*big.Rat{{new(big.Rat), big.NewRat(-1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(-1, 1), new(big.Rat), big.NewRat(2, 1)}, {big.NewRat(4, 1), big.NewRat(6, 1), new(big.Rat), big.NewRat(4, 1)}})
	r, e := IntersectComposedQuadraticConic(ctx, a, c, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
	// A t=sqrt(1/2); B s=t/(2-t), so 7s²-2s-1=0.
	expected, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "-2", "7"}, Lower: "1/2", Upper: "2/3"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	order, e := r.Hits[0].B.Root().Compare(ctx, expected, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	for _, event := range []*ExactComposedCurveEvent{r.Hits[0].A, r.Hits[0].B} {
		point, e := event.Enclose(ctx, ComposedCurveEndpointOptions{})
		if e != nil || point.Bounds.Min.Y > .5 || point.Bounds.Max.Y < .5 {
			t.Fatal(point, e)
		}
	}
	quartic := build([][4]*big.Rat{{new(big.Rat), big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(3, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(5, 4), new(big.Rat), big.NewRat(1, 1)}})
	pending, e := IntersectComposedQuadraticConic(ctx, a, quartic, ExactCurvePlaneEventOptions{})
	if e != nil || !pending.Complete || len(pending.Pending) != 0 || len(pending.Hits) != 0 {
		t.Fatal(pending, e)
	}

	quarticInside := build([][4]*big.Rat{{new(big.Rat), big.NewRat(-1, 4), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(3, 4), new(big.Rat), big.NewRat(1, 1)}})
	general, e := IntersectComposedQuadraticConic(ctx, a, quarticInside, ExactCurvePlaneEventOptions{})
	if e != nil || !general.Complete || len(general.Hits) != 2 || len(general.Pending) != 0 {
		t.Fatal(general, e)
	}
	for _, pair := range general.Hits {
		pa, e := pair.A.Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		pb, e := pair.B.Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := pa.Bounds.Intersect(pb.Bounds); !ok {
			t.Fatal(pa, pb)
		}
	}

}
