package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestBernsteinRestrictionPreservesPolynomialCancellation(t *testing.T) {
	p := make(rationalPolynomial, 9)
	for i := range p {
		p[i] = new(big.Rat).SetInt(new(big.Int).Binomial(8, int64(i)))
		if i%2 == 1 {
			p[i].Neg(p[i])
		}
	}
	b := &polynomialBudget{maxOperations: 100000, maxBits: 4096}
	bounds, e := polynomialBernsteinInterval(p, exactInterval{lo: big.NewRat(9, 10), hi: big.NewRat(99, 100)}, b)
	if e != nil {
		t.Fatal(e)
	}
	minimum := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(100), big.NewInt(8), nil))
	maximum := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(8), nil))
	if bounds.lo.Cmp(minimum) != 0 || bounds.hi.Cmp(maximum) != 0 {
		t.Fatal(bounds, minimum, maximum)
	}
}

func TestComposedPolylineFlatPolynomialTail(t *testing.T) {
	ctx := context.Background()
	net := make([][4]*big.Rat, 9)
	for i := range net {
		y := new(big.Rat)
		if i == 0 {
			y.SetInt64(1)
		}
		net[i] = [4]*big.Rat{big.NewRat(int64(i), 8), y, new(big.Rat), big.NewRat(1, 1)}
	}
	curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lower, _, e := NewExactRationalRoot(ctx, big.NewRat(9, 10), ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	upper, _ := cell.EndpointRoot(1)
	restricted, e := cell.RestrictAlgebraic(ctx, lower, upper, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	line, e := restricted.Cell.TessellateCertified(ctx, ComposedCurvePolylineOptions{Deflection: 1e-6, MaxSegments: 1})
	if e != nil || !line.Complete || len(line.Segments) != 1 || line.MaximumError > 1e-6 {
		t.Fatal(line, e)
	}
}
