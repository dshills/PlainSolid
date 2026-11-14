package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestTubeAlgebraicGraphEndpoint(t *testing.T) {
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
	a, e := NewSurfacePatch(spline, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e = a.Trim(ParameterRectangle{UMin: 7.0 / 32, UMax: .25, VMin: 5.0 / 8, VMax: 11.0 / 16})
	if e != nil {
		t.Fatal(e)
	}
	c := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, .5} })
	c, e = c.Trim(ParameterRectangle{UMin: 3.0 / 16, UMax: 9.0 / 32, VMin: 19.0 / 32, VMax: 23.0 / 32})
	if e != nil {
		t.Fatal(e)
	}
	tube, e := NewVerifiedSurfaceTube(a, c, SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil {
		t.Fatal(e)
	}
	root := importTestRoot(t, big.NewRat(-7, 16), true, "5/8", "11/16")
	r, e := tube.CertifyAlgebraicGraphEndpoint(ctx, true, root, TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !r.Proven || !r.SourceIsoTransverse || r.SourceIsoMultiplicity != 1 {
		t.Fatal(r, e)
	}
	discovered, e := tube.DiscoverAlgebraicGraphEndpoint(ctx, true, TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !discovered.Proven || !discovered.SourceIsoTransverse || discovered.ModelEvent == nil {
		t.Fatal(discovered, e)
	}
	for axis := 0; axis < 4; axis++ {
		order, e := discovered.Parameters[axis].Compare(ctx, r.Parameters[axis], ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal(order, e)
		}
	}
	lowerEndpoint, e := tube.DiscoverAlgebraicGraphEndpoint(ctx, false, TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !lowerEndpoint.Proven {
		t.Fatal(lowerEndpoint, e)
	}
	if _, e = tube.CertifyAlgebraicGraphEndpoint(ctx, true, nil, TubeAlgebraicGraphEndpointOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}

	section, e := tube.EvaluateRationalGraph(ctx, big.NewRat(7, 30), TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !section.Proven {
		t.Fatal(section, e)
	}
	projected, e := tube.EvaluateRationalProjectedGraph(ctx, big.NewRat(7, 30), [2]int{0, 1}, TubeAlgebraicGraphEndpointOptions{
		SurfaceCurveAgreementOptions: SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: section.ExactOperations}},
	})
	if e != nil || !projected.Proven || projected.ExactOperations != section.ExactOperations || projected.PointBounds != section.PointBounds {
		t.Fatal("reuse the original branch within the new proof's exact work allowance", projected, e)
	}
	for i, want := range section.Parameters {
		match, e := projected.Parameters[i].Compare(ctx, want, ExactCurvePlaneEventOptions{})
		if e != nil || !match.Proven || match.Order != 0 {
			t.Fatal("identity projection changes original chart root", i, match, e)
		}
	}
	expectedSection := importTestRoot(t, big.NewRat(-401, 900), true, "5/8", "11/16")
	order, e := section.Parameters[1].Compare(ctx, expectedSection, ExactCurvePlaneEventOptions{})
	if e != nil || order.Order != 0 {
		t.Fatal(order, e)
	}
	value, e := section.Parameters[0].Classify(ctx, ExactCurvePlaneEventOptions{})
	if e != nil || value.Rational == nil || value.Rational.Cmp(big.NewRat(7, 30)) != 0 {
		t.Fatal(value, e)
	}
	axis, domain, e := tube.ExactFreeDomain()
	if e != nil || axis != 0 || domain.Lower.Cmp(big.NewRat(7, 32)) != 0 || domain.Upper.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(axis, domain, e)
	}
	domain.Lower.SetInt64(99)
	inside := importTestRoot(t, big.NewRat(-1, 18), true, "7/32", "1/4")
	membership, e := tube.ContainsFreeRoot(ctx, inside, ExactCurvePlaneEventOptions{})
	if e != nil || !membership.Proven || !membership.Inside || membership.BoundaryMask != 0 {
		t.Fatal(membership, e)
	}
	endpoint := importTestRoot(t, big.NewRat(-1, 4), false, "1/4", "1/4")
	membership, e = tube.ContainsFreeRoot(ctx, endpoint, ExactCurvePlaneEventOptions{})
	if e != nil || !membership.Inside || membership.BoundaryMask != 2 {
		t.Fatal(membership, e)
	}
	if _, e = tube.EvaluateRationalGraph(ctx, big.NewRat(1, 3), TubeAlgebraicGraphEndpointOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}

	for _, axis := range []int{1, 3} {
		order, e := r.Parameters[axis].Compare(ctx, root, ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal(order, e)
		}
	}
	if r.PointBounds.Min.Z > .5 || r.PointBounds.Max.Z < .5 || r.ParameterBoundsA.Min.X != .25 || r.ParameterBoundsA.Max.X != .25 {
		t.Fatal(r)
	}
	// Swapping ordered owners still preserves their original parameter slots.
	reversed, e := NewVerifiedSurfaceTube(c, a, SurfacePairTubeOptions{FreeAxis: 2})
	if e != nil {
		t.Fatal(e)
	}
	rr, e := reversed.CertifyAlgebraicGraphEndpoint(ctx, true, root, TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !rr.Proven {
		t.Fatal(rr, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	wrong := importTestRoot(t, new(big.Rat).Sub(big.NewRat(-7, 16), tiny), true, "5/8", "11/16")
	if q, e := tube.CertifyAlgebraicGraphEndpoint(ctx, true, wrong, TubeAlgebraicGraphEndpointOptions{}); !errors.Is(e, ErrDomain) || q.Proven {
		t.Fatal(q, e)
	}
	if _, e = tube.CertifyAlgebraicGraphEndpoint(ctx, true, root, TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: 1}}}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = tube.CertifyAlgebraicGraphEndpoint(cancelCtx, true, root, TubeAlgebraicGraphEndpointOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
