package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestAlgebraicEndpointModelBounds(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, endpoint := range []int{1, 0} {
		r, e := cells.Cells[i].EncloseEndpoint(context.Background(), endpoint, ComposedCurveEndpointOptions{Tolerance: 1e-10})
		if e != nil || !r.HasBounds || !r.HasRepresentative || !r.Converged || r.ErrorUpper > 1e-10 {
			t.Fatal(r, e)
		}
		// The knot event is t=sqrt(1/2), and independent model coordinates are
		// x=1/2, y=t, z=3/2-t for this authored graph.
		expected := Vec3{.5, math.Sqrt(.5), 1.5 - math.Sqrt(.5)}
		if r.Point.Sub(expected).Length() > r.ErrorUpper+1e-15 {
			t.Fatal("representative error")
		}
		if r.Bounds.Min.X > .5 || r.Bounds.Max.X < .5 || r.Coordinates[0].Lower.Cmp(big.NewRat(1, 2)) > 0 || r.Coordinates[0].Upper.Cmp(big.NewRat(1, 2)) < 0 {
			t.Fatal("exact x enclosure")
		}
		for _, t0 := range []*big.Rat{r.Parameter.Lower, r.Parameter.Upper} {
			power, _, e := cells.Cells[i].HomogeneousPower(context.Background(), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b := &polynomialBudget{maxOperations: 100000, maxBits: 4096}
			w, _ := polynomialValue(power[3], t0, b)
			for axis := 0; axis < 3; axis++ {
				n, _ := polynomialValue(power[axis], t0, b)
				value := new(big.Rat).Quo(n, w)
				if value.Cmp(r.Coordinates[axis].Lower) < 0 || value.Cmp(r.Coordinates[axis].Upper) > 0 {
					t.Fatal("continuation endpoint enclosure")
				}
			}
		}
	}
}
func TestAlgebraicEndpointPartialBoundsAndRounding(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := cells.Cells[0].EncloseEndpoint(context.Background(), 1, ComposedCurveEndpointOptions{ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxSubdivisions: 1}, Tolerance: 1e-30})
	if e == nil || r.Converged || !r.HasBounds || !r.HasRepresentative {
		t.Fatal(r, e)
	}
	// A rational endpoint at t=0 with exact coordinate 1/3 cannot meet a
	// sub-roundoff tolerance; its retained error explicitly includes rounding.
	cell := &ExactComposedCurveCell{power: [4]rationalPolynomial{{big.NewRat(1, 3)}, nil, nil, {big.NewRat(1, 1)}}, endpoints: [2]*ExactGraphLineRoot{{polynomial: rationalPolynomial{new(big.Rat), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat), new(big.Rat)}}, cells.Cells[0].endpoints[1]}}
	r, e = cell.EncloseEndpoint(context.Background(), 0, ComposedCurveEndpointOptions{Tolerance: 1e-30})
	if e == nil || r.Converged || !r.HasBounds || r.ErrorUpper <= 0 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = cell.EncloseEndpoint(ctx, 0, ComposedCurveEndpointOptions{})
	if e != context.Canceled {
		t.Fatal(e)
	}
}
