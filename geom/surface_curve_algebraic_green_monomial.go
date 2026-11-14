package geom

import (
	"context"
	"math/big"
)

// BoundProjectedMonomialGreen encloses the signed integral of
// x^(i+1)*y^j/(i+1) dy on the cell, where x and y are the selected axes.
// On an oriented closed wire this is the area moment of x^i*y^j. Total
// degree up to three is admitted, including positive rational weights and
// algebraic cuts. The result uses the original curve parameter.
func (c *ExactComposedCurveCell) BoundProjectedMonomialGreen(ctx context.Context, axes [2]int, powers [2]int,
	width *big.Rat, options ExactCurvePlaneEventOptions) (r ProjectedGreenBounds, err error) {
	if ctx == nil || c == nil || width == nil || width.Sign() <= 0 ||
		axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] ||
		powers[0] < 0 || powers[0] > 3 || powers[1] < 0 || powers[1] > 3 || powers[0]+powers[1] > 3 ||
		c.endpoints[0] == nil || c.endpoints[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(options)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	if e := b.check(width); e != nil {
		return r, e
	}
	x, y, w := c.power[axes[0]], c.power[axes[1]], c.power[3]
	// In homogeneous coordinates dy = (Y'W - YW')/W².
	dy, e := polynomialProduct(polynomialDerivative(y), w, b)
	if e != nil {
		return r, e
	}
	yw, e := polynomialProduct(y, polynomialDerivative(w), b)
	if e != nil {
		return r, e
	}
	p, e := subtractScaledPolynomial(dy, yw, big.NewRat(1, 1), b)
	if e != nil {
		return r, e
	}
	for axis, row := range [2]rationalPolynomial{x, y} {
		n := powers[axis]
		if axis == 0 {
			n++
		}
		for range n {
			p, e = polynomialProduct(p, row, b)
			if e != nil {
				return r, e
			}
		}
	}
	div := big.NewRat(int64(powers[0]+1), 1)
	exponent := powers[0] + powers[1] + 3
	if len(w) == 1 {
		// Integrate the polynomial exactly before enclosing its endpoint
		// values. This is substantially tighter than interval quadrature.
		for range exponent {
			div.Mul(div, w[0])
			if e := b.check(div); e != nil {
				return r, e
			}
		}
		if div.Sign() <= 0 {
			return r, ErrUnrepresentable
		}
		primitive := make(rationalPolynomial, len(p)+1)
		primitive[0] = new(big.Rat)
		for i, a := range p {
			primitive[i+1] = new(big.Rat).Quo(a, new(big.Rat).Mul(div, big.NewRat(int64(i+1), 1)))
			if e := b.check(primitive[i+1]); e != nil {
				return r, e
			}
		}
		bound, splits, e := polynomialPrimitiveOnCell(ctx, c, primitive, width, o.MaxSubdivisions, b)
		r.Subdivisions = splits
		if e != nil {
			return r, e
		}
		r.Interval = ExactParameterInterval{Lower: bound.lo, Upper: bound.hi}
		return r, nil
	}
	for _, a := range p {
		a.Quo(a, div)
		if e := b.check(a); e != nil {
			return r, e
		}
	}
	d := rationalPolynomial{big.NewRat(1, 1)}
	for range exponent {
		d, e = polynomialProduct(d, w, b)
		if e != nil {
			return r, e
		}
	}
	return c.integrateRationalGreen(ctx, p, d, width, o, b)
}
