package intersect

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestTubeAlgebraicEndpointAdapterAndEquality(t *testing.T) {
	ctx := context.Background()
	knots := []float64{0, 0, 0, 1, 1, 1}
	poles := make([]geom.Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles[3*i+j] = geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z}
		}
	}
	source, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{0, 0, 1, 1}
	plane, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}, {X: 1, Y: 1, Z: .5}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	segment := func(lo, hi float64) TubeSegment {
		a, e := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a, e = a.Trim(geom.ParameterRectangle{UMin: lo, UMax: hi, VMin: 5.0 / 8, VMax: 11.0 / 16})
		if e != nil {
			t.Fatal(e)
		}
		b, e := geom.NewSurfacePatch(plane, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e = b.Trim(geom.ParameterRectangle{UMin: lo - 1.0/32, UMax: hi + 1.0/32, VMin: 19.0 / 32, VMax: 23.0 / 32})
		if e != nil {
			t.Fatal(e)
		}
		proof, e := geom.NewVerifiedSurfaceTube(a, b, geom.SurfacePairTubeOptions{FreeAxis: 0})
		if e != nil {
			t.Fatal(e)
		}
		return TubeSegment{a: source, b: plane, pa: a, pb: b, proof: proof, axis: 0, lo: lo, hi: hi}
	}
	root, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"-7/16", "0", "1"}, Lower: "5/8", Upper: "11/16"}, geom.ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	left, right := segment(7.0/32, .25), segment(.25, 9.0/32)
	trim, e := geom.NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(15, 64), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(15, 64), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(15, 64), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	crossings, e := left.IntersectQuadraticTrim(ctx, 0, trim, geom.TubeTrimIntersectionOptions{})
	if e != nil || !crossings.Complete || len(crossings.Hits) != 1 || !crossings.Hits[0].Transverse {
		t.Fatal(crossings, e)
	}

	comparison, e := CompareTubeAlgebraicGraphEndpoints(ctx, left, right, 1, 0, root, root, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !comparison.ProvenEqual || !comparison.SupportsEqual || comparison.ParametersDiffer || comparison.Left.ModelEvent == nil || comparison.Right.ModelEvent == nil {
		t.Fatal(comparison, e)
	}
	left.reversed = true
	reversed, e := left.CertifyAlgebraicGraphEndpoint(ctx, 0, root, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !reversed.Proven {
		t.Fatal(reversed, e)
	}
	automatic, e := left.DiscoverAlgebraicGraphEndpoint(ctx, 0, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !automatic.Proven {
		t.Fatal(automatic, e)
	}
	order, e := automatic.Parameters[1].Compare(ctx, root, geom.ExactCurvePlaneEventOptions{})
	if e != nil || order.Order != 0 {
		t.Fatal(order, e)
	}

	axis, domain, isReversed, e := left.ExactFreeDomain()
	if e != nil || axis != 0 || !isReversed || domain.Lower.Cmp(big.NewRat(7, 32)) != 0 {
		t.Fatal(axis, domain, isReversed, e)
	}
	section, e := left.EvaluateRationalGraph(ctx, big.NewRat(7, 30), geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !section.Proven {
		t.Fatal(section, e)
	}
	membership, e := left.ContainsFreeRoot(ctx, section.Parameters[0], geom.ExactCurvePlaneEventOptions{})
	if e != nil || !membership.Proven || !membership.Inside {
		t.Fatal(membership, e)
	}

	left.reversed = false
	other, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"-463/1024", "0", "1"}, Lower: "5/8", Upper: "11/16"}, geom.ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	different, e := CompareTubeAlgebraicGraphEndpoints(ctx, left, right, 0, 0, other, root, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || different.ProvenEqual || !different.SupportsEqual || !different.ParametersDiffer {
		t.Fatal(different, e)
	}
	// Independent but identical original surface objects must be compared exactly.
	duplicate, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	right.a = duplicate
	identical, e := CompareTubeAlgebraicGraphEndpoints(ctx, left, right, 1, 0, root, root, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !identical.ProvenEqual {
		t.Fatal(identical, e)
	}
	if _, e = left.CertifyAlgebraicGraphEndpoint(ctx, 2, root, geom.TubeAlgebraicGraphEndpointOptions{}); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = CompareTubeAlgebraicGraphEndpoints(cancelCtx, left, right, 1, 0, root, root, geom.TubeAlgebraicGraphEndpointOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
