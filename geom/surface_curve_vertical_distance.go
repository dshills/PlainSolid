package geom

import (
	"context"
	"math/big"
)

// AffineHeightCurveDistanceCandidates finds the exact stationary distances
// to one coordinate-preserving curve in coordinates axes. A nil height
// drops the third axis from the metric. Otherwise height specifies
// a+b*x+c*y for the third coordinate mapped back from this footprint.
// Returning the original model parameter supports further trim filtering.
func (c *ExactComposedCurveCell) AffineHeightCurveDistanceCandidates(ctx context.Context, axes [2]int, height *[3]*big.Rat, point [3]*big.Rat,
	o ExactCurvePlaneEventOptions) (r PolynomialCurveDistance, err error) {
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 ||
		axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	out, q, e := c.affineHeightDistanceCell(axes, height, point, b)
	if e != nil {
		return r, e
	}

	o.MaxOperations -= b.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	r, e = out.RationalQuadraticDistanceCandidates(ctx, q, o)
	r.Operations += b.operations
	return r, e
}

// AffineHeightCurveSquaredDistance evaluates the same exact transformed
// metric at a retained root, including a switch that is not stationary.
func (c *ExactComposedCurveCell) AffineHeightCurveSquaredDistance(ctx context.Context, axes [2]int, height *[3]*big.Rat, point [3]*big.Rat,
	root *ExactAlgebraicRoot, o ExactCurvePlaneEventOptions) (*ExactAlgebraicRoot, int, error) {
	if ctx == nil || c == nil || root == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 ||
		axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return nil, 0, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	cell, p, e := c.affineHeightDistanceCell(axes, height, point, b)
	if e != nil {
		return nil, b.operations, e
	}
	if len(cell.power[3]) < 1 || len(cell.power[3]) > 3 {
		return nil, b.operations, ErrUnrepresentable
	}
	var n rationalPolynomial
	for axis, v := range p {
		if v == nil {
			return nil, b.operations, ErrInvalid
		}
		if e := b.check(v); e != nil {
			return nil, b.operations, e
		}
		if len(cell.power[axis]) > 3 {
			return nil, b.operations, ErrUnrepresentable
		}
		d, e := subtractScaledPolynomial(cell.power[axis], cell.power[3], v, b)
		if e != nil {
			return nil, b.operations, e
		}
		q, e := polynomialProduct(d, d, b)
		if e != nil {
			return nil, b.operations, e
		}
		n, e = subtractScaledPolynomial(n, q, big.NewRat(-1, 1), b)
		if e != nil {
			return nil, b.operations, e
		}
	}
	den, e := polynomialProduct(cell.power[3], cell.power[3], b)
	if e != nil {
		return nil, b.operations, e
	}
	if len(n) == 0 {
		n = rationalPolynomial{new(big.Rat)}
	}
	cell.power = [4]rationalPolynomial{n, nil, nil, den}
	o.MaxOperations -= b.operations
	if o.MaxOperations < 1 {
		return nil, b.operations, ErrConvergence
	}
	v, ops, e := cell.CoordinateRoot(ctx, root, 0, o)
	return v, ops + b.operations, e
}

func (c *ExactComposedCurveCell) affineHeightDistanceCell(axes [2]int, height *[3]*big.Rat, point [3]*big.Rat, b *polynomialBudget) (ExactComposedCurveCell, [3]*big.Rat, error) {
	out := *c
	for _, p := range point {
		if p == nil {
			return out, point, ErrInvalid
		}
		if e := b.check(p); e != nil {
			return out, point, e
		}
	}
	drop := 3 - axes[0] - axes[1]
	if height == nil {
		out.power[drop] = nil
		point[drop] = new(big.Rat)
	} else {
		for _, v := range height {
			if v == nil {
				return out, point, ErrInvalid
			}
			if e := b.check(v); e != nil {
				return out, point, e
			}
		}
		v, e := subtractScaledPolynomial(nil, c.power[3], new(big.Rat).Neg(height[0]), b)
		if e != nil {
			return out, point, e
		}
		for i, axis := range axes {
			v, e = subtractScaledPolynomial(v, c.power[axis], new(big.Rat).Neg(height[i+1]), b)
			if e != nil {
				return out, point, e
			}
		}
		out.power[drop] = v
	}
	return out, point, nil
}
