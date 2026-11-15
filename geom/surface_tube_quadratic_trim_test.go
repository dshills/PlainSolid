package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestTubeQuadraticTrimExactCrossingAndTangency(t *testing.T) {
	ctx := context.Background()
	knots := []float64{0, 0, 0, 1, 1, 1}
	poles := make([]Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles[3*i+j] = Vec3{float64(i) / 2, float64(j) / 2, z}
		}
	}
	spline, e := NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	original, e := NewSurfacePatch(spline, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, .5} })
	tube := func(ad, bd ParameterRectangle) VerifiedSurfaceTube {
		a, e := original.Trim(ad)
		if e != nil {
			t.Fatal(e)
		}
		b, e := plane.Trim(bd)
		if e != nil {
			t.Fatal(e)
		}
		proof, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return proof
	}
	proof := tube(ParameterRectangle{7.0 / 32, .25, 5.0 / 8, 11.0 / 16}, ParameterRectangle{3.0 / 16, 9.0 / 32, 19.0 / 32, 23.0 / 32})
	trim, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(15, 64), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(15, 64), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(15, 64), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := proof.IntersectQuadraticTrim(ctx, original, plane, 0, trim, TubeTrimIntersectionOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || !r.Hits[0].Transverse || r.Hits[0].Tangent || r.Hits[0].Multiplicity != 1 {
		t.Fatal(r, e)
	}
	if r.Hits[0].SideSigns != [2]int{-1, 1} {
		t.Fatal("residual side signs", r.Hits[0].SideSigns)
	}
	binding, e := proof.BindAffineGraphEvent(ctx, r.Hits[0].TrimEvent, TubeAlgebraicGraphEndpointOptions{})
	if e != nil {
		t.Fatal("event binding", e)
	}
	boundRoots, e := binding.Parameters()
	if e != nil {
		t.Fatal(e)
	}
	for i := range boundRoots {
		order, e := boundRoots[i].Compare(ctx, r.Hits[0].Parameters[i], ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal("binding root", i, order, e)
		}
	}
	if binding.ModelEvent() != r.Hits[0].TrimEvent {
		t.Fatal("lost lineage")
	}
	if _, e := proof.BindAffineGraphEvent(ctx, r.Hits[0].TrimEvent, TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: 1}}}); e == nil {
		t.Fatal("binding limit")
	}
	wrongRoot, _, e := NewExactRationalRoot(ctx, big.NewRat(4, 5), ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	wrongEvent, e := r.Hits[0].TrimEvent.cell.BindRoot(ctx, wrongRoot, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := proof.BindAffineGraphEvent(ctx, wrongEvent, TubeAlgebraicGraphEndpointOptions{}); e == nil {
		t.Fatal("accepted nonincident event")
	}
	boundedTarget, e := plane.Trim(ParameterRectangle{0, .5, .5, .875})
	if e != nil {
		t.Fatal(e)
	}
	clipped, e := proof.IntersectQuadraticTrim(ctx, original, boundedTarget, 0, trim, TubeTrimIntersectionOptions{})
	if e != nil || !clipped.Complete || len(clipped.Hits) != 1 || !clipped.Hits[0].Transverse {
		t.Fatal(clipped, e)
	}

	root, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1823/4096", "0", "0", "0", "1"}, Lower: "3/4", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	order, e := r.Hits[0].TrimEvent.Root().Compare(ctx, root, ExactCurvePlaneEventOptions{})
	if e != nil || order.Order != 0 {
		t.Fatal(order, e)
	}
	// Same exact graph contact, but its source U is outside another proved tube.
	other := tube(ParameterRectangle{.25, 9.0 / 32, 5.0 / 8, 11.0 / 16}, ParameterRectangle{7.0 / 32, 10.0 / 32, 19.0 / 32, 23.0 / 32})
	miss, e := other.IntersectQuadraticTrim(ctx, original, plane, 0, trim, TubeTrimIntersectionOptions{})
	if e != nil || !miss.Complete || len(miss.Hits) != 0 {
		t.Fatal(miss, e)
	}
	tangentTube := tube(ParameterRectangle{15.0 / 32, 17.0 / 32, 7.0 / 16, 9.0 / 16}, ParameterRectangle{7.0 / 16, 9.0 / 16, 3.0 / 8, 5.0 / 8})
	tangent, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{new(big.Rat), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	touch, e := tangentTube.IntersectQuadraticTrim(ctx, original, plane, 0, tangent, TubeTrimIntersectionOptions{})
	if e != nil || !touch.Complete || len(touch.Hits) != 1 || !touch.Hits[0].Tangent || touch.Hits[0].Transverse || touch.Hits[0].Multiplicity != 2 {
		t.Fatal(touch, e)
	}
	permutation := [3]int{0, 2, 1}
	budget := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
	projectedA, e := permuteGraphPatch(original, permutation, budget)
	if e != nil {
		t.Fatal(e)
	}
	projectedB, e := permuteGraphPatch(plane, permutation, budget)
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range []struct {
		tube VerifiedSurfaceTube
		trim *ExactSplineCurve
		want TubeTrimIntersection
	}{{proof, trim, r}, {tangentTube, tangent, touch}} {
		projectedTube, _, _, e := item.tube.projectedGraphTube(ctx, [2]int{0, 2}, TubeAlgebraicGraphEndpointOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, axes := range [][2]int{{0, 2}, {2, 0}} {
			hit, e := projectedTube.IntersectQuadraticTrimProjected(ctx, projectedA, projectedB, 0, item.trim, axes, TubeTrimIntersectionOptions{})
			if e != nil || !hit.Complete || len(hit.Hits) != 1 || hit.ResidualAxis != 1 || hit.ProjectionAxes != axes {
				t.Fatal("projected trim", hit, e)
			}
			if hit.Hits[0].Multiplicity != item.want.Hits[0].Multiplicity || hit.Hits[0].SideSigns != item.want.Hits[0].SideSigns || hit.Hits[0].Tangent != item.want.Hits[0].Tangent {
				t.Fatal("projected semantics", hit, item.want)
			}
			if hit.Hits[0].Bounds.Min.Y > .5 || hit.Hits[0].Bounds.Max.Y < .5 {
				t.Fatal("original XYZ", hit.Hits[0].Bounds)
			}
			order, e := hit.Hits[0].TrimEvent.Root().Compare(ctx, item.want.Hits[0].TrimEvent.Root(), ExactCurvePlaneEventOptions{})
			if e != nil || order.Order != 0 {
				t.Fatal("original root", order, e)
			}
			point, e := hit.Hits[0].TrimEvent.Enclose(ctx, ComposedCurveEndpointOptions{})
			if e != nil || point.Bounds.Min.Y > .5 || point.Bounds.Max.Y < .5 {
				t.Fatal("restored event", point, e)
			}
		}
	}
	circle, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(1, 2), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(2, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	outsideArc, e := circle.RationalSegment(ctx, new(big.Rat), big.NewRat(1, 10), TubeAffineLineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	excludedCoincidence, e := proof.IntersectQuadraticTrim(ctx, original, plane, 0, outsideArc, TubeTrimIntersectionOptions{})
	if e != nil || !excludedCoincidence.Complete || len(excludedCoincidence.Hits) != 0 || len(excludedCoincidence.Coincident) != 0 {
		t.Fatal(excludedCoincidence, e)
	}
	crossingArc, e := circle.RationalSegment(ctx, new(big.Rat), big.NewRat(1, 3), TubeAffineLineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pendingCoincidence, e := proof.IntersectQuadraticTrim(ctx, original, plane, 0, crossingArc, TubeTrimIntersectionOptions{})
	if !errors.Is(e, ErrConvergence) || pendingCoincidence.Complete || len(pendingCoincidence.Coincident) == 0 {
		t.Fatal(pendingCoincidence, e)
	}
	spanOpts := TubeTrimIntersectionOptions{CoincidentSpans: true}
	coincidence, e := proof.IntersectQuadraticTrim(ctx, original, plane, 0, crossingArc, spanOpts)
	if e != nil || !coincidence.Complete || len(coincidence.Spans) != 1 || len(coincidence.Hits)+len(coincidence.Coincident) != 0 {
		t.Fatal("exact clipped conic coincidence", coincidence, e)
	}
	c := coincidence.Spans[0]
	if c.FreeOrder != -1 {
		t.Fatal("directed decreasing conic", c.FreeOrder)
	}
	for i, value := range []*big.Rat{big.NewRat(1, 4), big.NewRat(7, 32)} {
		want, _, e := NewExactRationalRoot(ctx, value, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, axis := range []int{0, 2} {
			q, e := want.Compare(ctx, c.Ends[i].Parameters[axis], ExactCurvePlaneEventOptions{})
			if e != nil || !q.Proven || q.Order != 0 {
				t.Fatal("equal conic chart endpoint roots", i, axis, q, e)
			}
		}
		if end := c.Ends[i]; end.TrimEvent.EndpointMask() != 0 || end.Multiplicity != 0 || end.Bounds.Min.Z > .5 || end.Bounds.Max.Z < .5 {
			t.Fatal("interior source root and original conic support", end)
		}
	}
	freeRoot, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-7", "0", "128"}, Lower: "7/32", Upper: "1/4"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	parameter, _, e := c.ParameterAtFreeRoot(ctx, freeRoot, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal("inverse of algebraic conic coordinate", e)
	}
	// X=(9-6t-t²)/(2(9+t²)) in the restricted source parameter.
	// Substituting 128 X²=7 gives this independent quartic.
	wantParameter, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1,
		Coefficients: []string{"2025", "-3456", "450", "384", "25"}, Lower: "1/2", Upper: "3/4"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q, e := parameter.Compare(ctx, wantParameter, ExactCurvePlaneEventOptions{}); e != nil || !q.Proven || q.Order != 0 {
		t.Fatal("original conic source preimage", q, e)
	}
	projectedTube, _, _, e := proof.projectedGraphTube(ctx, [2]int{0, 2}, TubeAlgebraicGraphEndpointOptions{})
	if e != nil {
		t.Fatal(e)
	}
	projectedPending, e := projectedTube.IntersectQuadraticTrimProjected(ctx, projectedA, projectedB, 0, crossingArc, [2]int{0, 2}, TubeTrimIntersectionOptions{})
	if !errors.Is(e, ErrConvergence) || projectedPending.Complete || len(projectedPending.Coincident) == 0 {
		t.Fatal("projected coincidence", projectedPending, e)
	}
	bounds, e := projectedPending.Coincident[0].Boundary[0].Enclose(ctx, ComposedCurveEndpointOptions{})
	if e != nil || bounds.Bounds.Min.Y > .5 || bounds.Bounds.Max.Y < .5 {
		t.Fatal("coincident event restoration", bounds, e)
	}
	projectedCoincidence, e := projectedTube.IntersectQuadraticTrimProjected(ctx, projectedA, projectedB, 0, crossingArc, [2]int{0, 2}, spanOpts)
	if e != nil || !projectedCoincidence.Complete || len(projectedCoincidence.Spans) != 1 {
		t.Fatal("projected positive coincidence", projectedCoincidence, e)
	}
	for i, end := range projectedCoincidence.Spans[0].Ends {
		q, e := end.TrimEvent.Root().Compare(ctx, c.Ends[i].TrimEvent.Root(), ExactCurvePlaneEventOptions{})
		if e != nil || !q.Proven || q.Order != 0 || end.Bounds.Min.Y > .5 || end.Bounds.Max.Y < .5 {
			t.Fatal("projected original coordinates and roots", i, end, q, e)
		}
	}
	endpointTube := tube(ParameterRectangle{.5, 17.0 / 32, 7.0 / 16, 9.0 / 16}, ParameterRectangle{7.0 / 16, 9.0 / 16, 3.0 / 8, 5.0 / 8})
	endpointOnly, e := endpointTube.IntersectQuadraticTrim(ctx, original, plane, 0, outsideArc, spanOpts)
	if e != nil || !endpointOnly.Complete || len(endpointOnly.Spans) != 0 || len(endpointOnly.Hits) != 1 {
		t.Fatal("the trim only reaches the free boundary at one end", endpointOnly, e)
	}
	if end := endpointOnly.Hits[0]; !end.CoincidentEndpoint || end.Transverse || end.Tangent || end.TrimEvent.EndpointMask() != 1 {
		t.Fatal("coincident endpoint must retain distinct source semantics", end)
	}

	if _, e = proof.IntersectQuadraticTrim(ctx, original, plane, 0, trim, TubeTrimIntersectionOptions{SurfaceCurveAgreementOptions: SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: 1}}}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = proof.IntersectQuadraticTrim(cancelCtx, original, plane, 0, trim, TubeTrimIntersectionOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
