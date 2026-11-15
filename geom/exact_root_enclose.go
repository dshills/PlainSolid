package geom

import (
	"context"
	"math/big"
)

// ExactRootEnclosure retains a proved interval even when refinement stops at a
// work limit. Converged means width<=the requested exact rational maximum.
type ExactRootEnclosure struct {
	HasBounds, Converged     bool
	Interval                 ExactParameterInterval
	Operations, Subdivisions int
}

// Enclose narrows this immutable root without replacing its exact provenance.
// maxWidth must be nonnegative; zero requests a singleton and may exhaust the
// budget for an irrational root. Every returned bound is caller-owned.
func (r *ExactAlgebraicRoot) Enclose(ctx context.Context, maxWidth *big.Rat, o ExactCurvePlaneEventOptions) (out ExactRootEnclosure, err error) {
	if ctx == nil || r == nil || r.root == nil || maxWidth == nil || maxWidth.Sign() < 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	options, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { out.Operations = b.operations }()
	if e := b.check(maxWidth); e != nil {
		return out, e
	}
	out.Interval = r.Interval()
	out.HasBounds = true
	if new(big.Rat).Sub(out.Interval.Upper, out.Interval.Lower).Cmp(maxWidth) <= 0 {
		out.Converged = true
		return out, nil
	}
	steps, e := isolatePolynomialRoots(r.root.polynomial, out.Interval.Lower, out.Interval.Upper, b, options.MaxSubdivisions, func(lo, hi *big.Rat, _ int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		out.Interval = ExactParameterInterval{Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi)}
		out.Converged = new(big.Rat).Sub(hi, lo).Cmp(maxWidth) <= 0
		return out.Converged, nil
	})
	out.Subdivisions = steps
	if e != nil {
		return out, e
	}
	if !out.Converged {
		return out, ErrConvergence
	}
	return out, nil
}

// RecoverRational tries exact rational reconstruction while refining this root.
// A nil result at the requested search width leaves the root unclassified.
// Every returned candidate is verified exactly against the original polynomial.
func (r *ExactAlgebraicRoot) RecoverRational(ctx context.Context, searchWidth *big.Rat, o ExactCurvePlaneEventOptions) (*big.Rat, error) {
	if ctx == nil || r == nil || r.root == nil || searchWidth == nil || searchWidth.Sign() <= 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return nil, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	if e := b.check(searchWidth); e != nil {
		return nil, e
	}
	p := r.root.polynomial
	for _, x := range p {
		if e := b.check(x); e != nil {
			return nil, e
		}
	}
	var result *big.Rat
	accepted := false
	_, e = isolatePolynomialRoots(p, r.root.bounds[0], r.root.bounds[1], b, o.MaxSubdivisions, func(lo, hi *big.Rat, _ int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		if accepted {
			return false, ErrConvergence
		}
		candidate, e := recoverRationalPolynomialRoot(p, lo, hi, b)
		if e != nil {
			return false, e
		}
		result = candidate
		accepted = result != nil || new(big.Rat).Sub(hi, lo).Cmp(searchWidth) <= 0
		return accepted, nil
	})
	if e != nil {
		return nil, e
	}
	if !accepted {
		return nil, ErrConvergence
	}
	return result, nil
}

// NewExactRationalRoot constructs an immutable singleton algebraic identity
// directly, with no text codec or numerical recovery. The input is copied.
func NewExactRationalRoot(ctx context.Context, value *big.Rat, o ExactCurvePlaneEventOptions) (root *ExactAlgebraicRoot, operations int, err error) {
	if ctx == nil || value == nil {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	options, e := graphRootOptions(o)
	if e != nil {
		return nil, 0, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	if e := b.check(value); e != nil {
		return nil, 0, e
	}
	return &ExactAlgebraicRoot{root: &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(value), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat).Set(value), new(big.Rat).Set(value)}}}, b.operations, nil
}
