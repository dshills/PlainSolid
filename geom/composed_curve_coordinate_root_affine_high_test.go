package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestComposedCoordinateRootDegreeSixAffineResidue(t *testing.T) {
	ctx := context.Background()
	root, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1/2", "0", "0", "0", "0", "0", "1"}, Lower: "4/5", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// X=3-2t reverses order; Y=t^6 is constant only modulo the source equation.
	cell := &ExactComposedCurveCell{power: [4]rationalPolynomial{
		{big.NewRat(3, 1), big.NewRat(-2, 1)},
		{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
		{new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(1, 1)},
	}}
	for i, q := range []int64{0, 1} {
		r, _, e := NewExactRationalRoot(ctx, big.NewRat(q, 1), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cell.endpoints[i] = r.root
	}
	image, ops, e := cell.CoordinateRoot(ctx, root, 0, ExactCurvePlaneEventOptions{})
	if e != nil || ops == 0 {
		t.Fatal("affine image", e)
	}
	want, _, e := root.AffineMap(ctx, big.NewRat(3, 1), big.NewRat(-2, 1), ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	order, e := image.Compare(ctx, want, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	constant, _, e := cell.CoordinateRoot(ctx, root, 1, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	kind, e := constant.Classify(ctx, ExactCurvePlaneEventOptions{})
	if e != nil || kind.Kind != ExactRootKindRational || kind.Rational.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(kind, e)
	}
	if result, _, e := cell.CoordinateRoot(ctx, root, 2, ExactCurvePlaneEventOptions{}); !errors.Is(e, ErrConvergence) || result != nil {
		t.Fatal("nonlinear degree-six image", result, e)
	}
	if result, _, e := cell.CoordinateRoot(ctx, root, 0, ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) || result != nil {
		t.Fatal("quota", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if result, _, e := cell.CoordinateRoot(cancelled, root, 0, ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) || result != nil {
		t.Fatal("cancel", e)
	}
}
