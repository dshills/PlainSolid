package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestComposedAffineEdgeIndependentAlgebraicParameters(t *testing.T) {
	ctx := context.Background()
	makeCurve := func(points [][3]*big.Rat) *ExactSplineCurve {
		net := make([][4]*big.Rat, len(points))
		for i, p := range points {
			net[i] = [4]*big.Rat{p[0], p[1], p[2], big.NewRat(1, 1)}
		}
		c, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	curve := makeCurve([][3]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 3)}, {big.NewRat(1, 2), big.NewRat(0, 1), big.NewRat(1, 3)}, {big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 3)}})
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	line := makeCurve([][3]*big.Rat{{big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(1, 3)}, {big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(1, 3)}})
	r, e := IntersectExactComposedCurveAffineEdge(ctx, cell, line, ComposedCurveLineOptions{})
	if e != nil || !r.Complete || r.Coincident || len(r.Hits) != 1 || r.Scale.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal(r, e)
	}
	expected, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"1/2", "-2", "1"}, Lower: "1/4", Upper: "1/3"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	order, e := r.Hits[0].LineRoot.Compare(ctx, expected, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	source := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	order, e = r.Hits[0].CurveEvent.Root().Compare(ctx, source, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	near := new(big.Rat).Add(big.NewRat(1, 3), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	wrong := makeCurve([][3]*big.Rat{{big.NewRat(1, 1), big.NewRat(1, 2), near}, {big.NewRat(0, 1), big.NewRat(1, 2), near}})
	miss, e := IntersectExactComposedCurveAffineEdge(ctx, cell, wrong, ComposedCurveLineOptions{})
	if e != nil || !miss.Complete || len(miss.Hits) != 0 {
		t.Fatal(miss, e)
	}
	coincident, e := NewExactComposedBezierCell(ctx, line, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	upper := importTestRoot(t, big.NewRat(-2, 3), true, "3/4", "1")
	cut, e := coincident.RestrictAlgebraic(ctx, source, upper, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	overlap, e := IntersectExactComposedCurveAffineEdge(ctx, cut.Cell, line, ComposedCurveLineOptions{})
	if e != nil || !overlap.Complete || !overlap.Coincident || len(overlap.Portions) != 1 {
		t.Fatal(overlap, e)
	}
	order, e = overlap.Portions[0].Start.LineRoot.Compare(ctx, source, ExactCurvePlaneEventOptions{})
	if e != nil || order.Order != 0 {
		t.Fatal(order, e)
	}
	if _, e = IntersectExactComposedCurveAffineEdge(ctx, cell, line, ComposedCurveLineOptions{ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxOperations: 1}}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = IntersectExactComposedCurveAffineEdge(cancelCtx, cell, line, ComposedCurveLineOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
