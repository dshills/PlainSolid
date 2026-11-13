package geom

import (
	"context"
	"math/big"
)

// ExactCurvePlaneEventOptions bounds exact arithmetic and root isolation.
type ExactCurvePlaneEventOptions struct{ MaxOperations, MaxCoefficientBits, MaxSubdivisions int }

// ExactCurvePlaneEvent proves the side of TestPlane at the unique EventPlane
// contact in the supplied closed interval. Side zero proves exact incidence,
// including irrational curve parameters. Lower/Upper are caller-owned brackets.
type ExactCurvePlaneEvent struct {
	Proven                   bool
	Side                     int
	Lower, Upper             *big.Rat
	Operations, Subdivisions int
}

// ClassifyExactCurvePlaneEvent compares two exact raw plane equations along a
// single rational Bezier span. Plane coefficients are Nx,Ny,Nz,D; nil means zero.
// The event interval must contain exactly one distinct event root. Coincident
// event spans, multiple roots in the interval, and multi-span curves remain
// unresolved. A repeated isolated root is supported. No numerical point or
// rounded parameter is accepted as evidence of incidence.
func ClassifyExactCurvePlaneEvent(ctx context.Context, c *ExactSplineCurve, eventPlane, testPlane [4]*big.Rat, lower, upper *big.Rat, o ExactCurvePlaneEventOptions) (r ExactCurvePlaneEvent, err error) {
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
	d := p.Domain()
	loDomain, hiDomain := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)
	if lower.Cmp(loDomain) < 0 || upper.Cmp(hiDomain) > 0 {
		return r, ErrDomain
	}
	if p.countU != p.degreeU+1 {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = budget.operations }()
	width := new(big.Rat).Sub(hiDomain, loDomain)
	lo := new(big.Rat).Quo(new(big.Rat).Sub(lower, loDomain), width)
	hi := new(big.Rat).Quo(new(big.Rat).Sub(upper, loDomain), width)
	for _, x := range []*big.Rat{lower, upper, lo, hi, width} {
		if e := budget.check(x); e != nil {
			return r, e
		}
	}
	planes := [2][4]*big.Rat{eventPlane, testPlane}
	var polynomials [2]rationalPolynomial
	for j, plane := range planes {
		q, e := exactEventPlanePolynomial(p, plane, budget)
		if e != nil {
			return r, e
		}
		polynomials[j] = q
	}

	if len(polynomials[0]) == 0 {
		return r, ErrConvergence
	}
	reserved := 0
	count := func(q rationalPolynomial, a, b *big.Rat) (int, error) {
		if len(q) == 0 {
			return -1, nil
		}
		n := 0
		remaining := o.MaxSubdivisions - r.Subdivisions - reserved
		if remaining < 1 {
			return 0, ErrConvergence
		}
		steps, e := isolatePolynomialRoots(q, a, b, budget, remaining, func(_, _ *big.Rat, _ int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			n++
			return true, nil
		})
		r.Subdivisions += steps
		return n, e
	}
	n, e := count(polynomials[0], lo, hi)
	if e != nil {
		return r, e
	}
	if n != 1 {
		return r, ErrConvergence
	}
	gcd, e := polynomialGCD(polynomials[0], polynomials[1], budget)
	if e != nil {
		return r, e
	}
	n, e = count(gcd, lo, hi)
	if e != nil {
		return r, e
	}
	setBounds := func(a, b *big.Rat) {
		r.Lower = new(big.Rat).Add(loDomain, new(big.Rat).Mul(width, a))
		r.Upper = new(big.Rat).Add(loDomain, new(big.Rat).Mul(width, b))
	}
	if n == 1 {
		r.Proven = true
		setBounds(lo, hi)
		return r, nil
	}
	remaining := o.MaxSubdivisions - r.Subdivisions
	if remaining < 1 {
		return r, ErrConvergence
	}
	if remaining < 2 {
		return r, ErrConvergence
	}
	reserved = remaining / 2
	steps, e := isolatePolynomialRoots(polynomials[0], lo, hi, budget, reserved, func(a, b *big.Rat, _ int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		// A closed interval free of test-plane roots has constant strict sign.
		n, e := count(polynomials[1], a, b)
		if e != nil {
			return false, e
		}
		if n != 0 {
			return false, nil
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
		v, e := polynomialValue(polynomials[1], mid, budget)
		if e != nil {
			return false, e
		}
		if v.Sign() == 0 {
			return false, ErrConvergence
		}
		r.Side = v.Sign()
		r.Proven = true
		setBounds(a, b)
		return true, nil
	})
	r.Subdivisions += steps
	if e != nil {
		return r, e
	}
	return r, nil
}
