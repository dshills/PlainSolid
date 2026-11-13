package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestAffineGraphContactClipsAlgebraicDomainCrossings(t *testing.T) {
	ctx := context.Background()
	target := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	target, e := target.Trim(ParameterRectangle{.25, .75, .25, .75})
	if e != nil {
		t.Fatal(e)
	}
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
	// XY=(t²,1/2) enters/exits the chart at t=1/2,sqrt(3/4).
	// Z=t²-1/2 has an interior contact at sqrt(1/2).
	cell := build([][4]*big.Rat{{new(big.Rat), big.NewRat(1, 2), big.NewRat(-1, 2), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 2), big.NewRat(-1, 2), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 1)}})
	r, e := IntersectExactComposedCurveAffineGraph(ctx, cell, target, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete || len(r.Cells) != 1 || len(r.Cells[0].Intersection.Events) != 1 {
		t.Fatal(r, e)
	}
	expected := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	order, e := r.Cells[0].Intersection.Events[0].Root().Compare(ctx, expected, ExactCurvePlaneEventOptions{})
	if e != nil || order.Order != 0 {
		t.Fatal(order, e)
	}
	// An outside-to-outside path touches the U-min side at exactly one point.
	// It must survive even though no open parameter interval lies in the chart.
	tangent := build([][4]*big.Rat{{new(big.Rat), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}})
	touch, e := IntersectExactComposedCurveAffineGraph(ctx, tangent, target, SurfaceCurveAgreementOptions{})
	if e != nil || !touch.Complete || len(touch.Cells) != 1 || len(touch.Cells[0].Intersection.Events) != 1 || touch.Cells[0].Intersection.Coincident {
		t.Fatal(touch, e)
	}
	half := importTestRoot(t, big.NewRat(-1, 2), false, "1/2", "1/2")
	order, e = touch.Cells[0].Intersection.Events[0].Root().Compare(ctx, half, ExactCurvePlaneEventOptions{})
	if e != nil || order.Order != 0 {
		t.Fatal(order, e)
	}
	if touch.Cells[0].Intersection.Events[0].Multiplicity() != 0 {
		t.Fatal("boundary-only interval order fabricated")
	}
}
