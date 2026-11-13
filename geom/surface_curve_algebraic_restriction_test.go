package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func importTestRoot(t *testing.T, constant *big.Rat, quadratic bool, lo, hi string) *ExactAlgebraicRoot {
	t.Helper()
	coefficients := []string{constant.RatString(), "1"}
	if quadratic {
		coefficients = []string{constant.RatString(), "0", "1"}
	}
	root, e := ImportExactRoot(context.Background(), ExactRootRecipe{Version: 1, Coefficients: coefficients, Lower: lo, Upper: hi}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return root
}
func TestDirectBezierAlgebraicRestrictionCloseCuts(t *testing.T) {
	_, uv, _, _ := algebraicSeamFixture(t)
	cell, e := NewExactComposedBezierCell(context.Background(), uv, SurfacePatchOptions{})
	if e != nil || cell.HasSurfaceCell() {
		t.Fatal(cell, e)
	}
	lower := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	upper := importTestRoot(t, new(big.Rat).Sub(big.NewRat(-1, 2), tiny), true, "2/3", "3/4")
	restriction, e := cell.RestrictAlgebraic(context.Background(), lower, upper, ExactCurvePlaneEventOptions{})
	if e != nil || !restriction.Proven {
		t.Fatal(restriction, e)
	}
	intervals := restriction.Cell.EndpointIntervals()
	if intervals[0].Upper.Cmp(intervals[1].Lower) >= 0 {
		t.Fatal("distinct cuts not separated")
	}
	middle := new(big.Rat).Quo(new(big.Rat).Add(intervals[0].Upper, intervals[1].Lower), big.NewRat(2, 1))
	point, e := restriction.Cell.EvaluateRational(context.Background(), middle, ExactCurvePlaneEventOptions{})
	if e != nil || point.Point[0].Cmp(new(big.Rat).Mul(middle, middle)) != 0 || point.Point[1].Cmp(middle) != 0 {
		t.Fatal(point, e)
	}
	for endpoint := 0; endpoint < 2; endpoint++ {
		bounds, e := restriction.Cell.EncloseEndpoint(context.Background(), endpoint, ComposedCurveEndpointOptions{Tolerance: 1e-12})
		if e != nil || !bounds.Converged {
			t.Fatal(bounds, e)
		}
	}
	event, e := cell.BindRoot(context.Background(), lower, ExactCurvePlaneEventOptions{})
	if e != nil || event.EndpointMask() != 0 {
		t.Fatal(event, e)
	}
	comparison, e := event.Root().Compare(context.Background(), lower, ExactCurvePlaneEventOptions{})
	if e != nil || !comparison.Proven || comparison.Order != 0 {
		t.Fatal(comparison, e)
	}
	_, e = cell.RestrictAlgebraic(context.Background(), upper, lower, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	outside := importTestRoot(t, big.NewRat(-2, 1), false, "2", "2")
	if _, e = cell.BindRoot(context.Background(), outside, ExactCurvePlaneEventOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
func TestAlgebraicRestrictionAcrossKnotSlices(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	collection, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lower := importTestRoot(t, big.NewRat(-1, 2), false, "1/2", "1/2")
	upper := importTestRoot(t, big.NewRat(-3, 4), false, "3/4", "3/4")
	restricted, e := collection.RestrictAlgebraic(context.Background(), lower, upper, ExactCurvePlaneEventOptions{})
	if e != nil || !restricted.Complete || len(restricted.Cells) != 2 {
		t.Fatal(restricted, e)
	}
	a, _ := restricted.Cells[0].EndpointRoot(1)
	b, _ := restricted.Cells[1].EndpointRoot(0)
	order, e := a.Compare(context.Background(), b, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	// A forged collection flag cannot make unrelated models continuous.
	direct, e := NewExactComposedBezierCell(context.Background(), uv, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	end, _ := direct.EndpointRoot(1)
	rogue, e := direct.RestrictAlgebraic(context.Background(), a, end, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	forged := collection
	forged.Cells = append([]*ExactComposedCurveCell(nil), collection.Cells...)
	forged.Cells[1] = rogue.Cell
	_, e = forged.RestrictAlgebraic(context.Background(), lower, upper, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	_, e = collection.RestrictAlgebraic(context.Background(), lower, upper, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil {
		t.Fatal("budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = direct.RestrictAlgebraic(ctx, lower, upper, ExactCurvePlaneEventOptions{})
	if e != context.Canceled {
		t.Fatal(e)
	}
}
