package geom

import (
	"context"
	"math/big"
)

// CoordinateRoot proves the exact coordinate N(root)/W(root) of a cell point.
// The source root must have a defining polynomial of degree at most four; the
// coordinate polynomials may have any supported cell degree. Exact reduction
// modulo a quadratic yields an affine expression; degree three/four uses a
// quotient-algebra characteristic polynomial with unique image-root isolation. No interval midpoint is used as the algebraic value.
// Membership in the true cell domain and a nonzero denominator are rechecked.
func (c *ExactComposedCurveCell) CoordinateRoot(ctx context.Context, root *ExactAlgebraicRoot, axis int, o ExactCurvePlaneEventOptions) (out *ExactAlgebraicRoot, operations int, err error) {
	if ctx == nil || c == nil || root == nil || root.root == nil || axis < 0 || axis > 2 || len(c.power[3]) == 0 || c.endpoints[0] == nil || c.endpoints[1] == nil {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	// Some private constructed coordinate maps retain explicit zero
	// high-order terms. Polynomial division requires the actual degree.
	canonical := *c
	for i, p := range canonical.power {
		canonical.power[i] = trimPolynomial(p)
	}
	c = &canonical
	if len(c.power[3]) == 0 {
		return nil, 0, ErrDegenerate
	}
	options, e := graphRootOptions(o)
	if e != nil {
		return nil, 0, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	if len(root.root.polynomial) > 5 || len(root.root.polynomial) < 2 {
		return nil, 0, ErrConvergence
	}
	subdivisions := 0
	for endpoint := 0; endpoint < 2; endpoint++ {
		comparison, e := compareRootBudget(ctx, root.root, c.endpoints[endpoint], b, options.MaxSubdivisions-subdivisions)
		subdivisions += comparison.Subdivisions
		if e != nil {
			return nil, 0, e
		}
		if !comparison.Proven {
			return nil, 0, ErrConvergence
		}
		if endpoint == 0 && comparison.Order < 0 || endpoint == 1 && comparison.Order > 0 {
			return nil, 0, ErrDomain
		}
	}
	remaining := options.MaxSubdivisions - subdivisions
	if remaining < 1 {
		return nil, 0, ErrConvergence
	}
	sign, signSteps, e := composedPolynomialRootSign(ctx, root.root, c.power[3], b, remaining)
	if e != nil {
		return nil, 0, e
	}
	subdivisions += signSteps
	if sign == 0 {
		return nil, 0, ErrDegenerate
	}
	p := root.root.polynomial
	// A denominator may vanish only at the unused rational conjugate. Remove
	// those factors after proving it does not vanish at the selected source root.
	gcd, e := polynomialGCD(p, c.power[3], b)
	if e != nil {
		return nil, 0, e
	}
	if len(gcd) > 1 {
		var remainder rationalPolynomial
		p, remainder, e = polynomialDivide(p, gcd, b)
		if e != nil {
			return nil, 0, e
		}
		if len(remainder) != 0 {
			return nil, 0, ErrConvergence
		}
	}
	_, n, e := polynomialDivide(c.power[axis], p, b)
	if e != nil {
		return nil, 0, e
	}
	_, w, e := polynomialDivide(c.power[3], p, b)
	if e != nil {
		return nil, 0, e
	}

	if len(p) > 3 {
		mapped, e := coordinateRootCharacteristic(ctx, root.root, p, n, w, b, options.MaxSubdivisions-subdivisions)
		if e != nil {
			return nil, 0, e
		}
		return &ExactAlgebraicRoot{root: mapped}, b.operations, nil
	}
	at := func(poly rationalPolynomial, i int) *big.Rat {
		if i >= len(poly) {
			return new(big.Rat)
		}
		return poly[i]
	}
	offset, scale := new(big.Rat), new(big.Rat)
	if len(p) == 2 {
		if at(w, 0).Sign() == 0 {
			return nil, 0, ErrDegenerate
		}
		offset.Quo(at(n, 0), at(w, 0))
	} else if len(p) == 3 {
		beta := new(big.Rat).Quo(p[1], p[2])
		gamma := new(big.Rat).Quo(p[0], p[2])
		n0, n1, w0, w1 := at(n, 0), at(n, 1), at(w, 0), at(w, 1)
		conjugate := new(big.Rat).Sub(w0, new(big.Rat).Mul(beta, w1))
		norm := new(big.Rat).Add(new(big.Rat).Mul(w0, conjugate), new(big.Rat).Mul(gamma, new(big.Rat).Mul(w1, w1)))
		if norm.Sign() == 0 {
			return nil, 0, ErrConvergence
		}
		offset.Quo(new(big.Rat).Add(new(big.Rat).Mul(n0, conjugate), new(big.Rat).Mul(gamma, new(big.Rat).Mul(n1, w1))), norm)
		scale.Quo(new(big.Rat).Sub(new(big.Rat).Mul(n1, w0), new(big.Rat).Mul(n0, w1)), norm)
	} else {
		return nil, 0, ErrConvergence
	}
	for _, q := range []*big.Rat{offset, scale} {
		if e := b.check(q); e != nil {
			return nil, 0, e
		}
	}
	source := copyExactRoot(root.root)
	source.polynomial = p
	mapped, e := affineMapRoot(source, offset, scale, b)
	if e != nil {
		return nil, 0, e
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	return &ExactAlgebraicRoot{root: mapped}, b.operations, nil
}
