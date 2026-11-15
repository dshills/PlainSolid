package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// WithExactEndpoints enables exact evaluation of this original active interval's
// endpoints. The source spline, knots, range and reversal are retained. Only
// positive-weight XY curves of degree 1 through 4 with at most 64 poles and
// knots are supported. Exact rational de Boor uses at most 160 arithmetic
// steps with 16,384-bit checked
// intermediates; both endpoints must be exactly representable. Interior EvalUV
// and all native/model agreement requirements are unchanged. No cached endpoint
// or caller assertion substitutes for the persisted original source.
func (c SplineUV) WithExactEndpoints() (SplineUV, error) {
	q := c
	q.ExactEndpoints = true
	if _, e := q.EvalUV(0); e != nil {
		return SplineUV{}, e
	}
	if _, e := q.EvalUV(1); e != nil {
		return SplineUV{}, e
	}
	return q, nil
}

func exactSplineUVEndpoint(c SplineUV, parameter float64) (geom.Vec2, error) {
	var zero geom.Vec2
	if c.Curve == nil || !finite(parameter) {
		return zero, geom.ErrDomain
	}
	p := c.Curve.Degree()
	if p < 1 || p > 4 || c.Curve.PoleCount() > 63-p {
		return zero, geom.ErrConvergence
	}
	poles, knots, weights := c.Curve.Poles(), c.Curve.Knots(), c.Curve.Weights()
	if len(poles) < p+1 || len(knots) != len(poles)+p+1 || len(weights) != 0 && len(weights) != len(poles) {
		return zero, geom.ErrInvalid
	}
	lo, hi := c.Curve.Domain()
	if c.Range.Min < lo || c.Range.Max > hi || parameter < lo || parameter > hi {
		return zero, geom.ErrDomain
	}
	for i, v := range poles {
		if !finite(v.X) || !finite(v.Y) || v.Z != 0 {
			return zero, geom.ErrDomain
		}
		if len(weights) > 0 && (!finite(weights[i]) || weights[i] <= 0) {
			return zero, geom.ErrDomain
		}
	}
	span := len(poles) - 1
	for i := p; i < len(poles); i++ {
		if parameter < knots[i+1] {
			span = i
			break
		}
	}
	check := func(v *big.Rat) bool { return v.Num().BitLen() <= 16384 && v.Denom().BitLen() <= 16384 }
	d := make([][3]*big.Rat, p+1)
	for j := 0; j <= p; j++ {
		i := span - p + j
		w := big.NewRat(1, 1)
		if len(weights) > 0 {
			w = ratInset(weights[i])
		}
		d[j] = [3]*big.Rat{new(big.Rat).Mul(ratInset(poles[i].X), w), new(big.Rat).Mul(ratInset(poles[i].Y), w), w}
		for _, v := range d[j] {
			if !check(v) {
				return zero, geom.ErrConvergence
			}
		}
	}
	t := ratInset(parameter)
	for r := 1; r <= p; r++ {
		for j := p; j >= r; j-- {
			lower, upper := ratInset(knots[j+span-p]), ratInset(knots[j+span-r+1])
			den := new(big.Rat).Sub(upper, lower)
			num := new(big.Rat).Sub(t, lower)
			if !check(den) || !check(num) || den.Sign() <= 0 || num.Sign() < 0 || num.Cmp(den) > 0 {
				return zero, geom.ErrConvergence
			}
			a := new(big.Rat).Quo(num, den)
			b := new(big.Rat).Sub(big.NewRat(1, 1), a)
			if !check(a) || !check(b) {
				return zero, geom.ErrConvergence
			}
			for k := 0; k < 3; k++ {
				x := new(big.Rat).Mul(b, d[j-1][k])
				y := new(big.Rat).Mul(a, d[j][k])
				v := new(big.Rat).Add(x, y)
				if !check(x) || !check(y) || !check(v) {
					return zero, geom.ErrConvergence
				}
				d[j][k] = v
			}
		}
	}
	if d[p][2].Sign() <= 0 {
		return zero, geom.ErrConvergence
	}
	var out [2]float64
	for k := 0; k < 2; k++ {
		v := new(big.Rat).Quo(d[p][k], d[p][2])
		if !check(v) {
			return zero, geom.ErrConvergence
		}
		out[k], _ = v.Float64()
		if !finite(out[k]) || ratInset(out[k]).Cmp(v) != 0 {
			return zero, geom.ErrUnrepresentable
		}
	}
	return geom.Vec2{X: out[0], Y: out[1]}, nil
}

func hasNativeExactSplineEndpoints(s Solid) bool {
	var has func(PCurve) bool
	has = func(c PCurve) bool {
		switch q := c.(type) {
		case SplineUV:
			return q.ExactEndpoints
		case subTrim:
			return has(q.base)
		case reverseTrim:
			return has(q.differentiableTrim)
		}
		return false
	}
	for _, e := range s.edges {
		if m, ok := e.Curve.(surfaceTrimCurve); ok && has(m.curve) {
			return true
		}
	}
	for _, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if has(u.Curve) {
					return true
				}
			}
		}
	}
	return false
}
