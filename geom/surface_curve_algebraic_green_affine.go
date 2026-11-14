package geom

import (
	"context"
	"math/big"
)

// BoundPolynomialProjectedAffineGreen bounds ∫(a*x+b*x²/2+c*x*y)dy
// on this oriented polynomial model edge, where (x,y) are the selected
// ordered axes and affine=(a,b,c). On an oriented closed wire the signed
// integral equals the planar integral of a+b*x+c*y. Varying homogeneous
// weights require a different integrator and are explicitly rejected.
func (c *ExactComposedCurveCell) BoundPolynomialProjectedAffineGreen(ctx context.Context, axes [2]int, affine [3]*big.Rat,
	width *big.Rat, options ExactCurvePlaneEventOptions) (r ProjectedGreenBounds, err error) {
	if ctx == nil || c == nil || width == nil || width.Sign() <= 0 ||
		axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] ||
		c.endpoints[0] == nil || c.endpoints[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if len(c.power[3]) != 1 || c.power[3][0].Sign() <= 0 {
		return r, ErrUnrepresentable
	}
	o, e := graphRootOptions(options)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	for _, v := range append(affine[:], width) {
		if v == nil {
			return r, ErrInvalid
		}
		if e := b.check(v); e != nil {
			return r, e
		}
	}
	var coordinates [2]rationalPolynomial
	for i, axis := range axes {
		for _, v := range c.power[axis] {
			q := new(big.Rat).Quo(v, c.power[3][0])
			if e := b.check(q); e != nil {
				return r, e
			}
			coordinates[i] = append(coordinates[i], q)
		}
	}
	x, y := coordinates[0], coordinates[1]
	// x(a+b*x/2+c*y) is the exact vertical primitive for the
	// affine density, regardless of the sign of dy in this edge.
	p, e := subtractScaledPolynomial(rationalPolynomial{affine[0]}, x, new(big.Rat).Quo(new(big.Rat).Neg(affine[1]), big.NewRat(2, 1)), b)
	if e != nil {
		return r, e
	}
	p, e = subtractScaledPolynomial(p, y, new(big.Rat).Neg(affine[2]), b)
	if e != nil {
		return r, e
	}
	p, e = polynomialProduct(x, p, b)
	if e != nil {
		return r, e
	}
	p, e = polynomialProduct(p, polynomialDerivative(y), b)
	if e != nil {
		return r, e
	}
	primitive := make(rationalPolynomial, len(p)+1)
	primitive[0] = new(big.Rat)
	for i, v := range p {
		primitive[i+1] = new(big.Rat).Quo(v, big.NewRat(int64(i+1), 1))
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
