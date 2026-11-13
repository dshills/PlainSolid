package geom

import (
	"context"
	"math/big"
)

// ExactCurvePlaneContact classifies a unique algebraic plane contact on one
// rational Bezier span. Order is exact residual vanishing order; Singular means
// all three curve velocity components vanish. Tangent requires regular velocity
// and order >1; Transverse requires order 1. Crossing/Touching describe the
// local two-sided plane sign only for interior contacts. Boundary contacts have
// neither flag, since their polynomial continuation is outside the given curve.
type ExactCurvePlaneContact struct {
	Proven, Transverse, Tangent, Singular, Crossing, Touching, Boundary bool
	Order                                                               int
	Lower, Upper                                                        *big.Rat
	Operations, Subdivisions                                            int
}

// ClassifyExactCurvePlaneContact uses exact polynomial root isolation and the
// GCD of all rational velocity numerators. No approximate derivative or sampled
// near-zero predicate participates. Coincident spans and nonunique intervals
// remain unresolved; repeated algebraic roots are fully supported.
func ClassifyExactCurvePlaneContact(ctx context.Context, c *ExactSplineCurve, plane [4]*big.Rat, lower, upper *big.Rat, o ExactCurvePlaneEventOptions) (r ExactCurvePlaneContact, err error) {
	if ctx == nil || c == nil || c.patch == nil || lower == nil || upper == nil || lower.Cmp(upper) > 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	p := c.patch
	if p.countU != p.degreeU+1 {
		return r, ErrConvergence
	}
	d := p.Domain()
	origin := new(big.Rat).SetFloat64(d.UMin)
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), origin)
	lo, hi := new(big.Rat).Quo(new(big.Rat).Sub(lower, origin), width), new(big.Rat).Quo(new(big.Rat).Sub(upper, origin), width)
	if lo.Sign() < 0 || hi.Cmp(big.NewRat(1, 1)) > 0 {
		return r, ErrDomain
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = budget.operations }()
	for _, x := range []*big.Rat{lo, hi} {
		if e := budget.check(x); e != nil {
			return r, e
		}
	}
	event, e := exactEventPlanePolynomial(p, plane, budget)
	if e != nil {
		return r, e
	}
	if len(event) == 0 {
		return r, ErrConvergence
	}
	count := 0
	var rootLo, rootHi *big.Rat
	steps, e := isolatePolynomialRoots(event, lo, hi, budget, o.MaxSubdivisions, func(a, b *big.Rat, m int) (bool, error) {
		count++
		rootLo, rootHi = new(big.Rat).Set(a), new(big.Rat).Set(b)
		r.Order = m
		return true, ctx.Err()
	})
	r.Subdivisions += steps
	if e != nil {
		return r, e
	}
	if count != 1 {
		return r, ErrConvergence
	}
	fields, e := sharedProjectionPowers(p, budget)
	if e != nil {
		return r, e
	}
	wd := polynomialDerivative(fields[3])
	var velocityGCD rationalPolynomial
	for axis := 0; axis < 3; axis++ {
		derivative := polynomialDerivative(fields[axis])
		left, e := polynomialProduct(derivative, fields[3], budget)
		if e != nil {
			return r, e
		}
		right, e := polynomialProduct(fields[axis], wd, budget)
		if e != nil {
			return r, e
		}
		size := max(len(left), len(right))
		numerator := make(rationalPolynomial, size)
		for i := range numerator {
			numerator[i] = new(big.Rat)
			if i < len(left) {
				numerator[i].Set(left[i])
			}
			if i < len(right) {
				numerator[i].Sub(numerator[i], right[i])
			}
			if e := budget.check(numerator[i]); e != nil {
				return r, e
			}
		}
		velocityGCD, e = polynomialGCD(velocityGCD, trimPolynomial(numerator), budget)
		if e != nil {
			return r, e
		}
	}
	common, e := polynomialGCD(event, velocityGCD, budget)
	if e != nil {
		return r, e
	}
	remaining := o.MaxSubdivisions - r.Subdivisions
	if remaining < 1 {
		return r, ErrConvergence
	}
	count = 0
	steps, e = isolatePolynomialRoots(common, rootLo, rootHi, budget, remaining, func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, ctx.Err() })
	r.Subdivisions += steps
	if e != nil {
		return r, e
	}
	r.Singular = count == 1
	r.Transverse = !r.Singular && r.Order == 1
	r.Tangent = !r.Singular && r.Order > 1
	r.Boundary = rootLo.Cmp(rootHi) == 0 && (rootLo.Sign() == 0 || rootLo.Cmp(big.NewRat(1, 1)) == 0)
	r.Crossing = !r.Boundary && r.Order%2 == 1
	r.Touching = !r.Boundary && r.Order%2 == 0
	r.Lower = new(big.Rat).Add(origin, new(big.Rat).Mul(width, rootLo))
	r.Upper = new(big.Rat).Add(origin, new(big.Rat).Mul(width, rootHi))
	r.Proven = true
	return r, nil
}
