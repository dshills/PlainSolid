package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestComposedGraphLocalWindowOriginalUnits(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lower := importTestRoot(t, big.NewRat(-1, 3), true, "1/2", "2/3")
	upper := importTestRoot(t, big.NewRat(-2, 3), true, "3/4", "1")
	// Exact continuation C=(t-11/20,1/2,t-7/10), represented with
	// W=t-1/4. W is positive on the whole true algebraic domain, but
	// negative near t=0; X also leaves the graph on the global continuation.
	b := &polynomialBudget{maxOperations: 100000, maxBits: 4096}
	w := rationalPolynomial{big.NewRat(-1, 4), big.NewRat(1, 1)}
	cell := &ExactComposedCurveCell{endpoints: [2]*ExactGraphLineRoot{copyExactRoot(lower.root), copyExactRoot(upper.root)}}
	for axis, poly := range []rationalPolynomial{{big.NewRat(-11, 20), big.NewRat(1, 1)}, {big.NewRat(1, 2)}, {big.NewRat(-7, 10), big.NewRat(1, 1)}, {big.NewRat(1, 1)}} {
		cell.power[axis], e = polynomialProduct(poly, w, b)
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := IntersectExactComposedCurveIdentityGraph(ctx, cell, p, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete || len(r.Cells) != 1 || len(r.Cells[0].Intersection.Events) != 1 {
		t.Fatal(r, e)
	}
	event := r.Cells[0].Intersection.Events[0]
	expected := importTestRoot(t, big.NewRat(-7, 10), false, "7/10", "7/10")
	order, e := event.Root().Compare(ctx, expected, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	q, e := event.Enclose(ctx, ComposedCurveEndpointOptions{})
	if e != nil || q.Bounds.Min.X > .15 || q.Bounds.Max.X < .15 || q.Bounds.Min.Y > .5 || q.Bounds.Max.Y < .5 || q.Bounds.Min.Z > 0 || q.Bounds.Max.Z < 0 {
		t.Fatal(q, e)
	}
	if r.Cells[0].EventSigns[0] != [2]int{-1, 1} {
		t.Fatal(r.Cells[0].EventSigns)
	}
}
