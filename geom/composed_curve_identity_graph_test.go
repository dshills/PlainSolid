package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestComposedCurveIdentityGraphRestrictedAlgebraic(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {Y: 1}, {X: 1, Z: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.5, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lower := importTestRoot(t, big.NewRat(-1, 3), true, "1/2", "2/3")
	upper := importTestRoot(t, big.NewRat(-2, 3), true, "3/4", "1")
	restriction, e := cell.RestrictAlgebraic(ctx, lower, upper, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntersectExactComposedCurveIdentityGraph(ctx, restriction.Cell, p, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete || len(r.Cells) != 2 {
		t.Fatal(r, e)
	}
	for _, c := range r.Cells {
		if len(c.Intersection.Events) != 1 {
			t.Fatal(c)
		}
		q, e := c.Intersection.Events[0].Enclose(ctx, ComposedCurveEndpointOptions{})
		if e != nil || q.Bounds.Min.Y > 1.0/3 || q.Bounds.Max.Y < 1.0/3 {
			t.Fatal(q, e)
		}
	}
	exactContact := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	edgeCut, e := cell.RestrictAlgebraic(ctx, exactContact, upper, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	edge, e := IntersectExactComposedCurveIdentityGraph(ctx, edgeCut.Cell, p, SurfaceCurveAgreementOptions{})
	if e != nil || !edge.Complete || len(edge.Cells) != 1 || len(edge.Cells[0].Intersection.Events) != 1 || edge.Cells[0].Intersection.Events[0].EndpointMask() != 1 {
		t.Fatal(edge, e)
	}

	// The actual irrational lower cut is strictly above the contact by 2^-100
	// in squared parameter; overlapping floating boxes cannot include that root.
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	near := importTestRoot(t, new(big.Rat).Sub(big.NewRat(-1, 2), tiny), true, "2/3", "3/4")
	restriction, e = cell.RestrictAlgebraic(ctx, near, upper, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = IntersectExactComposedCurveIdentityGraph(ctx, restriction.Cell, p, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete || len(r.Cells) != 1 || len(r.Cells[0].Intersection.Events) != 0 {
		t.Fatal(r, e)
	}
	limited, e := IntersectExactComposedCurveIdentityGraph(ctx, restriction.Cell, p, SurfaceCurveAgreementOptions{MaxSubdivisions: 1})
	if !errors.Is(e, ErrConvergence) || limited.Complete {
		t.Fatal(limited, e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = IntersectExactComposedCurveIdentityGraph(cancelCtx, cell, p, SurfaceCurveAgreementOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
