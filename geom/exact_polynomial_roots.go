package geom

import (
	"context"
	"math/big"
)

// ExactPolynomialMaxDegree is the maximum for direct bounded root isolation
// and polynomial sign evaluation. It does not change the separate limits
// for curve intersection or characteristic-polynomial construction.
const ExactPolynomialMaxDegree = 8

// ExactPolynomialRoot retains one distinct immutable root and its algebraic
// multiplicity in the supplied polynomial (ascending coefficients).
type ExactPolynomialRoot struct {
	Root         *ExactAlgebraicRoot
	Multiplicity int
}

// ExactPolynomialRootIsolation reports cumulative proof work for the complete
// root collection, so callers can charge it to a larger query budget.
type ExactPolynomialRootIsolation struct {
	Roots                    []ExactPolynomialRoot
	Operations, Subdivisions int
}

// ExactPolynomialSign reports all exact sign-proof work, including GCD and
// interval refinement, for cumulative caller accounting.
type ExactPolynomialSign struct{ Sign, Operations, Subdivisions int }

// IsolateExactPolynomialRoots proves the complete finite real-root ledger for
// a rational polynomial of degree at most ExactPolynomialMaxDegree.
// Identically zero polynomials return
// ErrDomain: coincident intervals are not isolated events. All coefficients,
// root arithmetic and subdivisions share the supplied bounded proof budget.
// No partial roots are returned on failure; inputs are copied.
func IsolateExactPolynomialRoots(ctx context.Context, coefficients []*big.Rat, lower, upper *big.Rat, o ExactCurvePlaneEventOptions) (out ExactPolynomialRootIsolation, err error) {
	defer func() {
		if err != nil {
			out.Roots = nil
		}
	}()
	if ctx == nil || len(coefficients) == 0 || len(coefficients) > ExactPolynomialMaxDegree+1 || lower == nil || upper == nil || lower.Cmp(upper) > 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.Operations = b.operations }()
	p := make(rationalPolynomial, len(coefficients))
	for i, c := range coefficients {
		if c == nil {
			return out, ErrInvalid
		}
		if e = b.check(c); e != nil {
			return out, e
		}
		p[i] = new(big.Rat).Set(c)
	}
	p = trimPolynomial(p)
	if len(p) == 0 {
		return out, ErrDomain
	}
	for _, v := range []*big.Rat{lower, upper} {
		if e = b.check(v); e != nil {
			return out, e
		}
	}
	out.Subdivisions, e = isolatePolynomialRoots(p, lower, upper, b, o.MaxSubdivisions, func(lo, hi *big.Rat, multiplicity int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		if lo.Cmp(hi) != 0 {
			for _, t := range []*big.Rat{lo, hi} {
				v, e := polynomialValue(p, t, b)
				if e != nil {
					return false, e
				}
				if v.Sign() == 0 {
					return false, nil
				}
			}
			if len(out.Roots) > 0 && out.Roots[len(out.Roots)-1].Root.Interval().Upper.Cmp(lo) == 0 {
				return false, nil
			}
		}
		poly := make(rationalPolynomial, len(p))
		for i, v := range p {
			poly[i] = new(big.Rat).Set(v)
		}
		root := &ExactAlgebraicRoot{root: &ExactGraphLineRoot{polynomial: poly, bounds: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}}}
		out.Roots = append(out.Roots, ExactPolynomialRoot{root, multiplicity})
		return true, nil
	})
	if e != nil {
		return out, e
	}
	return out, nil
}

// SignPolynomial proves the exact sign of a rational polynomial of degree at
// most ExactPolynomialMaxDegree at this immutable root. Shared factors prove
// exact zero; nonzero signs
// require outward interval exclusion. No floating root witness is used.
func (r *ExactAlgebraicRoot) SignPolynomial(ctx context.Context, coefficients []*big.Rat, o ExactCurvePlaneEventOptions) (out ExactPolynomialSign, err error) {
	if ctx == nil || r == nil || r.root == nil || len(coefficients) == 0 || len(coefficients) > ExactPolynomialMaxDegree+1 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.Operations = b.operations }()
	p := make(rationalPolynomial, len(coefficients))
	for i, c := range coefficients {
		if c == nil {
			return out, ErrInvalid
		}
		if e = b.check(c); e != nil {
			return out, e
		}
		p[i] = new(big.Rat).Set(c)
	}
	for _, v := range r.root.polynomial {
		if e = b.check(v); e != nil {
			return out, e
		}
	}
	out.Sign, out.Subdivisions, e = composedPolynomialRootSign(ctx, r.root, trimPolynomial(p), b, o.MaxSubdivisions)
	return out, e
}
