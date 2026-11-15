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
	squared, _, e := cell.CoordinateRoot(ctx, root, 2, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal("nonlinear degree-six image", e)
	}
	want, e = ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1/2", "0", "0", "1"}, Lower: "3/4", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if comparison, e := squared.Compare(ctx, want, ExactCurvePlaneEventOptions{}); e != nil || !comparison.Proven || comparison.Order != 0 {
		t.Fatal("squaring the sixth root should produce the exact cube root", comparison, e)
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

func TestComposedCoordinateRootHigherRationalImages(t *testing.T) {
	ctx := context.Background()
	for _, degree := range []int{5, 8} {
		p, wantCoefficients := make([]string, degree+1), make([]string, degree+1)
		for i := range p {
			p[i] = "0"
			// X = 1/(1+t), so 2*t^degree-1 gives
			// 2*(1-X)^degree-X^degree exactly.
			value := new(big.Int).Binomial(int64(degree), int64(i))
			value.Lsh(value, 1)
			if i%2 != 0 {
				value.Neg(value)
			}
			if i == degree {
				value.Sub(value, big.NewInt(1))
			}
			wantCoefficients[i] = value.String()
		}
		p[0], p[degree] = "-1", "2"
		root, err := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: p, Lower: "4/5", Upper: "1"}, ExactRootCodecOptions{})
		if err != nil {
			t.Fatal(err)
		}
		curve, err := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{
			{big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
			{big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(2, 1)},
		}, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		cell, err := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		want, err := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: wantCoefficients, Lower: "1/2", Upper: "2/3"}, ExactRootCodecOptions{})
		if err != nil {
			t.Fatal(err)
		}
		answer, operations, err := cell.CoordinateRoot(ctx, root, 0, ExactCurvePlaneEventOptions{})
		if err != nil || operations < 2 {
			t.Fatal("certified higher-degree rational image", degree, answer, operations, err)
		}
		comparison, err := answer.Compare(ctx, want, ExactCurvePlaneEventOptions{})
		if err != nil || !comparison.Proven || comparison.Order != 0 {
			t.Fatal("independent fractional-polynomial image", degree, comparison, err)
		}
		if partial, _, err := cell.CoordinateRoot(ctx, root, 0, ExactCurvePlaneEventOptions{MaxOperations: operations - 1}); !errors.Is(err, ErrConvergence) || partial != nil {
			t.Fatal("insufficient polynomial budget must be atomic", degree, partial, err)
		}
	}
}
