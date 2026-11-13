package geom

import (
	"errors"
	"math"
)

var ErrConvergence = errors.New("geom: numerical iteration or evaluation limit reached")

// LengthOptions controls adaptive arc-length quadrature. Zero fields select
// defaults (1e-9 absolute/relative tolerance, depth 24, 100000 evaluations).
// Tolerances describe an estimated error, not a certified interval bound.
type LengthOptions struct {
	AbsoluteTolerance, RelativeTolerance float64
	MaxDepth, MaxEvaluations             int
}

// LengthResult contains a numerical length and quadrature error estimate.
// Length and EstimatedError are valid only when ArcLength returns nil error.
type LengthResult struct {
	Length, EstimatedError float64
	Evaluations            int
}

// ArcLength integrates the speed on [start,end] using adaptive paired 3/5
// point Gaussian quadrature. Ordinary and periodic spline knot spans are
// integrated separately; jumps
// at discontinuous knots do not contribute a connecting line segment. Sampling
// can miss narrow features of arbitrary Curve implementations; the error is an
// estimate. Exhausted work limits return ErrConvergence rather than success.
func ArcLength(curve Curve, start, end float64, options LengthOptions) (LengthResult, error) {
	result := LengthResult{}
	if curve == nil || !finite(start) || !finite(end) || end < start || !finite(end-start) {
		return result, ErrDomain
	}
	o := options
	if o.AbsoluteTolerance == 0 {
		o.AbsoluteTolerance = 1e-9
	}
	if o.RelativeTolerance == 0 {
		o.RelativeTolerance = 1e-9
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxEvaluations == 0 {
		o.MaxEvaluations = 100000
	}
	if !finite(o.AbsoluteTolerance) || !finite(o.RelativeTolerance) || o.AbsoluteTolerance < 0 || o.RelativeTolerance < 0 || o.MaxDepth < 1 || o.MaxDepth > 64 || o.MaxEvaluations < 7 {
		return result, ErrInvalid
	}
	breaks := []float64{start}
	if c, ok := curve.(*BSpline); ok {
		if !c.validParameter(start) || !c.validParameter(end) {
			return result, ErrDomain
		}
		for _, k := range c.knots {
			if k > start && k < end && k != breaks[len(breaks)-1] {
				breaks = append(breaks, k)
			}
		}
	}
	if start == end {
		_, e := curve.Derivative(start)
		return result, e
	}
	if periodic, ok := curve.(*PeriodicBSpline); ok {
		knots, e := periodic.periodicBreakpoints(start, end, o.MaxEvaluations/7)
		if e != nil {
			return result, e
		}
		breaks = append(breaks, knots...)
	}
	breaks = append(breaks, end)
	speed := func(t float64) (float64, error) {
		if result.Evaluations >= o.MaxEvaluations {
			return 0, ErrConvergence
		}
		result.Evaluations++
		v, e := curve.Derivative(t)
		if e != nil {
			return 0, e
		}
		length := v.Length()
		if !finite(length) {
			return 0, ErrInvalid
		}
		return length, nil
	}
	var integrate func(float64, float64, float64, int) (float64, float64, error)
	integrate = func(a, b, absTol float64, depth int) (float64, float64, error) {
		mid := a + (b-a)/2
		half := (b - a) / 2
		if mid == a || mid == b {
			return 0, 0, ErrConvergence
		}
		center, e := speed(mid)
		if e != nil {
			return 0, 0, e
		}
		five := .5688888888888889 * center
		three := .8888888888888888 * center
		for i, x := range []float64{.5384693101056831, .9061798459386640, .7745966692414834} {
			left, e := speed(mid - half*x)
			if e != nil {
				return 0, 0, e
			}
			right, e := speed(mid + half*x)
			if e != nil {
				return 0, 0, e
			}
			sum := left + right
			switch i {
			case 0:
				five += .4786286704993665 * sum
			case 1:
				five += .2369268850561891 * sum
			case 2:
				three += .5555555555555556 * sum
			}
		}
		five *= half
		three *= half
		estimate := math.Abs(five - three)
		if !finite(five) || !finite(estimate) {
			return 0, 0, ErrInvalid
		}
		if estimate <= absTol+o.RelativeTolerance*five {
			return five, estimate, nil
		}
		if depth >= o.MaxDepth {
			return 0, 0, ErrConvergence
		}
		l, le, e := integrate(a, mid, absTol/2, depth+1)
		if e != nil {
			return 0, 0, e
		}
		r, re, e := integrate(mid, b, absTol/2, depth+1)
		return l + r, le + re, e
	}
	for i := 1; i < len(breaks); i++ {
		a, b := breaks[i-1], breaks[i]
		length, estimate, e := integrate(a, b, o.AbsoluteTolerance*((b-a)/(end-start)), 0)
		if e != nil {
			return LengthResult{Evaluations: result.Evaluations}, e
		}
		result.Length += length
		result.EstimatedError += estimate
	}
	if !finite(result.Length) || !finite(result.EstimatedError) {
		return LengthResult{Evaluations: result.Evaluations}, ErrInvalid
	}
	return result, nil
}
