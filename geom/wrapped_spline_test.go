package geom

import (
	"errors"
	"math"
	"testing"
)

func TestWrappedSplineRationalCircleAndNonuniformKnots(t *testing.T) {
	circle, _ := NewCircle(Vec3{}, Vec3{Z: 1}, 2)
	curve, _ := circle.RationalBSpline()
	curve, e := curve.InsertKnot(.13, 1)
	if e != nil {
		t.Fatal(e)
	}
	w, d, e := NewWrappedBSpline(curve, ClosedCurveOptions{DerivativeOrder: 1})
	if e != nil || d.PositionResidual > 1e-12 || d.DerivativeResidual > 1e-12 {
		t.Fatal(d, e)
	}
	start, end := curve.Domain()
	period := end - start
	for i := -20; i <= 20; i++ {
		u := start + .375*period
		actual, e := w.Eval(u + float64(i)*period)
		expected, _ := curve.Eval(u)
		if e != nil || actual.Distance(expected) > 1e-12 {
			t.Fatal(actual, expected, e)
		}
	}
	for _, u := range []float64{start, end, start - period, end + period} {
		p, _ := w.Eval(u)
		q, _ := curve.Eval(start)
		if p != q {
			t.Fatal(p, q)
		}
	}
	for _, u := range []float64{math.MaxFloat64, -math.MaxFloat64} {
		parameter, e := w.WrappedParameter(u)
		if e != nil || parameter < start || parameter >= end {
			t.Fatal(parameter, e)
		}
		p, e := w.Eval(u)
		if e != nil || math.Abs(p.Length()-2) > 1e-12 {
			t.Fatal(p, e)
		}
	}
	original := curve.Knots()
	copy := w.FundamentalCurve().Knots()
	for i := range copy {
		if original[i] != copy[i] {
			t.Fatal("knots changed")
		}
	}
}
func TestWrappedSplineDerivativeMismatchAndTolerance(t *testing.T) {
	curve, _ := NewBezier([]Vec3{{}, {X: 1}, {Y: 1}, {}}, nil)
	if w, d, e := NewWrappedBSpline(curve, ClosedCurveOptions{DerivativeOrder: 1}); !errors.Is(e, ErrDegenerate) || w != nil || d.DerivativeResidual == 0 {
		t.Fatal(w, d, e)
	}
	if _, _, e := NewWrappedBSpline(curve, ClosedCurveOptions{}); e != nil {
		t.Fatal(e)
	}
	curve, _ = NewBezier([]Vec3{{}, {X: 1}, {X: 1e-8}}, nil)
	if _, _, e := NewWrappedBSpline(curve, ClosedCurveOptions{}); !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	w, d, e := NewWrappedBSpline(curve, ClosedCurveOptions{PositionTolerance: 1e-7})
	if e != nil || d.PositionResidual != 1e-8 {
		t.Fatal(d, e)
	}
	p, _ := w.Eval(1)
	if p != (Vec3{}) {
		t.Fatal("seam must select start", p)
	}
}
func TestWrappedSplineExactExtremeReduction(t *testing.T) {
	// Domain length 3 has exact representable period; 2^1023 mod 3 = 2.
	curve, _ := NewBSpline(1, []Vec3{{}, {X: 1}, {}}, []float64{0, 0, 1, 3, 3}, nil)
	w, _, e := NewWrappedBSpline(curve, ClosedCurveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ input, want float64 }{{math.Ldexp(1, 1023), 2}, {-math.Ldexp(1, 1023), 1}, {-1, 2}, {3, 0}} {
		got, e := w.WrappedParameter(tc.input)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	if _, e := w.WrappedParameter(math.Inf(1)); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
