package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSingleExactBezierKeepsOnlyIdenticalContiguousCellLaws(t *testing.T) {
	ctx := context.Background()
	var o ExactCurvePlaneEventOptions
	source := composedTouchFixture(t, false)
	seam, _, err := NewExactRationalRoot(ctx, big.NewRat(3, 7), o)
	if err != nil {
		t.Fatal(err)
	}
	a, err := source.EndpointRoot(0)
	if err != nil {
		t.Fatal(err)
	}
	z, err := source.EndpointRoot(1)
	if err != nil {
		t.Fatal(err)
	}
	left, err := source.RestrictAlgebraic(ctx, a, seam, o)
	if err != nil {
		t.Fatal(err)
	}
	right, err := source.RestrictAlgebraic(ctx, seam, z, o)
	if err != nil {
		t.Fatal(err)
	}
	ends := [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}
	joined := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{left.Cell, right.Cell}}
	curve, work, err := joined.SingleExactBezier(ctx, ends, o)
	if err != nil || work <= 0 {
		t.Fatalf("compatible cells: %v (%d operations)", err, work)
	}
	recovered, err := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := source.CompareSameParameter(ctx, recovered, o)
	if err != nil || !proof.Equal {
		t.Fatalf("joined curve changed its model law: %+v %v", proof, err)
	}
	// A distinct continuation still meets the first cell's endpoint: add
	// t-3/7 to one homogeneous numerator on the second cell only.
	changed := *right.Cell
	changed.power[2] = clonePolynomialForJoinedBezierTest(changed.power[2])
	for len(changed.power[2]) < 2 {
		changed.power[2] = append(changed.power[2], new(big.Rat))
	}
	changed.power[2][0].Sub(changed.power[2][0], big.NewRat(3, 7))
	changed.power[2][1].Add(changed.power[2][1], big.NewRat(1, 1))
	joined.Cells[1] = &changed
	if c, _, e := joined.SingleExactBezier(ctx, ends, o); c != nil || !errors.Is(e, ErrConvergence) {
		t.Fatalf("distinct laws joined: %v %v", c, e)
	}
	joined.Cells = []*ExactComposedCurveCell{right.Cell, left.Cell}
	if c, _, e := joined.SingleExactBezier(ctx, ends, o); c != nil || !errors.Is(e, ErrConvergence) {
		t.Fatalf("unordered cells joined: %v %v", c, e)
	}
}

func clonePolynomialForJoinedBezierTest(p rationalPolynomial) rationalPolynomial {
	out := make(rationalPolynomial, len(p))
	for i, q := range p {
		out[i] = new(big.Rat).Set(q)
	}
	return out
}
