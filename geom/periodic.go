package geom

import "math"

// PeriodicBSpline is an immutable uniform periodic rational curve. Its supplied
// poles/weights describe one cyclic control net, not interpolation points.
// Internally an exactly uniform integer knot lattice and degree repeated poles
// define the fundamental domain. Physical parameter wrapping is float64; no
// function can recover phase already lost in a caller's rounded parameter.
// The curve is C^(degree-1) across its seam, subject to evaluation roundoff.
type PeriodicBSpline struct {
	curve              *BSpline
	poles              []Vec3
	weights            []float64
	degree             int
	start, period, end float64
}

// NewPeriodicBSpline constructs a uniform cyclic curve with positive weights.
// There must be more poles than degree, with degree in [1,64]. Nil weights mean
// a polynomial curve. start+period must be finite and distinguishable from start.
// A physical knot span must also be distinguishable at both domain endpoints.
// Existing BSpline constructors and their domain semantics are unchanged.
func NewPeriodicBSpline(degree int, poles []Vec3, weights []float64, start, period float64) (*PeriodicBSpline, error) {
	if degree < 1 || degree > 64 || len(poles) <= degree || uint64(len(poles)) > 1<<52 || !finite(start) || !finite(period) || period <= 0 {
		return nil, ErrInvalid
	}
	end := start + period
	if !finite(end) || end <= start {
		return nil, ErrInvalid
	}
	spacing := period / float64(len(poles))
	if spacing == 0 || start+spacing == start || end-spacing == end {
		return nil, ErrInvalid
	}
	for _, p := range poles {
		if !p.IsFinite() {
			return nil, ErrInvalid
		}
	}
	w, e := copyWeights(len(poles), weights)
	if e != nil {
		return nil, e
	}
	expandedPoles := make([]Vec3, len(poles)+degree)
	expandedWeights := make([]float64, len(expandedPoles))
	for i := range expandedPoles {
		expandedPoles[i], expandedWeights[i] = poles[i%len(poles)], w[i%len(poles)]
	}
	knots := make([]float64, len(poles)+2*degree+1)
	for i := range knots {
		knots[i] = float64(i - degree)
	}
	curve, e := NewBSpline(degree, expandedPoles, knots, expandedWeights)
	if e != nil {
		return nil, e
	}
	return &PeriodicBSpline{curve, append([]Vec3(nil), poles...), w, degree, start, period, end}, nil
}
func (c *PeriodicBSpline) Degree() int {
	if c == nil {
		return 0
	}
	return c.degree
}

// Domain is the fundamental physical interval. Nil/zero values return NaNs.
func (c *PeriodicBSpline) Domain() (float64, float64) {
	if c == nil || c.curve == nil {
		return math.NaN(), math.NaN()
	}
	return c.start, c.end
}
func (c *PeriodicBSpline) Period() float64 {
	if c == nil || c.curve == nil {
		return math.NaN()
	}
	return c.period
}

// Poles and Weights return independent copies of one cyclic control net.
func (c *PeriodicBSpline) Poles() []Vec3 {
	if c == nil {
		return nil
	}
	return append([]Vec3(nil), c.poles...)
}
func (c *PeriodicBSpline) Weights() []float64 {
	if c == nil {
		return nil
	}
	return append([]float64(nil), c.weights...)
}
func (c *PeriodicBSpline) wrappedParameter(t float64) (float64, error) {
	if c == nil || c.curve == nil || !finite(t) {
		return 0, ErrDomain
	}
	if t == c.start || t == c.end {
		return 0, nil
	}
	difference := t - c.start
	phase := 0.0
	if finite(difference) {
		phase = math.Mod(difference, c.period)
		if phase < 0 {
			phase += c.period
		}
	} else { // Separately reduce finite operands when subtraction would overflow.
		rt, rs := math.Mod(t, c.period), math.Mod(c.start, c.period)
		if rt < 0 {
			rt += c.period
		}
		if rs < 0 {
			rs += c.period
		}
		phase = rt - rs
		if phase < 0 {
			phase += c.period
		}
	}
	count := float64(len(c.poles))
	if phase >= c.period {
		return math.Nextafter(count, 0), nil
	}
	u := (phase / c.period) * count
	if u >= count {
		u = math.Nextafter(count, 0)
	}
	return u, nil
}
func (c *PeriodicBSpline) Eval(t float64) (Vec3, error) {
	u, e := c.wrappedParameter(t)
	if e != nil {
		return Vec3{}, e
	}
	return c.curve.Eval(u)
}

