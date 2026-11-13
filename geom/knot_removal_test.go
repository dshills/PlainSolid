package geom

import (
	"errors"
	"math/big"
	"math/rand"
	"testing"
)

// exactCurveOracle evaluates via rational Cox-de Boor basis functions. It is
// independent of both the production floating evaluator and the reinsertion
// control-net proof, and includes actual stored float64 input values exactly.
func exactCurveOracle(curve *BSpline, t float64) [3]*big.Rat {
	_, b := curve.Domain()
	if t == b {
		p := curve.poles[len(curve.poles)-1]
		return [3]*big.Rat{new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y), new(big.Rat).SetFloat64(p.Z)}
	}
	parameter := new(big.Rat).SetFloat64(t)
	knots := make([]*big.Rat, len(curve.knots))
	for i, k := range curve.knots {
		knots[i] = new(big.Rat).SetFloat64(k)
	}
	basis := make([]*big.Rat, len(knots)-1)
	for i := range basis {
		basis[i] = new(big.Rat)
		if curve.knots[i] <= t && t < curve.knots[i+1] {
			basis[i].SetInt64(1)
		}
	}
	for degree := 1; degree <= curve.degree; degree++ {
		next := make([]*big.Rat, len(basis)-1)
		for i := range next {
			next[i] = new(big.Rat)
			leftDen := new(big.Rat).Sub(knots[i+degree], knots[i])
			if leftDen.Sign() != 0 {
				factor := new(big.Rat).Quo(new(big.Rat).Sub(parameter, knots[i]), leftDen)
				next[i].Add(next[i], new(big.Rat).Mul(factor, basis[i]))
			}
			rightDen := new(big.Rat).Sub(knots[i+degree+1], knots[i+1])
			if rightDen.Sign() != 0 {
				factor := new(big.Rat).Quo(new(big.Rat).Sub(knots[i+degree+1], parameter), rightDen)
				next[i].Add(next[i], new(big.Rat).Mul(factor, basis[i+1]))
			}
		}
		basis = next
	}
	numerator := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	denominator := new(big.Rat)
	for i, p := range curve.poles {
		weight := new(big.Rat).Mul(basis[i], new(big.Rat).SetFloat64(curve.weights[i]))
		denominator.Add(denominator, weight)
		for axis := 0; axis < 3; axis++ {
			numerator[axis].Add(numerator[axis], new(big.Rat).Mul(weight, new(big.Rat).SetFloat64(coordinate(p, axis))))
		}
	}
	for axis := range numerator {
		numerator[axis].Quo(numerator[axis], denominator)
	}
	return numerator
}
func verifyRemovalBound(t *testing.T, source, candidate *BSpline, bound float64) {
	t.Helper()
	square := new(big.Rat).SetFloat64(bound)
	square.Mul(square, square)
	a, b := source.Domain()
	for i := 0; i <= 16; i++ {
		u := a + (b-a)*float64(i)/16
		left, right := exactCurveOracle(source, u), exactCurveOracle(candidate, u)
		distanceSquared := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			delta := new(big.Rat).Sub(left[axis], right[axis])
			distanceSquared.Add(distanceSquared, new(big.Rat).Mul(delta, delta))
		}
		if distanceSquared.Cmp(square) > 0 {
			t.Fatalf("exact geometric error exceeds bound at %g: bound %g", u, bound)
		}
	}
}
func TestRandomKnotInsertRemoveAndExactBound(t *testing.T) {
	random := rand.New(rand.NewSource(44549))
	for trial := 0; trial < 35; trial++ {
		degree := 1 + random.Intn(5)
		poles := make([]Vec3, degree+1)
		weights := make([]float64, len(poles))
		for i := range poles {
			poles[i] = Vec3{6*random.Float64() - 3, 6*random.Float64() - 3, 6*random.Float64() - 3}
			weights[i] = .25 + 3*random.Float64()
		}
		original, e := NewBezier(poles, weights)
		if e != nil {
			t.Fatal(e)
		}
		count := 1 + random.Intn(degree+1)
		inserted, e := original.InsertKnot(.37, count)
		if e != nil {
			t.Fatal(e)
		}
		removed, diagnostics, e := inserted.RemoveKnot(.37, count, KnotRemovalOptions{Tolerance: 1e-9})
		if e != nil || diagnostics.Removed != count || len(removed.Poles()) != len(poles) || diagnostics.GlobalErrorBound > 1e-9 {
			t.Fatal(trial, diagnostics, e)
		}
		verifyRemovalBound(t, inserted, removed, diagnostics.GlobalErrorBound)
		for i := 0; i <= 20; i++ {
			u := float64(i) / 20
			p, _ := original.Eval(u)
			q, _ := removed.Eval(u)
			near(t, q, p, 2e-12)
		}
	}
}
func TestRationalCircleKnotRemoval(t *testing.T) {
	circle, _ := NewCircle(Vec3{2, 3, 4}, Vec3{0, 0, 1}, 2)
	source, _ := circle.RationalBSpline()
	refined, e := source.InsertKnot(.41, 2)
	if e != nil {
		t.Fatal(e)
	}
	removed, diagnostics, e := refined.RemoveKnot(.41, 2, KnotRemovalOptions{})
	if e != nil || len(removed.Poles()) != len(source.Poles()) {
		t.Fatal(diagnostics, e)
	}
	verifyRemovalBound(t, refined, removed, diagnostics.GlobalErrorBound)
	if curve, diagnostics, e := source.RemoveKnot(1, 1, KnotRemovalOptions{Tolerance: 1e-8}); curve != nil || !errors.Is(e, ErrKnotRemoval) || diagnostics.Removed != 0 {
		t.Fatal(diagnostics, e)
	}
}
func TestKnotRemovalRejectsShapeAndBoundsAcceptedChange(t *testing.T) {
	curve, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 1, 0}, {2, 0, 0}}, []float64{0, 0, .5, 1, 1}, nil)
	candidate, diagnostics, e := curve.RemoveKnot(.5, 1, KnotRemovalOptions{Tolerance: 1e-3})
	if candidate != nil || !errors.Is(e, ErrKnotRemoval) || diagnostics.GlobalErrorBound <= 1e-3 || diagnostics.Removed != 0 {
		t.Fatal(diagnostics, e)
	}
	candidate, diagnostics, e = curve.RemoveKnot(.5, 1, KnotRemovalOptions{Tolerance: 1})
	if e != nil || diagnostics.GlobalErrorBound > 1 {
		t.Fatal(diagnostics, e)
	}
	verifyRemovalBound(t, curve, candidate, diagnostics.GlobalErrorBound)
	badWeights, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 1, 0}, {2, 0, 0}}, []float64{0, 0, .5, 1, 1}, []float64{.001, .001, 1})
	if candidate, _, e = badWeights.RemoveKnot(.5, 1, KnotRemovalOptions{Tolerance: 100}); candidate != nil || !errors.Is(e, ErrKnotRemoval) {
		t.Fatal(e)
	}
}
func TestKnotRemovalLimitsAndOwnership(t *testing.T) {
	original, _ := NewBezier([]Vec3{{0, 0, 0}, {1, 2, 0}, {2, 0, 0}}, []float64{1, .7, 1})
	curve, _ := original.InsertKnot(.37, 1)
	for _, options := range []KnotRemovalOptions{{MaxWork: 1}, {MaxPoles: 3}, {MaxExactOperations: 1}, {MaxCoefficientBits: 64}} {
		if _, _, e := curve.RemoveKnot(.37, 1, options); !errors.Is(e, ErrConvergence) {
			t.Fatal(options, e)
		}
	}
	if _, _, e := curve.RemoveKnot(.37, 1, KnotRemovalOptions{RConditionLimit: 1.01}); !errors.Is(e, ErrIllConditioned) {
		t.Fatal(e)
	}
	for _, parameter := range []float64{0, 1} {
		if _, _, e := curve.RemoveKnot(parameter, 1, KnotRemovalOptions{}); !errors.Is(e, ErrDomain) {
			t.Fatal(parameter, e)
		}
	}
	if _, _, e := curve.RemoveKnot(.5, 1, KnotRemovalOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	copy, diagnostics, e := curve.RemoveKnot(.37, 0, KnotRemovalOptions{})
	if e != nil || diagnostics.GlobalErrorBound != 0 {
		t.Fatal(diagnostics, e)
	}
	copy.poles[0] = Vec3{99, 99, 99}
	if copy.poles[0] == curve.poles[0] {
		t.Fatal("alias")
	}
}

func TestKnotRemovalDiscontinuousSource(t *testing.T) {
	source, e := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {1, .01, 0}, {2, .01, 0}}, []float64{0, 0, .5, .5, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	candidate, diagnostics, e := source.RemoveKnot(.5, 2, KnotRemovalOptions{Tolerance: .02})
	if e != nil || diagnostics.Removed != 2 || diagnostics.GlobalErrorBound > .02 {
		t.Fatal(diagnostics, e)
	}
	verifyRemovalBound(t, source, candidate, diagnostics.GlobalErrorBound)
}
