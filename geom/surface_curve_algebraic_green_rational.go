package geom

import (
	"context"
	"math/big"
)

// BoundProjectedGreen encloses the same integral as
// BoundPolynomialProjectedGreen and additionally accepts rational model
// cells. A positive denominator is proved over every integration interval.
// Subdivision and root refinement consume one shared budget.
func (c *ExactComposedCurveCell) BoundProjectedGreen(ctx context.Context, axes [2]int, width *big.Rat, options ExactCurvePlaneEventOptions) (r ProjectedGreenBounds, err error) {
	if ctx == nil || c == nil || width == nil || width.Sign() <= 0 ||
		axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] ||
		c.endpoints[0] == nil || c.endpoints[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if len(c.power[3]) == 1 {
		return c.BoundPolynomialProjectedGreen(ctx, axes, width, options)
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
	// The terms involving W' cancel, so the exact Green density is p/(2W²).
	den, e := polynomialProduct(w, w, b)
	if e != nil {
		return r, e
	}
	for _, v := range den {
		v.Mul(v, big.NewRat(2, 1))
		if e := b.check(v); e != nil {
			return r, e
		}
	}
	boundDensity := func(a, z *big.Rat) (exactInterval, error) {
		domain := exactInterval{lo: a, hi: z}
		pb, e := polynomialBernsteinInterval(p, domain, b)
		if e != nil {
			return exactInterval{}, e
		}
		db, e := polynomialBernsteinInterval(den, domain, b)
		if e != nil {
			return exactInterval{}, e
		}
		if db.lo.Sign() <= 0 {
			return exactInterval{}, ErrConvergence
		}
		return intervalDivide(pb, db, b)
	}

	// Integrate the interval guaranteed to be in the true domain and bound
	// the two remaining algebraic endpoint fringes in the original parameter.
	var inside [2]*big.Rat
	lower, upper := new(big.Rat), new(big.Rat)
	for side, root := range c.endpoints {
		a, z := root.bounds[0], root.bounds[1]
		if a.Cmp(z) != 0 {
			d, e := boundDensity(a, z)
			if e != nil {
				return r, e
			}
			m := new(big.Rat).Abs(d.lo)
			if n := new(big.Rat).Abs(d.hi); m.Cmp(n) < 0 {
				m = n
			}
			if m.Sign() != 0 {
				lim := new(big.Rat).Quo(width, new(big.Rat).Mul(m, big.NewRat(8, 1)))
				if b.maxOperations-b.operations < 1 || o.MaxSubdivisions-r.Subdivisions < 1 {
					return r, ErrConvergence
				}
				q, e := (&ExactAlgebraicRoot{root}).Enclose(ctx, lim, ExactCurvePlaneEventOptions{
					MaxOperations: b.maxOperations - b.operations, MaxSubdivisions: o.MaxSubdivisions - r.Subdivisions, MaxCoefficientBits: b.maxBits,
				})
				b.operations += q.Operations
				r.Subdivisions += q.Subdivisions
				if e != nil {
					return r, e
				}
				a, z = q.Interval.Lower, q.Interval.Upper
				errorBound := new(big.Rat).Mul(m, new(big.Rat).Sub(z, a))
				lower.Sub(lower, errorBound)
				upper.Add(upper, errorBound)
			}
		}
		inside[side] = z
		if side == 1 {
			inside[side] = a
		}
	}
	if inside[0].Cmp(inside[1]) > 0 {
		return r, ErrConvergence
	}

	type part struct{ a, z, width *big.Rat }
	pending := []part{{inside[0], inside[1], new(big.Rat).Quo(width, big.NewRat(2, 1))}}
	for len(pending) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		s := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if s.a.Cmp(s.z) == 0 {
			continue
		}
		area, proven, e := rationalGreenSegment(p, den, s.a, s.z, b)
		if e != nil {
			return r, e
		}
		if proven && new(big.Rat).Sub(area.hi, area.lo).Cmp(s.width) <= 0 {
			lower.Add(lower, area.lo)
			upper.Add(upper, area.hi)
			for _, v := range []*big.Rat{lower, upper} {
				if e := b.check(v); e != nil {
					return r, e
				}
			}
			continue
		}
		r.Subdivisions++
		if r.Subdivisions > o.MaxSubdivisions {
			return r, ErrConvergence
		}
		m := new(big.Rat).Quo(new(big.Rat).Add(s.a, s.z), big.NewRat(2, 1))
		nextWidth := new(big.Rat).Quo(s.width, big.NewRat(2, 1))
		pending = append(pending, part{m, s.z, nextWidth}, part{s.a, m, nextWidth})
	}
	if new(big.Rat).Sub(upper, lower).Cmp(width) > 0 {
		return r, ErrConvergence
	}
	r.Interval = ExactParameterInterval{Lower: lower, Upper: upper}
	return r, nil
}

