package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactCurveIdentityGraphAlgebraicKnotContacts(t *testing.T) {
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
	curve := func(z [3]*big.Rat) *ExactSplineCurve {
		c, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(0, 1), big.NewRat(1, 3), z[0], big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 3), z[1], big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 3), z[2], big.NewRat(1, 1)}}, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	c := curve([3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2)})
	r, e := IntersectExactCurveIdentityGraph(ctx, c, p, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete || len(r.Cells) != 2 {
		t.Fatal(r, e)
	}
	for _, cell := range r.Cells {
		if len(cell.Intersection.Events) != 1 || cell.Intersection.Events[0].Multiplicity() != 1 {
			t.Fatal(cell)
		}
		q, e := cell.Intersection.Events[0].Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: 1e-9})
		if e != nil || q.Bounds.Min.X > .5 || q.Bounds.Max.X < .5 || q.Bounds.Min.Z > .5 || q.Bounds.Max.Z < .5 {
			t.Fatal(q, e)
		}
	}
	if r.Cells[0].EventSigns[0] != [2]int{1, 0} || r.Cells[1].EventSigns[0] != [2]int{0, -1} {
		t.Fatal("crossing sides", r.Cells)
	}
	limited, e := IntersectExactCurveIdentityGraph(ctx, c, p, SurfaceCurveAgreementOptions{MaxSubdivisions: 1})
	if !errors.Is(e, ErrConvergence) || limited.Complete {
		t.Fatal(limited, e)
	}

	order, e := r.Cells[0].Intersection.Events[0].CompareParameter(ctx, r.Cells[1].Intersection.Events[0], ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	c = curve([3]*big.Rat{big.NewRat(-1, 4), big.NewRat(1, 4), big.NewRat(3, 4)})
	r, e = IntersectExactCurveIdentityGraph(ctx, c, p, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete || len(r.Cells[0].Intersection.Events) != 1 || r.Cells[0].Intersection.Events[0].Multiplicity() != 2 {
		t.Fatal(r, e)
	}
	if r.Cells[0].EventSigns[0] != [2]int{-1, -1} {
		t.Fatal("tangent sides", r.Cells[0].EventSigns)
	}

	c = curve([3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)})
	r, e = IntersectExactCurveIdentityGraph(ctx, c, p, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for _, cell := range r.Cells {
		if !cell.Intersection.Coincident {
			t.Fatal(cell)
		}
	}
	weighted, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(2, 3), big.NewRat(1, 1), big.NewRat(2, 1)}, {big.NewRat(1, 1), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rationalResult, e := IntersectExactCurveIdentityGraph(ctx, weighted, p, SurfaceCurveAgreementOptions{})
	if e != nil || !rationalResult.Complete || len(rationalResult.Cells) != 2 {
		t.Fatal(rationalResult, e)
	}
	for _, cell := range rationalResult.Cells {
		if len(cell.Intersection.Events) != 1 {
			t.Fatal(cell)
		}
	}

	if _, e = IntersectExactCurveIdentityGraph(ctx, c, p, SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: 1}}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	_, _, wrong, _ := algebraicSeamFixture(t)
	if _, e = IntersectExactCurveIdentityGraph(ctx, c, wrong, SurfaceCurveAgreementOptions{}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = IntersectExactCurveIdentityGraph(cancelCtx, c, p, SurfaceCurveAgreementOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
