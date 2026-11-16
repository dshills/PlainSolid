package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicHigherRationalDerivativesFarCycle(t *testing.T) {
	ctx := context.Background()
	curve, err := NewExactPeriodicSplineCurve(ctx, 1, []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(3, 1)}, [][4]*big.Rat{
		{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)},
		{big.NewRat(2, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(2, 1)},
	}, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cycle := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 100))
	t0 := new(big.Rat).Add(big.NewRat(1, 2), new(big.Rat).Mul(new(big.Rat).SetInt(cycle), big.NewRat(3, 1)))
	got, err := curve.EvaluateDerivativesRational(ctx, t0, 16, SurfacePatchOptions{})
	if err != nil || got.Cycle.Cmp(cycle) != 0 || got.Span != 0 || got.Domain.Upper.Cmp(new(big.Rat).Add(t0, big.NewRat(1, 2))) != 0 {
		t.Fatal("lost the original far-cyclic span", got, err)
	}
	checkDegreeOneRationalDerivatives(t, got, 1)
	first, err := curve.EvaluateRational(ctx, t0, SurfacePatchOptions{})
	if err != nil || first.Point[0].Cmp(got.Derivatives[0][0]) != 0 || first.Derivative[0].Cmp(got.Derivatives[1][0]) != 0 {
		t.Fatal("the existing point/first jet disagrees at the same owned span", err)
	}
	if got, err := curve.EvaluateDerivativesRational(ctx, t0, 16, SurfacePatchOptions{MaxOperations: got.ExactOperations - 1}); !errors.Is(err, ErrConvergence) || len(got.Derivatives) != 0 {
		t.Fatal("one short derivative budget published a partial jet", err)
	}
	// The uniform source has the same rational law on its first span,
	// but the original physical parameter is scaled by exactly two.
	uniform, err := NewPeriodicBSplineWithExactWeights(1, []Vec3{{}, {X: 1}}, []float64{1, 2}, -2, 4)
	if err != nil {
		t.Fatal(err)
	}
	g, err := uniform.EvaluateDerivativesRational(ctx, big.NewRat(-1, 1), 16, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	checkDegreeOneRationalDerivatives(t, g, 2)
	if g, err := uniform.EvaluateDerivativesRational(ctx, big.NewRat(-1, 1), 17, SurfacePatchOptions{}); !errors.Is(err, ErrInvalid) || len(g.Derivatives) != 0 {
		t.Fatal("unbounded derivative request was admitted", err)
	}
}

func checkDegreeOneRationalDerivatives(t *testing.T, g RationalPeriodicCurveDerivatives, width int64) {
	t.Helper()
	if len(g.Derivatives) != 17 || g.Derivatives[0][0].Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal("first span did not reproduce 2s/(1+s)", g.Derivatives)
	}
	var factorial int64 = 1
	for k := 1; k <= 16; k++ {
		factorial *= int64(k)
		sign := int64(1 - 2*((k+1)%2))
		den := new(big.Int).Exp(big.NewInt(3), big.NewInt(int64(k+1)), nil)
		den.Mul(den, new(big.Int).Exp(big.NewInt(width), big.NewInt(int64(k)), nil))
		num := new(big.Int).Lsh(big.NewInt(sign*2*factorial), uint(k+1))
		want := new(big.Rat).SetFrac(num, den)
		if g.Derivatives[k][0].Cmp(want) != 0 || g.Derivatives[k][1].Sign() != 0 || g.Derivatives[k][2].Sign() != 0 {
			t.Fatal("physical higher derivative differs from the analytic quotient", width, k, g.Derivatives[k], want)
		}
	}
}

func TestPeriodicHigherDerivativesAtRepeatedKnot(t *testing.T) {
	ctx := context.Background()
	control := func(x, y, z int64) [4]*big.Rat {
		return [4]*big.Rat{big.NewRat(x, 2), big.NewRat(y, 2), big.NewRat(z, 2), big.NewRat(1, 1)}
	}
	s, err := NewExactPeriodicSplineCurve(ctx, 2, []*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(2, 1)},
		[][4]*big.Rat{control(18, 6, 4), control(0, 2, 0), control(0, 4, 1), control(2, 6, 4)}, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		t      *big.Rat
		span   int
		firstX int64
		accelX int64
	}{{big.NewRat(1, 2), 1, 1, 2}, {big.NewRat(1, 1), 3, 16, -34}, {big.NewRat(2, 1), 1, 0, 2}} {
		g, err := s.EvaluateDerivativesRational(ctx, c.t, 4, SurfacePatchOptions{})
		if err != nil || g.Span != c.span || g.Derivatives[1][0].Cmp(big.NewRat(c.firstX, 1)) != 0 ||
			g.Derivatives[2][0].Cmp(big.NewRat(c.accelX, 1)) != 0 || g.Derivatives[3][0].Sign() != 0 {
			t.Fatal("repeated knot did not select the original right polynomial", c, g, err)
		}
	}
}
