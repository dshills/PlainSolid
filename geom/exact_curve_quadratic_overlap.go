package geom

import (
	"context"
	"math/big"
)

func certifyQuadraticArcOverlap(ctx context.Context, curve *ExactSplineCurve, span ParameterSpan, linear [3][4]*big.Rat, weights [3]*big.Rat, arcLo, arcHi float64, b *polynomialBudget) (r ExactQuadraticArcOverlap, proven bool, err error) {
	if e := ctx.Err(); e != nil {
		return r, false, e
	}
	p, e := trimPatchBudget(curve.patch, ParameterRectangle{span.Start, span.End, 0, 1}, b)
	if e != nil {
		return r, false, e
	}
	var lambda [3][]*big.Rat
	for k := 0; k < 3; k++ {
		lambda[k] = make([]*big.Rat, p.countU)
		for i := 0; i < p.countU; i++ {
			h := p.net[i*p.countV]
			x := new(big.Rat)
			for axis := 0; axis < 4; axis++ {
				x.Add(x, new(big.Rat).Mul(linear[k][axis], h[axis]))
				if e := b.check(x); e != nil {
					return r, false, e
				}
			}
			if x.Sign() < 0 {
				return r, false, nil
			}
			lambda[k][i] = x
		}
	}
	// Nonnegative Bernstein coefficients prove closed-triangle membership.
	// N/D is the normalized inverse arc parameter; D is positive throughout
	// the open span unless it vanishes identically (the constant last endpoint).
	numerator, denominator := make([]*big.Rat, p.countU), make([]*big.Rat, p.countU)
	positive := false
	for i := 0; i < p.countU; i++ {
		numerator[i] = new(big.Rat).Mul(weights[0], lambda[1][i])
		denominator[i] = new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(2, 1), weights[1]), lambda[0][i]), numerator[i])
		positive = positive || denominator[i].Sign() > 0
		if e := b.check(denominator[i]); e != nil {
			return r, false, e
		}
	}
	if !positive {
		return r, false, nil
	}
	n, e := bernsteinToPower(numerator, b)
	if e != nil {
		return r, false, e
	}
	d, e := bernsteinToPower(denominator, b)
	if e != nil {
		return r, false, e
	}
	values := [2]*big.Rat{}
	for i, t := range []*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)} {
		nv, e := polynomialValue(n, t, b)
		if e != nil {
			return r, false, e
		}
		dv, e := polynomialValue(d, t, b)
		if e != nil {
			return r, false, e
		}
		if dv.Sign() <= 0 {
			return r, false, nil
		}
		values[i] = new(big.Rat).Quo(nv, dv)
		if e := b.check(values[i]); e != nil {
			return r, false, e
		}
	}
	slope := new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Sub(values[1], values[0]))
	offset := new(big.Rat).Sub(values[0], new(big.Rat).Quo(slope, big.NewRat(3, 1)))
	// The samples only propose coefficients. This identity proves the map.
	expected, e := polynomialProduct(rationalPolynomial{offset, slope}, d, b)
	if e != nil {
		return r, false, e
	}
	difference, e := sharedProjectionSubtract(n, expected, b)
	if e != nil {
		return r, false, e
	}
	if len(difference) != 0 || slope.Sign() == 0 {
		return r, false, nil
	}
	end := new(big.Rat).Add(offset, slope)
	one := big.NewRat(1, 1)
	if offset.Sign() < 0 || offset.Cmp(one) > 0 || end.Sign() < 0 || end.Cmp(one) > 0 {
		return r, false, nil
	}
	// On the open span the conic equation, triangle membership and inverse
	// identify the unique Bézier point. Positive rational supports are continuous,
	// so identity extends to both endpoints even if D vanishes there.
	arcOrigin := new(big.Rat).SetFloat64(arcLo)
	arcWidth := new(big.Rat).Sub(new(big.Rat).SetFloat64(arcHi), arcOrigin)
	r.CurveSpan = span
	r.ArcStart = new(big.Rat).Add(arcOrigin, new(big.Rat).Mul(arcWidth, offset))
	r.ArcEnd = new(big.Rat).Add(arcOrigin, new(big.Rat).Mul(arcWidth, end))
	r.ArcScale = new(big.Rat).Quo(new(big.Rat).Sub(r.ArcEnd, r.ArcStart), new(big.Rat).Sub(new(big.Rat).SetFloat64(span.End), new(big.Rat).SetFloat64(span.Start)))
	r.ArcOffset = new(big.Rat).Sub(r.ArcStart, new(big.Rat).Mul(r.ArcScale, new(big.Rat).SetFloat64(span.Start)))
	for _, x := range []*big.Rat{r.ArcStart, r.ArcEnd, r.ArcScale, r.ArcOffset} {
		if e := b.check(x); e != nil {
			return r, false, e
		}
	}
	return r, true, nil
}
