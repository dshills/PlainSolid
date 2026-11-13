package geom

import (
	"context"
	"math/big"
)

// MapMobiusRoot proves membership in domain and maps the exact root by
// (m0+m1*t)/(m2+m3*t). It requires ordered domain endpoints, a nonzero
// determinant, and one strict denominator sign on the entire closed domain.
// CoordinateRoot's defining degree<=4 limit applies. Each bounded proof uses
// the supplied limits; no rational representative replaces an algebraic value.
func MapMobiusRoot(ctx context.Context, root *ExactAlgebraicRoot, domain [2]*ExactAlgebraicRoot, m [4]*big.Rat, o ExactCurvePlaneEventOptions) (*ExactAlgebraicRoot, error) {
	if ctx == nil || root == nil || root.root == nil || domain[0] == nil || domain[1] == nil {
		return nil, ErrInvalid
	}
	opts, e := graphRootOptions(o)
	if e != nil {
		return nil, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, q := range m {
		if q == nil {
			return nil, ErrInvalid
		}
		if e := b.check(q); e != nil {
			return nil, e
		}
	}
	cmp, e := domain[0].Compare(ctx, domain[1], opts)
	if e != nil {
		return nil, e
	}
	if !cmp.Proven || cmp.Order != -1 {
		return nil, ErrDomain
	}
	for i, r := range domain {
		q, e := root.Compare(ctx, r, opts)
		if e != nil {
			return nil, e
		}
		if !q.Proven {
			return nil, ErrConvergence
		}
		if i == 0 && q.Order < 0 || i == 1 && q.Order > 0 {
			return nil, ErrDomain
		}
	}
	det := new(big.Rat).Sub(new(big.Rat).Mul(m[1], m[2]), new(big.Rat).Mul(m[0], m[3]))
	if e := b.check(det); e != nil {
		return nil, e
	}
	if det.Sign() == 0 {
		return nil, ErrDegenerate
	}
	num := rationalPolynomial{new(big.Rat).Set(m[0]), new(big.Rat).Set(m[1])}
	den := rationalPolynomial{new(big.Rat).Set(m[2]), new(big.Rat).Set(m[3])}
	sign := 0
	for _, r := range domain {
		v, _, e := composedPolynomialRootSign(ctx, r.root, trimPolynomial(den), b, opts.MaxSubdivisions)
		if e != nil {
			return nil, e
		}
		if v == 0 || sign != 0 && sign != v {
			return nil, ErrDomain
		}
		sign = v
	}
	if sign < 0 {
		for _, p := range []rationalPolynomial{num, den} {
			for _, q := range p {
				q.Neg(q)
			}
		}
	}
	cell := &ExactComposedCurveCell{endpoints: [2]*ExactGraphLineRoot{domain[0].root, domain[1].root}}
	cell.power[0] = trimPolynomial(num)
	cell.power[3] = trimPolynomial(den)
	mapped, _, e := cell.CoordinateRoot(ctx, root, 0, opts)
	return mapped, e
}
