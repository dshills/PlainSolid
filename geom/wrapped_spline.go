package geom

import (
	"math"
	"math/big"
)

// ClosedCurveOptions validates a clamped curve seam through derivative order
// 0 (position only) or 1. Zero tolerances select 1e-9 in position and derivative
// units respectively. Validation is numerical; accepted nonzero seam residuals
// do not make the underlying curve mathematically smooth or exactly closed.
type ClosedCurveOptions struct {
	DerivativeOrder                        int
	PositionTolerance, DerivativeTolerance float64
}

// ClosedCurveDiagnostics records endpoint residuals before wrapping.
type ClosedCurveDiagnostics struct {
	DerivativeOrder                      int
	PositionResidual, DerivativeResidual float64
}

// WrappedBSpline periodically evaluates an immutable copy of an arbitrary
// nonuniform clamped rational curve. It preserves knots, poles and weights;
// no periodic control-net reconstruction or reparameterization is performed.
// Endpoint matching is tolerance-qualified, unlike the cyclic construction in
// PeriodicBSpline. Interior knot discontinuities remain unchanged.
type WrappedBSpline struct {
	curve      *BSpline
	start, end float64
	period     *big.Rat
	seam       ClosedCurveDiagnostics
}

// NewWrappedBSpline requires a finite nonzero domain, clamping, and endpoint
// agreement through the requested derivative order. Orders above one are
// unsupported and return ErrInvalid. Seam failure returns ErrDegenerate.
func NewWrappedBSpline(curve *BSpline, options ClosedCurveOptions) (*WrappedBSpline, ClosedCurveDiagnostics, error) {
	d := ClosedCurveDiagnostics{DerivativeOrder: options.DerivativeOrder}
	o := options
	if o.PositionTolerance == 0 {
		o.PositionTolerance = 1e-9
	}
	if o.DerivativeTolerance == 0 {
		o.DerivativeTolerance = 1e-9
	}
	if curve == nil || len(curve.poles) == 0 || o.DerivativeOrder < 0 || o.DerivativeOrder > 1 || !finite(o.PositionTolerance) || o.PositionTolerance <= 0 || !finite(o.DerivativeTolerance) || o.DerivativeTolerance <= 0 {
		return nil, d, ErrInvalid
	}
	start, end := curve.Domain()
	if !finite(end-start) || end <= start {
		return nil, d, ErrInvalid
	}
	for i := 0; i <= curve.degree; i++ {
		if curve.knots[i] != start || curve.knots[len(curve.knots)-1-i] != end {
			return nil, d, ErrInvalid
		}
	}
	a, e := curve.Eval(start)
	if e != nil {
		return nil, d, e
	}
	b, e := curve.Eval(end)
	if e != nil {
		return nil, d, e
	}
	d.PositionResidual = a.Distance(b)
	if !finite(d.PositionResidual) || d.PositionResidual > o.PositionTolerance {
		return nil, d, ErrDegenerate
	}
	if o.DerivativeOrder == 1 {
		a, e = curve.Derivative(start)
		if e != nil {
			return nil, d, e
		}
		b, e = curve.Derivative(end)
		if e != nil {
			return nil, d, e
		}
		d.DerivativeResidual = a.Distance(b)
		if !finite(d.DerivativeResidual) || d.DerivativeResidual > o.DerivativeTolerance {
			return nil, d, ErrDegenerate
		}
	}
	// Direct copying preserves the already-normalized stored weights bit for bit.
	copy := &BSpline{degree: curve.degree, poles: append([]Vec3(nil), curve.poles...), weights: append([]float64(nil), curve.weights...), knots: append([]float64(nil), curve.knots...)}
	period := new(big.Rat).Sub(new(big.Rat).SetFloat64(end), new(big.Rat).SetFloat64(start))
	return &WrappedBSpline{copy, start, end, period, d}, d, nil
}
func (c *WrappedBSpline) Domain() (float64, float64) {
	if c == nil || c.curve == nil {
		return math.NaN(), math.NaN()
	}
	return c.start, c.end
}

// Period is the nearest float64 representation of the exact endpoint difference.
func (c *WrappedBSpline) Period() float64 {
	if c == nil || c.period == nil {
		return math.NaN()
	}
	p, _ := c.period.Float64()
	return p
}
func (c *WrappedBSpline) SeamDiagnostics() ClosedCurveDiagnostics {
	if c == nil {
		return ClosedCurveDiagnostics{}
	}
	return c.seam
}

// FundamentalCurve returns the immutable unwrapped curve with its original
// endpoint semantics. Curve accessors return copies of coefficient arrays.
func (c *WrappedBSpline) FundamentalCurve() *BSpline {
	if c == nil {
		return nil
	}
	return c.curve
}

// WrappedParameter reduces the exact rational value of the float64 input modulo
// the exact endpoint difference, even when t-start would overflow. Only the
// resulting parameter is rounded to float64. Values rounding to the upper
// endpoint are clamped one float inward; exact seams select the start/right side.
// Phase lost before the caller supplies t cannot be recovered.
func (c *WrappedBSpline) WrappedParameter(t float64) (float64, error) {
	if c == nil || c.curve == nil || !finite(t) {
		return 0, ErrDomain
	}
	if t >= c.start && t < c.end {
		return t, nil
	}
	delta := new(big.Rat).Sub(new(big.Rat).SetFloat64(t), new(big.Rat).SetFloat64(c.start))
	ratio := new(big.Rat).Quo(delta, c.period)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(ratio.Num(), ratio.Denom(), remainder)
	if remainder.Sign() < 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}
	phase := new(big.Rat).Sub(delta, new(big.Rat).Mul(new(big.Rat).SetInt(quotient), c.period))
	wrapped := new(big.Rat).Add(new(big.Rat).SetFloat64(c.start), phase)
	value, _ := wrapped.Float64()
	if value >= c.end {
		value = math.Nextafter(c.end, c.start)
	}
	if value < c.start {
		value = c.start
	}
	return value, nil
}
func (c *WrappedBSpline) Eval(t float64) (Vec3, error) {
	u, e := c.WrappedParameter(t)
	if e != nil {
		return Vec3{}, e
	}
	return c.curve.Eval(u)
}

// Derivative uses the selected wrapped polynomial piece. Position-only seam
// validation does not imply equal left/right seam derivatives.
func (c *WrappedBSpline) Derivative(t float64) (Vec3, error) {
	u, e := c.WrappedParameter(t)
	if e != nil {
		return Vec3{}, e
	}
	return c.curve.Derivative(u)
}
