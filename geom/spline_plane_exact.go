package geom

import (
	"math/big"
)

// Exact polynomial machinery for spline-plane sections. Coefficients ascend in
// power. Operations are bounded by an explicit arithmetic/bit-size budget.
type rationalPolynomial []*big.Rat
type polynomialBudget struct{ operations, maxOperations, maxBits int }

func (b *polynomialBudget) check(v *big.Rat) error {
	b.operations++
	if b.operations > b.maxOperations || v.Num().BitLen() > b.maxBits || v.Denom().BitLen() > b.maxBits {
		return ErrConvergence
	}
	return nil
}
func trimPolynomial(p rationalPolynomial) rationalPolynomial {
	for len(p) > 0 && p[len(p)-1].Sign() == 0 {
		p = p[:len(p)-1]
	}
	return p
}
func clonePolynomial(p rationalPolynomial) rationalPolynomial {
	q := make(rationalPolynomial, len(p))
	for i, v := range p {
		q[i] = new(big.Rat).Set(v)
	}
	return q
}
func polynomialDerivative(p rationalPolynomial) rationalPolynomial {
	if len(p) < 2 {
		return nil
	}
	d := make(rationalPolynomial, len(p)-1)
	for i := 1; i < len(p); i++ {
		d[i-1] = new(big.Rat).Mul(p[i], big.NewRat(int64(i), 1))
	}
	return trimPolynomial(d)
}
func polynomialValue(p rationalPolynomial, x *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	v := new(big.Rat)
	for i := len(p) - 1; i >= 0; i-- {
		v.Mul(v, x)
		v.Add(v, p[i])
		if e := b.check(v); e != nil {
			return nil, e
		}
	}
	return v, nil
}
func polynomialDivide(a, divisor rationalPolynomial, b *polynomialBudget) (rationalPolynomial, rationalPolynomial, error) {
	if len(divisor) == 0 {
		return nil, nil, ErrDegenerate
	}
	r := clonePolynomial(a)
	q := make(rationalPolynomial, max(0, len(a)-len(divisor)+1))
	for i := range q {
		q[i] = new(big.Rat)
	}
	for len(r) >= len(divisor) {
		index := len(r) - len(divisor)
		factor := new(big.Rat).Quo(r[len(r)-1], divisor[len(divisor)-1])
		if e := b.check(factor); e != nil {
			return nil, nil, e
		}
		q[index].Set(factor)
		for j, v := range divisor {
			r[index+j].Sub(r[index+j], new(big.Rat).Mul(factor, v))
			if e := b.check(r[index+j]); e != nil {
				return nil, nil, e
			}
		}
		r = trimPolynomial(r)
	}
	return trimPolynomial(q), r, nil
}
func polynomialMonic(p rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	if len(p) == 0 {
		return nil, nil
	}
	q := clonePolynomial(p)
	leading := new(big.Rat).Set(q[len(q)-1])
	for _, v := range q {
		v.Quo(v, leading)
		if e := b.check(v); e != nil {
			return nil, e
		}
	}
	return q, nil
}
func polynomialGCD(a, c rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	a = clonePolynomial(a)
	c = clonePolynomial(c)
	for len(c) > 0 {
		_, r, e := polynomialDivide(a, c, b)
		if e != nil {
			return nil, e
		}
		r, e = polynomialMonic(r, b)
		if e != nil {
			return nil, e
		}
		a, c = c, r
	}
	return polynomialMonic(a, b)
}
func polynomialSturm(p rationalPolynomial, b *polynomialBudget) ([]rationalPolynomial, error) {
	if len(p) < 2 {
		return nil, nil
	}
	sequence := []rationalPolynomial{clonePolynomial(p), polynomialDerivative(p)}
	for len(sequence[len(sequence)-1]) > 1 {
		_, remainder, e := polynomialDivide(sequence[len(sequence)-2], sequence[len(sequence)-1], b)
		if e != nil {
			return nil, e
		}
		if len(remainder) == 0 {
			break
		}
		for _, v := range remainder {
			v.Neg(v)
		} // Positive scaling preserves Sturm signs and limits coefficient growth.
		scale := new(big.Rat).Abs(remainder[len(remainder)-1])
		for _, v := range remainder {
			v.Quo(v, scale)
			if e = b.check(v); e != nil {
				return nil, e
			}
		}
		sequence = append(sequence, remainder)
	}
	return sequence, nil
}
func sturmVariations(sequence []rationalPolynomial, x *big.Rat, b *polynomialBudget) (int, error) {
	previous, variations := 0, 0
	for _, p := range sequence {
		v, e := polynomialValue(p, x, b)
		if e != nil {
			return 0, e
		}
		sign := v.Sign()
		if sign != 0 {
			if previous != 0 && sign != previous {
				variations++
			}
			previous = sign
		}
	}
	return variations, nil
}
func sturmCount(sequence []rationalPolynomial, a, c *big.Rat, b *polynomialBudget) (int, error) {
	left, e := sturmVariations(sequence, a, b)
	if e != nil {
		return 0, e
	}
	right, e := sturmVariations(sequence, c, b)
	return left - right, e
}
func bernsteinToPower(coefficients []*big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	n := len(coefficients) - 1
	p := make(rationalPolynomial, n+1)
	for k := 0; k <= n; k++ {
		p[k] = new(big.Rat)
		for i := 0; i <= k; i++ {
			factor := new(big.Rat)
			if n < len(smallBernsteinPowerFactors) {
				factor.SetInt64(smallBernsteinPowerFactors[n][k][i])
			} else {
				factor.SetInt(new(big.Int).Mul(new(big.Int).Binomial(int64(n), int64(i)), new(big.Int).Binomial(int64(n-i), int64(k-i))))
			}
			term := new(big.Rat).Mul(coefficients[i], factor)
			if (k-i)%2 == 1 {
				term.Neg(term)
			}
			p[k].Add(p[k], term)
			if e := b.check(p[k]); e != nil {
				return nil, e
			}
		}
	}
	return trimPolynomial(p), nil
}
func rootMultiplicity(p rationalPolynomial, x *big.Rat, b *polynomialBudget) (int, error) {
	multiplicity := 0
	for len(p) > 0 {
		v, e := polynomialValue(p, x, b)
		if e != nil {
			return 0, e
		}
		if v.Sign() != 0 {
			break
		}
		multiplicity++
		p = polynomialDerivative(p)
	}
	return multiplicity, nil
}

