package geom

import (
	"errors"
	"math"
)

var ErrNotBracketed = errors.New("geom: function values do not bracket a root")

// RootOptions bounds safeguarded scalar root solving. Zero fields select
// absolute/relative parameter tolerances 1e-12, residual tolerance 1e-12 and
// 128 iterations. A success requires both interval and residual criteria,
// except an exactly zero function value, which is accepted immediately.
type RootOptions struct {
	AbsoluteTolerance, RelativeTolerance, ResidualTolerance float64
	MaxIterations                                           int
}

// RootResult retains a sign bracket and the sampled point with least absolute
// residual among the retained bracket endpoints. On error this is diagnostic
// partial progress, not a root claim.
type RootResult struct {
	Parameter, Value, Lower, Upper float64
	Iterations, Evaluations        int
}

// SolveBracketed finds one root of a caller-supplied continuous scalar function.
// It combines a secant proposal with middle-half safeguarding, so no accepted
// iteration abandons the bracket. Continuity is a caller precondition; jumps
// are not mistaken for roots merely because the parameter interval is small.
// Nonfinite evaluations, callback errors, invalid brackets and exhausted work
// are explicit failures. The function receives only points inside [lower,upper].
func SolveBracketed(function func(float64) (float64, error), lower, upper float64, options RootOptions) (RootResult, error) {
	result := RootResult{Lower: lower, Upper: upper}
	o := options
	if o.AbsoluteTolerance == 0 {
		o.AbsoluteTolerance = 1e-12
	}
	if o.RelativeTolerance == 0 {
		o.RelativeTolerance = 1e-12
	}
	if o.ResidualTolerance == 0 {
		o.ResidualTolerance = 1e-12
	}
	if o.MaxIterations == 0 {
		o.MaxIterations = 128
	}
	if function == nil || !finite(lower) || !finite(upper) || lower >= upper || !finite(upper-lower) || !finite(o.AbsoluteTolerance) || !finite(o.RelativeTolerance) || !finite(o.ResidualTolerance) || o.AbsoluteTolerance < 0 || o.RelativeTolerance < 0 || o.ResidualTolerance < 0 || o.MaxIterations < 1 || o.MaxIterations > 1000000 {
		return result, ErrInvalid
	}
	evaluate := func(t float64) (float64, error) {
		result.Evaluations++
		v, e := function(t)
		if e != nil {
			return 0, e
		}
		if !finite(v) {
			return 0, ErrInvalid
		}
		return v, nil
	}
	left, e := evaluate(lower)
	if e != nil {
		return result, e
	}
	result.Parameter, result.Value = lower, left
	if left == 0 {
		result.Lower, result.Upper = lower, lower
		return result, nil
	}
	right, e := evaluate(upper)
	if e != nil {
		return result, e
	}
	if math.Abs(right) < math.Abs(result.Value) {
		result.Parameter, result.Value = upper, right
	}
	if right == 0 {
		result.Parameter, result.Value, result.Lower, result.Upper = upper, 0, upper, upper
		return result, nil
	}
	if math.Signbit(left) == math.Signbit(right) {
		return result, ErrNotBracketed
	}
	for iteration := 0; iteration < o.MaxIterations; iteration++ {
		width := upper - lower
		threshold := o.AbsoluteTolerance + o.RelativeTolerance*math.Abs(result.Parameter)
		if finite(threshold) && width <= threshold && math.Abs(result.Value) <= o.ResidualTolerance {
			return result, nil
		}
		maximum := math.Max(math.Abs(left), math.Abs(right))
		scaledLeft, scaledRight := math.Abs(left)/maximum, math.Abs(right)/maximum
		fraction := scaledLeft / (scaledLeft + scaledRight)
		candidate := lower + width*fraction
		if candidate < lower+width*.25 || candidate > upper-width*.25 {
			candidate = lower + width*.5
		}
		if candidate == lower || candidate == upper {
			return result, ErrConvergence
		}
		value, e := evaluate(candidate)
		if e != nil {
			return result, e
		}
		result.Iterations = iteration + 1
		if math.Abs(value) < math.Abs(result.Value) {
			result.Parameter, result.Value = candidate, value
		}
		if value == 0 {
			result.Parameter, result.Value, result.Lower, result.Upper = candidate, 0, candidate, candidate
			return result, nil
		}
		if math.Signbit(value) == math.Signbit(left) {
			lower, left = candidate, value
		} else {
			upper, right = candidate, value
		}
		result.Lower, result.Upper = lower, upper
		// Keep the reported candidate inside the retained bracket. A discarded
		// endpoint can have a small residual far from this bracket's root.
		result.Parameter, result.Value = lower, left
		if math.Abs(right) < math.Abs(left) {
			result.Parameter, result.Value = upper, right
		}
	}
	width := upper - lower
	threshold := o.AbsoluteTolerance + o.RelativeTolerance*math.Abs(result.Parameter)
	if finite(threshold) && width <= threshold && math.Abs(result.Value) <= o.ResidualTolerance {
		return result, nil
	}
	return result, ErrConvergence
}
