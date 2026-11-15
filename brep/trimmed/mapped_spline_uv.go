package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// MappedSplineUV retains an original cubic/quartic SplineUV and an exact positive
// Möbius map phi(t)=weight*t/(1-t+weight*t). Its normalized t is still the native
// model-edge parameter; only the original author's source speed changes. Source
// range/reversal are applied after phi. This is not arbitrary reparameterization.
// Immutable construction proves the entire source trace before multiplying each
// homogeneous Bernstein control i by weight^i. No sampled fitting is performed.
type MappedSplineUV struct {
	source SplineUV
	weight *big.Rat
	curve  ExactBezierUV
}

// NewMappedSplineUV admits degree3..4 and at most eight proved redundant spans,
// 64 original poles/knots. Existing descriptor stages retain o's proof budgets;
// map construction adds at most20 rational products, charged against o once,
// with at most4096-bit input/output coefficients and the supplied smaller bit
// limit. A positive map fixes both endpoints and is strictly increasing.
func NewMappedSplineUV(ctx context.Context, source SplineUV, weight *big.Rat, o geom.SurfacePatchOptions) (MappedSplineUV, error) {
	var zero MappedSplineUV
	if ctx == nil || source.Curve == nil || weight == nil || weight.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if weight.Num().BitLen() > 4096 || weight.Denom().BitLen() > 4096 || len(source.Curve.Poles()) > 64 || len(source.Curve.Knots()) > 64 {
		return zero, geom.ErrConvergence
	}
	mapped, e := mixedStoredHigherSplineUV(ctx, source, o)
	if e != nil {
		return zero, e
	}
	h := mapped.(ExactBezierUV).HomogeneousControls()
	maxBits, maxOps := o.MaxCoefficientBits, o.MaxOperations
	if maxBits == 0 || maxBits > 4096 {
		maxBits = 4096
	}
	if maxOps == 0 {
		maxOps = 1000000
	}
	if maxBits < 1 || maxOps < 1 {
		return zero, geom.ErrInvalid
	}
	pow := big.NewRat(1, 1)
	work := 0
	for i := range h {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if i > 0 {
			work++
			if work > maxOps {
				return zero, geom.ErrConvergence
			}
			pow.Mul(pow, weight)
		}
		if pow.Num().BitLen() > maxBits || pow.Denom().BitLen() > maxBits {
			return zero, geom.ErrConvergence
		}
		for k := range h[i] {
			work++
			if work > maxOps {
				return zero, geom.ErrConvergence
			}
			h[i][k].Mul(h[i][k], pow)
			if h[i][k].Num().BitLen() > maxBits || h[i][k].Denom().BitLen() > maxBits {
				return zero, geom.ErrConvergence
			}
		}
	}
	q, e := NewExactBezierUV(h)
	if e != nil {
		return zero, e
	}
	return MappedSplineUV{source: source, weight: new(big.Rat).Set(weight), curve: q}, nil
}

// Source returns the unchanged original authored spline, interval and reversal.
func (q MappedSplineUV) Source() SplineUV { return q.source }

// ParameterWeight returns an independent copy of the positive map coefficient.
func (q MappedSplineUV) ParameterWeight() *big.Rat {
	if q.weight == nil {
		return nil
	}
	return new(big.Rat).Set(q.weight)
}

// AuthoredParameter returns the exact original Curve parameter, not model t.
// Input numerator/denominator are bounded to4096 bits; output to16384 bits.
func (q MappedSplineUV) AuthoredParameter(t *big.Rat) (*big.Rat, error) {
	if q.weight == nil || t == nil || t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, geom.ErrDomain
	}
	if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return nil, geom.ErrConvergence
	}
	x := new(big.Rat).Mul(q.weight, t)
	den := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(1, 1), t), x)
	x.Quo(x, den)
	if q.source.Reversed {
		x.Sub(big.NewRat(1, 1), x)
	}
	x.Mul(x, new(big.Rat).Sub(ratInset(q.source.Range.Max), ratInset(q.source.Range.Min)))
	x.Add(x, ratInset(q.source.Range.Min))
	if x.Num().BitLen() > 16384 || x.Denom().BitLen() > 16384 {
		return nil, geom.ErrConvergence
	}
	return x, nil
}
func (q MappedSplineUV) EvalUV(t float64) (geom.Vec2, error)       { return q.curve.EvalUV(t) }
func (q MappedSplineUV) DerivativeUV(t float64) (geom.Vec2, error) { return q.curve.DerivativeUV(t) }
func (q MappedSplineUV) BoundsUV() (geom.UVBox, error)             { return q.curve.BoundsUV() }

func hasNativeMappedSpline(s Solid) bool {
	var has func(PCurve) bool
	has = func(c PCurve) bool {
		switch q := c.(type) {
		case MappedSplineUV:
			return true
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
