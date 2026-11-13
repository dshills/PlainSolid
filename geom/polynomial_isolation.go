package geom

import "math/big"

func polynomialProduct(a, c rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	if len(a) == 0 || len(c) == 0 {
		return nil, nil
	}
	result := make(rationalPolynomial, len(a)+len(c)-1)
	for i := range result {
		result[i] = new(big.Rat)
	}
	for i, x := range a {
		for j, y := range c {
			result[i+j].Add(result[i+j], new(big.Rat).Mul(x, y))
			if e := b.check(result[i+j]); e != nil {
				return nil, e
			}
		}
	}
	return trimPolynomial(result), nil
}

// polynomialRootBound gives the exact Cauchy bound 1+max |a_i/a_n|.
func polynomialRootBound(p rationalPolynomial, b *polynomialBudget) (*big.Rat, error) {
	if len(p) < 2 {
		return nil, ErrInvalid
	}
	maximum := new(big.Rat)
	for _, coefficient := range p[:len(p)-1] {
		ratio := new(big.Rat).Abs(new(big.Rat).Quo(coefficient, p[len(p)-1]))
		if e := b.check(ratio); e != nil {
			return nil, e
		}
		if ratio.Cmp(maximum) > 0 {
			maximum.Set(ratio)
		}
	}
	return maximum.Add(maximum, big.NewRat(1, 1)), nil
}

// isolatePolynomialRoots visits every distinct real root on a closed rational
// interval. accept decides whether its exact bracket is sufficiently refined.
// Multiplicities are exact; no sampled sign test can discard even-order roots.
func isolatePolynomialRoots(polynomial rationalPolynomial, lower, upper *big.Rat, budget *polynomialBudget, maxSubdivisions int, accept func(*big.Rat, *big.Rat, int) (bool, error)) (int, error) {
	if len(polynomial) == 0 {
		return 0, ErrDegenerate
	}
	if lower.Cmp(upper) > 0 {
		return 0, nil
	}
	if lower.Cmp(upper) == 0 {
		multiplicity, e := rootMultiplicity(polynomial, lower, budget)
		if e != nil {
			return 0, e
		}
		if multiplicity > 0 {
			ok, e := accept(lower, upper, multiplicity)
			if e != nil {
				return 0, e
			}
			if !ok {
				return 0, ErrConvergence
			}
		}
		return 0, nil
	}
	interior := clonePolynomial(polynomial)
	for _, endpoint := range []*big.Rat{lower, upper} {
		multiplicity, e := rootMultiplicity(interior, endpoint, budget)
		if e != nil {
			return 0, e
		}
		if multiplicity > 0 {
			ok, e := accept(endpoint, endpoint, multiplicity)
			if e != nil {
				return 0, e
			}
			if !ok {
				return 0, ErrConvergence
			}
			factor := rationalPolynomial{new(big.Rat).Neg(endpoint), big.NewRat(1, 1)}
			for i := 0; i < multiplicity; i++ {
				interior, _, e = polynomialDivide(interior, factor, budget)
				if e != nil {
					return 0, e
				}
			}
		}
	}
	if len(interior) < 2 {
		return 0, nil
	}
	gcd, e := polynomialGCD(interior, polynomialDerivative(interior), budget)
	if e != nil {
		return 0, e
	}
	squareFree, _, e := polynomialDivide(interior, gcd, budget)
	if e != nil {
		return 0, e
	}
	sturm, e := polynomialSturm(squareFree, budget)
	if e != nil {
		return 0, e
	}
	multiplicitySturm := [][]rationalPolynomial{}
	for len(gcd) > 1 {
		sequence, e := polynomialSturm(gcd, budget)
		if e != nil {
			return 0, e
		}
		multiplicitySturm = append(multiplicitySturm, sequence)
		gcd, e = polynomialGCD(gcd, polynomialDerivative(gcd), budget)
		if e != nil {
			return 0, e
		}
	}
	type interval struct {
		lower, upper *big.Rat
		count        int
	}
	count, e := sturmCount(sturm, lower, upper, budget)
	if e != nil {
		return 0, e
	}
	pending := []interval{{new(big.Rat).Set(lower), new(big.Rat).Set(upper), count}}
	subdivisions := 0
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current.count == 0 {
			continue
		}
		if current.count == 1 {
			left, right := current.lower, current.upper
			atRight, e := polynomialValue(squareFree, right, budget)
			if e != nil {
				return subdivisions, e
			}
			if atRight.Sign() == 0 {
				left = right
			}
			multiplicity := 1
			if left.Cmp(right) == 0 {
				multiplicity, e = rootMultiplicity(polynomial, left, budget)
				if e != nil {
					return subdivisions, e
				}
			} else {
				for _, sequence := range multiplicitySturm {
					n, e := sturmCount(sequence, left, right, budget)
					if e != nil {
						return subdivisions, e
					}
					if n > 0 {
						multiplicity++
					}
				}
			}
			ok, e := accept(left, right, multiplicity)
			if e != nil {
				return subdivisions, e
			}
			if ok {
				continue
			}
			if left.Cmp(right) == 0 {
				return subdivisions, ErrConvergence
			}
		}
		if subdivisions >= maxSubdivisions {
			return subdivisions, ErrConvergence
		}
		subdivisions++
		mid := new(big.Rat).Quo(new(big.Rat).Add(current.lower, current.upper), big.NewRat(2, 1))
		leftCount, e := sturmCount(sturm, current.lower, mid, budget)
		if e != nil {
			return subdivisions, e
		}
		pending = append(pending, interval{mid, current.upper, current.count - leftCount}, interval{current.lower, mid, leftCount})
	}
	return subdivisions, nil
}
