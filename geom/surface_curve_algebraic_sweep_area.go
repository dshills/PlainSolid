package geom

import (
	"context"
	"math/big"
)

// BoundVerticalSweepArea encloses ∫ (a+b*x+c*y) sqrt(x'²+y'²) dt
// along a retained rational model curve. The given axes define the
// footprint. The supplied affine height gap must be nonnegative; the
// integral is of its absolute value on the represented domain.
// The result uses outward exact interval bounds on the true domain.
func (c *ExactComposedCurveCell) BoundVerticalSweepArea(ctx context.Context, axes [2]int, height [3]*big.Rat, width *big.Rat,
	opts ExactCurvePlaneEventOptions) (r ProjectedGreenBounds, err error) {
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 ||
		axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] || width == nil || width.Sign() <= 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(opts)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	if e := b.check(width); e != nil {
		return r, e
	}
	for _, v := range height {
		if v == nil {
			return r, ErrInvalid
		}
		if e := b.check(v); e != nil {
			return r, e
		}
	}
	w := c.power[3]
	h, e := subtractScaledPolynomial(nil, w, new(big.Rat).Neg(height[0]), b)
	if e != nil {
		return r, e
	}
	var speed rationalPolynomial
	for i, axis := range axes {
		h, e = subtractScaledPolynomial(h, c.power[axis], new(big.Rat).Neg(height[i+1]), b)
		if e != nil {
			return r, e
		}
		p, e := polynomialProduct(polynomialDerivative(c.power[axis]), w, b)
		if e != nil {
			return r, e
		}
		q, e := polynomialProduct(c.power[axis], polynomialDerivative(w), b)
		if e != nil {
			return r, e
		}
		d, e := subtractScaledPolynomial(p, q, big.NewRat(1, 1), b)
		if e != nil {
			return r, e
		}
		s, e := polynomialProduct(d, d, b)
		if e != nil {
			return r, e
		}
		speed, e = subtractScaledPolynomial(speed, s, big.NewRat(-1, 1), b)
		if e != nil {
			return r, e
		}
	}
	// Squared density is H²*(DX²+DY²)/W⁶. Treating the square
	// as one exact polynomial retains endpoint zeroes and cancellations.
	h2, e := polynomialProduct(h, h, b)
	if e != nil {
		return r, e
	}
	num, e := polynomialProduct(h2, speed, b)
	if e != nil {
		return r, e
	}
	density := func(a, z *big.Rat) (exactInterval, error) {
		d := exactInterval{lo: a, hi: z}
		h, e := polynomialBernsteinInterval(w, d, b)
		if e != nil {
			return exactInterval{}, e
		}
		if h.lo.Sign() <= 0 {
			// Nonnegative control weights prove positivity on this
			// original cell, but an interval may fail to reprove it.
			return exactInterval{}, ErrConvergence
		}
		q, e := polynomialBernsteinInterval(num, d, b)
		if e != nil {
			return exactInterval{}, e
		}
		if q.hi.Sign() < 0 {
			return exactInterval{}, ErrConvergence
		}
		if q.lo.Sign() < 0 {
			q.lo = new(big.Rat)
		}
		den := exactInterval{lo: new(big.Rat).Set(h.lo), hi: new(big.Rat).Set(h.hi)}
		for i := 1; i < 6; i++ {
			den, e = intervalMultiply(den, h, b)
			if e != nil {
				return exactInterval{}, e
			}
		}
		s, e := intervalDivide(q, den, b)
		if e != nil {
			return exactInterval{}, e
		}
		bits := min(96, (b.maxBits-max(s.lo.Num().BitLen(), s.hi.Num().BitLen()))/2)
		if bits < 4 {
			return exactInterval{}, ErrConvergence
		}
		lo, _, e := rationalSqrtBounds(s.lo, bits, b)
		if e != nil {
			return exactInterval{}, e
		}
		_, hi, e := rationalSqrtBounds(s.hi, bits, b)
		if e != nil {
			return exactInterval{}, e
		}
		return exactInterval{lo, hi}, nil
	}
	// The nonnegative integrand on the two endpoint fringes only
	// increases the upper total, so no signed cancellation is hidden.
	var inside [2]*big.Rat
	total := exactInterval{new(big.Rat), new(big.Rat)}
	for i, root := range c.endpoints {
		a, z := root.bounds[0], root.bounds[1]
		if a.Cmp(z) != 0 {
			q, e := density(a, z)
			if e != nil {
				return r, e
			}
			if q.hi.Sign() != 0 {
				wanted := new(big.Rat).Quo(width, new(big.Rat).Mul(q.hi, big.NewRat(8, 1)))
				ops, steps := o.MaxOperations-b.operations, o.MaxSubdivisions-r.Subdivisions
				if ops < 1 || steps < 1 {
					return r, ErrConvergence
				}
				v, e := (&ExactAlgebraicRoot{root}).Enclose(ctx, wanted, ExactCurvePlaneEventOptions{MaxOperations: ops, MaxSubdivisions: steps, MaxCoefficientBits: b.maxBits})
				b.operations += v.Operations
				r.Subdivisions += v.Subdivisions
				if e != nil {
					return r, e
				}
				if !v.Converged {
					return r, ErrConvergence
				}
				a, z = v.Interval.Lower, v.Interval.Upper
				total.hi.Add(total.hi, new(big.Rat).Mul(q.hi, new(big.Rat).Sub(z, a)))
			}
		}
		inside[i] = z
		if i == 1 {
			inside[i] = a
		}
	}
	if inside[0].Cmp(inside[1]) > 0 {
		return r, ErrConvergence
	}
	type region struct{ a, z, allowed *big.Rat }
	pending := []region{{inside[0], inside[1], new(big.Rat).Quo(width, big.NewRat(2, 1))}}
	for len(pending) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		q := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if q.a.Cmp(q.z) == 0 {
			continue
		}
		d, e := density(q.a, q.z)
		if e != nil && e != ErrConvergence {
			return r, e
		}
		if e == nil {
			dt := new(big.Rat).Sub(q.z, q.a)
			lo, hi := new(big.Rat).Mul(d.lo, dt), new(big.Rat).Mul(d.hi, dt)
			if new(big.Rat).Sub(hi, lo).Cmp(q.allowed) <= 0 {
				total.lo.Add(total.lo, lo)
				total.hi.Add(total.hi, hi)
				for _, v := range []*big.Rat{total.lo, total.hi} {
					if e := b.check(v); e != nil {
						return r, e
					}
				}
				continue
			}
		}
		r.Subdivisions++
		if r.Subdivisions >= o.MaxSubdivisions || b.operations >= o.MaxOperations {
			return r, ErrConvergence
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(q.a, q.z), big.NewRat(2, 1))
		half := new(big.Rat).Quo(q.allowed, big.NewRat(2, 1))
		pending = append(pending, region{mid, q.z, half}, region{q.a, mid, half})
	}
	if new(big.Rat).Sub(total.hi, total.lo).Cmp(width) > 0 {
		return r, ErrConvergence
	}
	r.Interval = ExactParameterInterval{Lower: total.lo, Upper: total.hi}
	return r, nil
}