// exactPlaneBezierSpans refines the scalar homogeneous numerator without
// rounding control geometry. Each block of degree+1 coefficients is one span.
func exactPlaneBezierSpans(curve *BSpline, plane Plane, b *polynomialBudget) ([]float64, []*big.Rat, error) {
	coefficients := make([]*big.Rat, len(curve.poles))
	for i, p := range curve.poles {
		v := new(big.Rat)
		for j := 0; j < 3; j++ {
			delta := new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(p, j)), new(big.Rat).SetFloat64(coordinate(plane.origin, j)))
			term := new(big.Rat).Mul(delta, new(big.Rat).SetFloat64(coordinate(plane.normal, j)))
			v.Add(v, term)
		}
		v.Mul(v, new(big.Rat).SetFloat64(curve.weights[i]))
		if e := b.check(v); e != nil {
			return nil, nil, e
		}
		coefficients[i] = v
	}
	knots := append([]float64(nil), curve.knots...)
	start, end := curve.Domain()
	breaks := []float64{start}
	for _, t := range knots {
		if t > breaks[len(breaks)-1] && t < end {
			breaks = append(breaks, t)
		}
	}
	breaks = append(breaks, end)
	for _, t := range breaks[1 : len(breaks)-1] {
		for {
			first, last := -1, -1
			for i, k := range knots {
				if k == t {
					if first < 0 {
						first = i
					}
					last = i
				}
			}
			multiplicity := last - first + 1
			if multiplicity == curve.degree+1 {
				break
			}
			k := last
			s := multiplicity
			next := make([]*big.Rat, len(coefficients)+1)
			for i := 0; i <= k-curve.degree; i++ {
				next[i] = new(big.Rat).Set(coefficients[i])
			}
			for i := k - s; i < len(coefficients); i++ {
				next[i+1] = new(big.Rat).Set(coefficients[i])
			}
			for i := k - curve.degree + 1; i <= k-s; i++ {
				numerator := new(big.Rat).Sub(new(big.Rat).SetFloat64(t), new(big.Rat).SetFloat64(knots[i]))
				denominator := new(big.Rat).Sub(new(big.Rat).SetFloat64(knots[i+curve.degree]), new(big.Rat).SetFloat64(knots[i]))
				alpha := new(big.Rat).Quo(numerator, denominator)
				v := new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 1), alpha), coefficients[i-1])
				v.Add(v, new(big.Rat).Mul(alpha, coefficients[i]))
				if e := b.check(v); e != nil {
					return nil, nil, e
				}
				next[i] = v
			}
			newKnots := make([]float64, len(knots)+1)
			copy(newKnots, knots[:k+1])
			newKnots[k+1] = t
			copy(newKnots[k+2:], knots[k+1:])
			knots, coefficients = newKnots, next
		}
	}
	return breaks, coefficients, nil
}
