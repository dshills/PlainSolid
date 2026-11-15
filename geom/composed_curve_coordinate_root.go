package geom

import (
	"context"
	"math/big"
)

// CoordinateRoot proves the exact coordinate N(root)/W(root) of a cell point.
// General coordinate images support defining polynomials of degree at most
// eight. Exact reduction modulo a quadratic yields an affine expression.
// Higher degrees use an affine shortcut where possible and otherwise isolate
// the unique image root of a quotient-algebra characteristic polynomial.
// An exactly isolated rational event can use its verified linear identity.
// Membership in the true cell domain and a nonzero denominator are rechecked.
func (c *ExactComposedCurveCell) CoordinateRoot(ctx context.Context, root *ExactAlgebraicRoot, axis int, o ExactCurvePlaneEventOptions) (out *ExactAlgebraicRoot, operations int, err error) {
	image, work, _, err := c.CoordinateRootWithWork(ctx, root, axis, o)
	return image, work, err
}

// CoordinateRootWithWork returns the same immutable coordinate identity and
// both work counters, for callers sharing one budget across several images.
func (c *ExactComposedCurveCell) CoordinateRootWithWork(ctx context.Context, root *ExactAlgebraicRoot, axis int, o ExactCurvePlaneEventOptions) (out *ExactAlgebraicRoot, operations, subdivisions int, err error) {
	if ctx == nil || c == nil || root == nil || root.root == nil || axis < 0 || axis > 2 || len(c.power[3]) == 0 || c.endpoints[0] == nil || c.endpoints[1] == nil {
		return nil, 0, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, 0, e
	}
	// Some private constructed coordinate maps retain explicit zero
	// high-order terms. Polynomial division requires the actual degree.
	canonical := *c
	for i, p := range canonical.power {
		canonical.power[i] = trimPolynomial(p)
	}
	c = &canonical
	if len(c.power[3]) == 0 {
		return nil, 0, 0, ErrDegenerate
	}
	options, e := graphRootOptions(o)
	if e != nil {
		return nil, 0, 0, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	if source := root.root; len(source.polynomial) > 5 && source.bounds[0].Cmp(source.bounds[1]) == 0 {
		// A composed weighted residual can retain extraneous algebraic
		// factors even when isolation has already proved an exact rational
		// event. Use its verified linear identity for the coordinate image.
		parameter := source.bounds[0]
		value, e := polynomialValue(source.polynomial, parameter, b)
		if e != nil {
			return nil, 0, subdivisions, e
		}
		if value.Sign() != 0 {
			return nil, 0, subdivisions, ErrConvergence
		}
		isolated := *source
		isolated.polynomial = rationalPolynomial{new(big.Rat).Neg(parameter), big.NewRat(1, 1)}
		root = &ExactAlgebraicRoot{root: &isolated}
	}
	if len(root.root.polynomial) > 9 || len(root.root.polynomial) < 2 {
		return nil, 0, subdivisions, ErrConvergence
	}
	if len(root.root.polynomial) > 5 {
		for _, poly := range append([]rationalPolynomial{root.root.polynomial}, c.power[:]...) {
			for _, q := range poly {
				if e := b.check(q); e != nil {
					return nil, 0, subdivisions, e
				}
			}
		}
	}
	for endpoint := 0; endpoint < 2; endpoint++ {
		comparison, e := compareRootBudget(ctx, root.root, c.endpoints[endpoint], b, options.MaxSubdivisions-subdivisions)
		subdivisions += comparison.Subdivisions
		if e != nil {
			return nil, 0, subdivisions, e
		}
		if !comparison.Proven {
			return nil, 0, subdivisions, ErrConvergence
		}
		if endpoint == 0 && comparison.Order < 0 || endpoint == 1 && comparison.Order > 0 {
			return nil, 0, subdivisions, ErrDomain
		}
	}
	remaining := options.MaxSubdivisions - subdivisions
	if remaining < 1 {
		return nil, 0, subdivisions, ErrConvergence
	}
	sign, signSteps, e := composedPolynomialRootSign(ctx, root.root, c.power[3], b, remaining)
	subdivisions += signSteps
	if e != nil {
		return nil, 0, subdivisions, e
	}
	if sign == 0 {
		return nil, 0, subdivisions, ErrDegenerate
	}
	p := root.root.polynomial
	// A denominator may vanish only at the unused rational conjugate. Remove
	// those factors after proving it does not vanish at the selected source root.
	gcd, e := polynomialGCD(p, c.power[3], b)
	if e != nil {
		return nil, 0, subdivisions, e
	}
	if len(gcd) > 1 {
		var remainder rationalPolynomial
		p, remainder, e = polynomialDivide(p, gcd, b)
		if e != nil {
			return nil, 0, subdivisions, e
		}
		if len(remainder) != 0 {
			return nil, 0, subdivisions, ErrConvergence
		}
	}
	_, n, e := polynomialDivide(c.power[axis], p, b)
	if e != nil {
		return nil, 0, subdivisions, e
	}
	_, w, e := polynomialDivide(c.power[3], p, b)
	if e != nil {
		return nil, 0, subdivisions, e
	}

	if len(p) > 5 && len(w) == 1 && w[0].Sign() != 0 && len(n) <= 2 {
		offset, scale := new(big.Rat), new(big.Rat)
		if len(n) > 0 {
			offset.Quo(n[0], w[0])
		}
		if len(n) > 1 {
			scale.Quo(n[1], w[0])
		}
		for _, q := range []*big.Rat{offset, scale} {
			if e := b.check(q); e != nil {
				return nil, 0, subdivisions, e
			}
		}
		source := copyExactRoot(root.root)
		source.polynomial = p
		mapped, e := affineMapRoot(source, offset, scale, b)
		if e != nil {
			return nil, 0, subdivisions, e
		}
		if e = ctx.Err(); e != nil {
			return nil, 0, subdivisions, e
		}
		return &ExactAlgebraicRoot{root: mapped}, b.operations, subdivisions, nil
	}
	if len(p) > 3 {
		mapped, used, e := coordinateRootCharacteristic(ctx, root.root, p, n, w, b, options.MaxSubdivisions-subdivisions)
		subdivisions += used
		if e != nil {
			return nil, 0, subdivisions, e
		}
		return &ExactAlgebraicRoot{root: mapped}, b.operations, subdivisions, nil
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
			return nil, 0, subdivisions, ErrDegenerate
		}
		offset.Quo(at(n, 0), at(w, 0))
	} else if len(p) == 3 {
		beta := new(big.Rat).Quo(p[1], p[2])
		gamma := new(big.Rat).Quo(p[0], p[2])
		n0, n1, w0, w1 := at(n, 0), at(n, 1), at(w, 0), at(w, 1)
		conjugate := new(big.Rat).Sub(w0, new(big.Rat).Mul(beta, w1))
		norm := new(big.Rat).Add(new(big.Rat).Mul(w0, conjugate), new(big.Rat).Mul(gamma, new(big.Rat).Mul(w1, w1)))
		if norm.Sign() == 0 {
			return nil, 0, subdivisions, ErrConvergence
		}
		offset.Quo(new(big.Rat).Add(new(big.Rat).Mul(n0, conjugate), new(big.Rat).Mul(gamma, new(big.Rat).Mul(n1, w1))), norm)
		scale.Quo(new(big.Rat).Sub(new(big.Rat).Mul(n1, w0), new(big.Rat).Mul(n0, w1)), norm)
	} else {
		return nil, 0, subdivisions, ErrConvergence
	}
	for _, q := range []*big.Rat{offset, scale} {
		if e := b.check(q); e != nil {
			return nil, 0, subdivisions, e
		}
	}
	source := copyExactRoot(root.root)
	source.polynomial = p
	mapped, e := affineMapRoot(source, offset, scale, b)
	if e != nil {
		return nil, 0, subdivisions, e
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, subdivisions, e
	}
	return &ExactAlgebraicRoot{root: mapped}, b.operations, subdivisions, nil
}
