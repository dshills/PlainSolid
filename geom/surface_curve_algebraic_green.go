package geom

import (
	"context"
	"math/big"
)

// ProjectedGreenBounds encloses the signed Green integral 1/2 ∫(x dy − y dx)
// in the ordered projection axes, in increasing cell parameter.
type ProjectedGreenBounds struct {
	Interval                 ExactParameterInterval
	Operations, Subdivisions int
}

// BoundPolynomialProjectedGreen integrates a polynomial model cell exactly up
// to a rational enclosure of the algebraic endpoint values. A nonconstant
// homogeneous weight is outside this operation's domain. The requested width
// is an exact positive rational. Limits and cancellation return no interval.
func (c *ExactComposedCurveCell) BoundPolynomialProjectedGreen(ctx context.Context, axes [2]int, width *big.Rat, options ExactCurvePlaneEventOptions) (r ProjectedGreenBounds, err error) {
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
	if e := b.check(width); e != nil {
		return r, e
	}
	x, y := c.power[axes[0]], c.power[axes[1]]
	xy, e := polynomialProduct(x, polynomialDerivative(y), b)
	if e != nil {
		return r, e
	}
	yx, e := polynomialProduct(y, polynomialDerivative(x), b)
	if e != nil {
		return r, e
	}
	p, e := subtractScaledPolynomial(xy, yx, big.NewRat(1, 1), b)
	if e != nil {
		return r, e
	}
	primitive := make(rationalPolynomial, len(p)+1)
	primitive[0] = new(big.Rat)
	weight := c.power[3][0]
	scale := new(big.Rat).Mul(weight, weight)
	scale.Mul(scale, big.NewRat(2, 1))
	for i, q := range p {
		primitive[i+1] = new(big.Rat).Quo(q, new(big.Rat).Mul(scale, big.NewRat(int64(i+1), 1)))
		if e := b.check(primitive[i+1]); e != nil {
			return r, e
		}
	}
	value, splits, e := polynomialPrimitiveOnCell(ctx, c, primitive, width, o.MaxSubdivisions, b)
	r.Subdivisions = splits
	if e != nil {
		return r, e
	}
	r.Interval = ExactParameterInterval{Lower: value.lo, Upper: value.hi}
	return r, nil
}

func polynomialPrimitiveOnCell(ctx context.Context, c *ExactComposedCurveCell, primitive rationalPolynomial, width *big.Rat,
	maxSubdivisions int, b *polynomialBudget) (exactInterval, int, error) {
	var zero exactInterval
	splits := 0
	// Refine against a Lipschitz bound derived from the antiderivative's
	// derivative on both original endpoint enclosures. An interval evaluation
	// of the antiderivative itself can be looser; evaluating its midpoint and
	// applying the same Lipschitz bound avoids dependency overestimates.
	derivative := polynomialDerivative(primitive)
	var ends [2]exactInterval
	for i, root := range c.endpoints {
		if e := ctx.Err(); e != nil {
			return zero, splits, e
		}
		d, e := polynomialBernsteinInterval(derivative, exactInterval{lo: root.bounds[0], hi: root.bounds[1]}, b)
		if e != nil {
			return zero, splits, e
		}
		speed := new(big.Rat).Abs(d.lo)
		if upper := new(big.Rat).Abs(d.hi); speed.Cmp(upper) < 0 {
			speed = upper
		}
		a, z := root.bounds[0], root.bounds[1]
		if speed.Sign() != 0 && a.Cmp(z) != 0 {
			lim := new(big.Rat).Quo(width, new(big.Rat).Mul(big.NewRat(2, 1), speed))
			if b.maxOperations-b.operations < 1 || maxSubdivisions-splits < 1 {
				return zero, splits, ErrConvergence
			}
			q, e := (&ExactAlgebraicRoot{root}).Enclose(ctx, lim, ExactCurvePlaneEventOptions{
				MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits, MaxSubdivisions: maxSubdivisions - splits,
			})
			b.operations += q.Operations
			splits += q.Subdivisions
			if e != nil {
				return zero, splits, e
			}
			if !q.Converged {
				return zero, splits, ErrConvergence
			}
			a, z = q.Interval.Lower, q.Interval.Upper
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(a, z), big.NewRat(2, 1))
		v, e := polynomialValue(primitive, mid, b)
		if e != nil {
			return zero, splits, e
		}
		radius := new(big.Rat).Mul(speed, new(big.Rat).Sub(z, mid))
		ends[i] = exactInterval{lo: new(big.Rat).Sub(v, radius), hi: new(big.Rat).Add(v, radius)}
	}
	value, e := intervalSubtract(ends[1], ends[0], b)
	if e != nil {
		return zero, splits, e
	}
	if new(big.Rat).Sub(value.hi, value.lo).Cmp(width) > 0 {
		return zero, splits, ErrConvergence
	}
	return value, splits, nil
}
