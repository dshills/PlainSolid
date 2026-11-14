package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestComposedCoordinateRootQuadraticRationalImage(t *testing.T) {
	ctx := context.Background()
	curve, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(2, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	source := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	image, ops, e := cell.CoordinateRoot(ctx, source, 0, ExactCurvePlaneEventOptions{})
	if e != nil || ops == 0 {
		t.Fatal(image, ops, e)
	}
	// x=t/(1+t), t²=1/2 implies x²+2x-1=0, selected x>0.
	expected, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "2", "1"}, Lower: "1/3", Upper: "1/2"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	order, e := image.Compare(ctx, expected, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	_, uv, _, _ := algebraicSeamFixture(t)
	quadratic, e := NewExactComposedBezierCell(ctx, uv, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	constant, _, e := quadratic.CoordinateRoot(ctx, source, 0, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	classification, e := constant.Classify(ctx, ExactCurvePlaneEventOptions{})
	if e != nil || classification.Kind != ExactRootKindRational || classification.Rational.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(classification, e)
	}
	upper := importTestRoot(t, big.NewRat(-1, 2), false, "1/2", "1/2")
	lower, _ := cell.EndpointRoot(0)
	cut, e := cell.RestrictAlgebraic(ctx, lower, upper, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = cut.Cell.CoordinateRoot(ctx, source, 0, ExactCurvePlaneEventOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if _, _, e = cell.CoordinateRoot(ctx, source, 0, ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e = cell.CoordinateRoot(cancelCtx, source, 0, ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestComposedCoordinateRootQuarticConjugateBinding(t *testing.T) {
	ctx := context.Background()
	root, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1/2", "0", "0", "0", "1"}, Lower: "3/4", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, uv, _, _ := algebraicSeamFixture(t)
	square, e := NewExactComposedBezierCell(ctx, uv, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	image, _, e := square.CoordinateRoot(ctx, root, 0, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	expected := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	order, e := image.Compare(ctx, expected, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	curve, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(2, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	fraction, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rationalImage, work, subdivisions, e := fraction.CoordinateRootWithWork(ctx, root, 0, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if work == 0 || subdivisions == 0 {
		t.Fatal("quartic image isolation must report consumed arithmetic and subdivisions", work, subdivisions)
	}
	// x=t/(1+t): 2x^4=(1-x)^4. Other conjugate images must not be selected.
	expectedImage, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "4", "-6", "4", "1"}, Lower: "2/5", Upper: "1/2"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	order, e = rationalImage.Compare(ctx, expectedImage, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	// Use a wide, still single-root isolator and a rational coordinate
	// with a denominator zero at the isolator's lower rational bound.
	// Image selection then needs several subdivisions.
	wide, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1/2", "0", "0", "0", "1"}, Lower: "0", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	nearPole := *fraction
	nearPole.power[0] = rationalPolynomial{big.NewRat(1, 1)}
	nearPole.power[3] = rationalPolynomial{big.NewRat(-4, 5), big.NewRat(1, 1)}
	frame, w, s, e := nearPole.CoordinateRootWithWork(ctx, wide, 0, ExactCurvePlaneEventOptions{})
	if e != nil || frame == nil || w == 0 || s < 2 {
		t.Fatal("expected genuine subdivision during characteristic image selection", frame, w, s, e)
	}
	if incomplete, _, _, err := nearPole.CoordinateRootWithWork(ctx, wide, 0, ExactCurvePlaneEventOptions{MaxSubdivisions: s - 1}); !errors.Is(err, ErrConvergence) || incomplete != nil {
		t.Fatal("an exhausted subdivision budget cannot publish an image", incomplete, err)
	}
	limited, _, e := fraction.CoordinateRoot(ctx, root, 0, ExactCurvePlaneEventOptions{MaxOperations: 20})
	if !errors.Is(e, ErrConvergence) || limited != nil {
		t.Fatal(limited, e)
	}
}