// rationalGreenSegment subtracts a cubic in the exact centred coordinate
// s∈[-1,1]. The remainder is exactly s⁴ R(s)/D(s), so bounding R/D
// and integrating the nonnegative s⁴ gives a rigorous quadrature interval.
func rationalGreenSegment(p, den rationalPolynomial, a, z *big.Rat, b *polynomialBudget) (exactInterval, bool, error) {
	mid := new(big.Rat).Quo(new(big.Rat).Add(a, z), big.NewRat(2, 1))
	half := new(big.Rat).Sub(mid, a)
	t := rationalPolynomial{mid, half}
	n, e := composeProjectionPolynomial(p, t, b)
	if e != nil {
		return exactInterval{}, false, e
	}
	d, e := composeProjectionPolynomial(den, t, b)
	if e != nil {
		return exactInterval{}, false, e
	}
	domain := exactInterval{lo: big.NewRat(-1, 1), hi: big.NewRat(1, 1)}
	db, e := polynomialBernsteinInterval(d, domain, b)
	if e != nil {
		return exactInterval{}, false, e
	}
	if db.lo.Sign() <= 0 {
		return exactInterval{}, false, nil
	}
	coef := make(rationalPolynomial, 4)
	for i := range coef {
		coef[i] = new(big.Rat)
		if i < len(n) {
			coef[i].Set(n[i])
		}
		for j := 1; j <= i && j < len(d); j++ {
			coef[i].Sub(coef[i], new(big.Rat).Mul(d[j], coef[i-j]))
		}
		coef[i].Quo(coef[i], d[0])
		if e := b.check(coef[i]); e != nil {
			return exactInterval{}, false, e
		}
	}
	dc, e := polynomialProduct(d, coef, b)
	if e != nil {
		return exactInterval{}, false, e
	}
	residual, e := subtractScaledPolynomial(n, dc, big.NewRat(1, 1), b)
	if e != nil {
		return exactInterval{}, false, e
	}
	for _, c := range residual[:min(len(residual), 4)] {
		if c.Sign() != 0 {
			return exactInterval{}, false, ErrConvergence
		}
	}
	area := new(big.Rat).Mul(half, new(big.Rat).Add(new(big.Rat).Mul(coef[0], big.NewRat(2, 1)), new(big.Rat).Mul(coef[2], big.NewRat(2, 3))))
	if len(residual) <= 4 {
		r, e := boundedExactInterval(area, area, b)
		return r, e == nil, e
	}
	nr, e := polynomialBernsteinInterval(residual[4:], domain, b)
	if e != nil {
		return exactInterval{}, false, e
	}
	ratio, e := intervalDivide(nr, db, b)
	if e != nil {
		return exactInterval{}, false, e
	}
	scale := new(big.Rat).Mul(half, big.NewRat(2, 5))
	r, e := boundedExactInterval(new(big.Rat).Add(area, new(big.Rat).Mul(scale, ratio.lo)),
		new(big.Rat).Add(area, new(big.Rat).Mul(scale, ratio.hi)), b)
	return r, e == nil, e
}
