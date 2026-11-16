package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicCurveExactJetRetainsFarCycleAndPhysicalParameter(t *testing.T) {
	ctx := context.Background()
	c, err := NewPeriodicBSplineWithExactWeights(3,
		[]Vec3{{1, 2, 3}, {0, -2, 1}, {-3, 1, 2}, {2, 1, 0}, {4, 0, -1}},
		[]float64{1, .75, 3, 1.25, 2}, -.25, 1.75)
	if err != nil {
		t.Fatal(err)
	}
	cycle := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 100))
	seam := new(big.Rat).Add(big.NewRat(-1, 4), new(big.Rat).Mul(new(big.Rat).SetInt(cycle), big.NewRat(7, 4)))
	for i, offset := range []*big.Rat{big.NewRat(-1, 97), new(big.Rat), big.NewRat(7, 20), big.NewRat(81, 97)} {
		at := new(big.Rat).Add(seam, offset)
		got, err := c.EvaluateRational(ctx, at, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		owner := new(big.Int).Set(cycle)
		if i == 0 {
			owner.Sub(owner, big.NewInt(1))
		}
		wantSpan := [4]int{4, 0, 1, 2}[i]
		if got.Cycle.Cmp(owner) != 0 || got.Span != wantSpan ||
			got.Domain.Lower.Cmp(at) > 0 || got.Domain.Upper.Cmp(at) <= 0 ||
			new(big.Rat).Sub(got.Domain.Upper, got.Domain.Lower).Cmp(big.NewRat(7, 20)) != 0 {
			t.Fatal("wrong original cyclic owner", got, owner, at)
		}
		want, slope := periodicRefinedCurveJet(t, c, at)
		for axis := range want {
			if got.Point[axis].Cmp(want[axis]) != 0 || got.Derivative[axis].Cmp(slope[axis]) != 0 {
				t.Fatal("rational de Boor jet differs from the original exact span restriction", got.Point, got.Derivative, want, slope)
			}
		}
		assertExactPointInBox(t, got.Point, got.PointBounds)
		assertExactPointInBox(t, got.Derivative, got.DerivativeBounds)
		if partial, err := c.EvaluateRational(ctx, at, SurfacePatchOptions{MaxOperations: got.ExactOperations - 1}); !errors.Is(err, ErrConvergence) ||
			partial.Point[0] != nil || partial.Cycle != nil {
			t.Fatal("one-short proof leaked a cyclic point", partial, err)
		}
		got.Point[0].SetInt64(99)
		got.Domain.Lower.SetInt64(100)
		got.Cycle.SetInt64(101)
		again, err := c.EvaluateRational(ctx, at, SurfacePatchOptions{})
		if err != nil || again.Cycle.Cmp(owner) != 0 || again.Point[0].Cmp(want[0]) != 0 || again.Domain.Lower.Cmp(at) > 0 {
			t.Fatal("returned exact data aliases the original curve or parameter", again, err)
		}
	}
}

func TestPeriodicCurveExactSeamUsesRightJet(t *testing.T) {
	ctx := context.Background()
	c, err := NewPeriodicBSplineWithExactWeights(1, []Vec3{{0, 0, 0}, {2, 0, 0}, {2, 2, 0}}, []float64{1, 3, 1}, -.25, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i, at := range []*big.Rat{big.NewRat(-1, 4), big.NewRat(11, 4), big.NewRat(23, 4)} {
		jet, err := c.EvaluateRational(ctx, at, SurfacePatchOptions{})
		if err != nil || jet.Span != 0 || jet.Cycle.Cmp(big.NewInt(int64(i))) != 0 ||
			jet.Point[0].Sign() != 0 || jet.Point[1].Sign() != 0 ||
			jet.Derivative[0].Cmp(big.NewRat(6, 1)) != 0 || jet.Derivative[1].Sign() != 0 {
			t.Fatal("one-sided physical seam law changed", jet, err)
		}
	}
	if partial, err := c.EvaluateRational(ctx, big.NewRat(1, 2), SurfacePatchOptions{MaxCoefficientBits: 1}); !errors.Is(err, ErrInvalid) ||
		partial.Cycle != nil {
		t.Fatal("invalid coefficient bound produced an evaluation", partial, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, err := c.EvaluateRational(cancelled, new(big.Rat), SurfacePatchOptions{}); !errors.Is(err, context.Canceled) ||
		partial.Point[0] != nil {
		t.Fatal("cancelled evaluation returned a point", partial, err)
	}
}