// Derivative differentiates physical parameters. At the seam degree-one curves
// use the right derivative; degrees >=2 have matching one-sided derivatives up
// to arithmetic error. Explicit Domain() endpoints both select the same seam.
func (c *PeriodicBSpline) Derivative(t float64) (Vec3, error) {
	u, e := c.wrappedParameter(t)
	if e != nil {
		return Vec3{}, e
	}
	derivative, e := c.curve.Derivative(u)
	if e != nil {
		return Vec3{}, e
	}
	numeratorMantissa, numeratorExponent := math.Frexp(float64(len(c.poles)))
	denominatorMantissa, denominatorExponent := math.Frexp(c.period)
	scale := numeratorMantissa / denominatorMantissa
	exponent := numeratorExponent - denominatorExponent
	values := [3]float64{}
	for axis := range values {
		mantissa, power := math.Frexp(coordinate(derivative, axis))
		values[axis] = math.Ldexp(mantissa*scale, power+exponent)
	}
	result := Vec3{values[0], values[1], values[2]}
	if !result.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return result, nil
}

// Reversed preserves the fundamental domain and reverses traversal:
// reversed.Eval(start+t) equals Eval(start+period-t), modulo roundoff.
func (c *PeriodicBSpline) Reversed() (*PeriodicBSpline, error) {
	if c == nil || c.curve == nil {
		return nil, ErrInvalid
	}
	poles := make([]Vec3, len(c.poles))
	weights := make([]float64, len(poles))
	for i := range poles {
		j := (c.degree - 1 - i) % len(poles)
		if j < 0 {
			j += len(poles)
		}
		poles[i], weights[i] = c.poles[j], c.weights[j]
	}
	return NewPeriodicBSpline(c.degree, poles, weights, c.start, c.period)
}

// Sample uniformly samples the fundamental interval, repeating the seam point.
// It does not certify a chord-error bound.
func (c *PeriodicBSpline) Sample(count int) ([]Vec3, error) {
	if c == nil || c.curve == nil || count < 2 || count > 10000000 {
		return nil, ErrInvalid
	}
	points := make([]Vec3, count)
	for i := range points {
		t := c.start + c.period*(float64(i)/float64(count-1))
		if i == count-1 {
			t = c.end
		}
		p, e := c.Eval(t)
		if e != nil {
			return nil, e
		}
		points[i] = p
	}
	return points, nil
}

// periodicBreakpoints enumerates represented knot breaks within a bounded
// physical range. It refuses ranges that exceed an integration/tessellation
// work budget or cannot resolve adjacent spans in float64 parameters.
func (c *PeriodicBSpline) periodicBreakpoints(start, end float64, maxIntervals int) ([]float64, error) {
	u, e := c.wrappedParameter(start)
	if e != nil {
		return nil, e
	}
	spanCount := ((end - start) / c.period) * float64(len(c.poles))
	if !finite(spanCount) || spanCount < 0 || math.Ceil(spanCount) > float64(maxIntervals) {
		return nil, ErrConvergence
	}
	count := int(math.Ceil(spanCount)) + 1
	breaks := make([]float64, 0, count)
	base := math.Floor(u)
	previous := start
	for i := 1; i <= count; i++ {
		offset := ((base + float64(i) - u) / float64(len(c.poles))) * c.period
		t := start + offset
		if t >= end {
			break
		}
		if t <= previous || len(breaks) >= maxIntervals-1 {
			return nil, ErrConvergence
		}
		breaks = append(breaks, t)
		previous = t
	}
	return breaks, nil
}
