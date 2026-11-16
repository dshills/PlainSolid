package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactNonuniformKnotInsertion(t *testing.T) {
	ctx := context.Background()
	original, err := NewPeriodicBSplineWithExactWeights(3,
		[]Vec3{{1, 0, 0}, {3, 2, 1}, {2, -1, 3}, {-1, 1, 2}, {-2, 0, 1}},
		[]float64{1, 3, 2, 4, 5}, -.25, 1.75)
	if err != nil {
		t.Fatal(err)
	}
	c, err := original.ExactCyclicSpline(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	knots := c.Knots()
	if len(knots) != 6 || knots[0].Cmp(big.NewRat(-1, 4)) != 0 || knots[1].Cmp(big.NewRat(1, 10)) != 0 || knots[5].Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("wrong original physical lattice", knots)
	}
	first := new(big.Rat).Add(knots[0], big.NewRat(1, 19))
	last := new(big.Rat).Sub(knots[5], big.NewRat(1, 1295))
	period := big.NewRat(7, 4)
	hugeCycle := new(big.Int).Lsh(big.NewInt(1), 100)
	negativeShift := new(big.Rat).Mul(period, new(big.Rat).SetInt(new(big.Int).Neg(hugeCycle)))
	stations := []*big.Rat{new(big.Rat).Add(first, negativeShift), last, knots[1], knots[1]}
	for i, x := range stations {
		refined, err := c.InsertKnot(ctx, x, SurfacePatchOptions{})
		if err != nil {
			t.Fatalf("insertion %d at %s: %v", i, x, err)
		}
		if len(c.Knots()) != 6+i || len(refined.HomogeneousControls()) != 6+i || refined.Degree() != 3 {
			t.Fatal("insertion changed the source, degree or cyclic net count", i)
		}
		c = refined
		newKnots := c.Knots()
		for j, a := range newKnots {
			checkExactPeriodicCurveAgainstBezier(t, ctx, original, c, a)
			checkExactPeriodicCurveAgainstBezier(t, ctx, original, c, new(big.Rat).Add(a, negativeShift))
			if j > 0 && a.Cmp(newKnots[j-1]) != 0 {
				b := new(big.Rat).Quo(new(big.Rat).Add(a, newKnots[j-1]), big.NewRat(2, 1))
				checkExactPeriodicCurveAgainstBezier(t, ctx, original, c, b)
			}
		}
	}
	for _, x := range []*big.Rat{knots[1], knots[0], new(big.Rat).Add(knots[5], negativeShift)} {
		if got, err := c.InsertKnot(ctx, x, SurfacePatchOptions{}); got != nil || !errors.Is(err, ErrDomain) {
			t.Fatal("insertion violated the maximum multiplicity or seam", x, err)
		}
	}
	if got, err := c.InsertKnot(ctx, big.NewRat(1, 7), SurfacePatchOptions{MaxOperations: 58}); got != nil || !errors.Is(err, ErrConvergence) {
		t.Fatal("insertion escaped its budget", err)
	}
	if got, err := c.InsertKnot(ctx, big.NewRat(1, 7), SurfacePatchOptions{MaxPoles: 9}); got != nil || !errors.Is(err, ErrConvergence) {
		t.Fatal("insertion exceeded the pole limit", err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if got, err := c.InsertKnot(cancelled, big.NewRat(1, 7), SurfacePatchOptions{}); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled insertion published a curve", err)
	}
}

func checkExactPeriodicCurveAgainstBezier(t *testing.T, ctx context.Context, original *PeriodicBSpline, exact *ExactPeriodicSplineCurve, x *big.Rat) {
	t.Helper()
	p, d := periodicRefinedCurveJet(t, original, x)
	got, err := exact.EvaluateRational(ctx, x, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(x, err)
	}
	if got.Domain.Lower.Cmp(x) > 0 || got.Domain.Upper.Cmp(x) <= 0 {
		t.Fatalf("parameter %s assigned to span [%s, %s)", x, got.Domain.Lower, got.Domain.Upper)
	}
	for axis := range p {
		if got.Point[axis].Cmp(p[axis]) != 0 || got.Derivative[axis].Cmp(d[axis]) != 0 {
			t.Fatalf("parameter %s coordinate %d: got %s, %s; want %s, %s", x, axis, got.Point[axis], got.Derivative[axis], p[axis], d[axis])
		}
	}
}

func TestPeriodicExactAuthoredNonuniformCurve(t *testing.T) {
	ctx := context.Background()
	knots := []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 1)}
	control := [][4]*big.Rat{}
	for _, h := range [][4]int64{{0, 0, 0, 1}, {6, 0, 0, 3}, {2, 2, 0, 1}} {
		control = append(control, [4]*big.Rat{big.NewRat(h[0], 1), big.NewRat(h[1], 1), big.NewRat(h[2], 1), big.NewRat(h[3], 1)})
	}
	c, err := NewExactPeriodicSplineCurve(ctx, 1, knots, control, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	knots[1].SetInt64(0)
	control[1][3].SetInt64(0)
	c.Knots()[1].SetInt64(0)
	c.HomogeneousControls()[1][3].SetInt64(0)
	for _, tc := range []struct {
		x, px, dx, dy *big.Rat
		span          int
	}{{big.NewRat(1, 6), big.NewRat(3, 2), big.NewRat(9, 2), new(big.Rat), 0},
		{big.NewRat(1, 3), big.NewRat(2, 1), new(big.Rat), big.NewRat(4, 1), 1}} {
		j, err := c.EvaluateRational(ctx, tc.x, SurfacePatchOptions{})
		if err != nil || j.Point[0].Cmp(tc.px) != 0 || j.Derivative[0].Cmp(tc.dx) != 0 || j.Derivative[1].Cmp(tc.dy) != 0 || j.Span != tc.span {
			t.Fatalf("wrong exact authored jet at %s: %+v: %v", tc.x, j, err)
		}
		short := SurfacePatchOptions{MaxOperations: j.ExactOperations - 1}
		if got, err := c.EvaluateRational(ctx, tc.x, short); !errors.Is(err, ErrConvergence) || got.Point[0] != nil || got.Cycle != nil {
			t.Fatal("limited evaluation published an uncertified jet", err)
		}
	}
	if got, err := NewExactPeriodicSplineCurve(ctx, 1, knots, c.HomogeneousControls(), SurfacePatchOptions{}); got != nil || !errors.Is(err, ErrInvalid) {
		t.Fatal("accepted a repeated seam", err)
	}
	if got, err := NewExactPeriodicSplineCurve(ctx, 1, c.Knots(), control, SurfacePatchOptions{}); got != nil || !errors.Is(err, ErrInvalid) {
		t.Fatal("accepted a zero weight", err)
	}
	knots = []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 3), big.NewRat(1, 1)}
	if got, err := NewExactPeriodicSplineCurve(ctx, 1, knots, c.HomogeneousControls(), SurfacePatchOptions{}); got != nil || !errors.Is(err, ErrInvalid) {
		t.Fatal("accepted interior multiplicity above degree", err)
	}
}
