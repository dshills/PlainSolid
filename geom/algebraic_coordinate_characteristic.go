package geom

import (
	"context"
	"math/big"
)

func quotientMultiplicationMatrix(poly, modulus rationalPolynomial, b *polynomialBudget) ([][]*big.Rat, error) {
	n := len(modulus) - 1
	m := make([][]*big.Rat, n)
	for i := range m {
		m[i] = make([]*big.Rat, n)
		for j := range m[i] {
			m[i][j] = new(big.Rat)
		}
	}
	for j := 0; j < n; j++ {
		shifted := make(rationalPolynomial, j+len(poly))
		for i := 0; i < j; i++ {
			shifted[i] = new(big.Rat)
		}
		copy(shifted[j:], poly)
		_, rem, e := polynomialDivide(shifted, modulus, b)
		if e != nil {
			return nil, e
		}
		for i, x := range rem {
			m[i][j].Set(x)
		}
	}
	return m, nil
}

// Solve multiplication-by-denominator in Q[t]/P, then obtain the characteristic
// polynomial of multiplication by its rational-function quotient. Cayley–
// Hamilton proves the selected coordinate is a root, including reducible P.
func rationalCoordinateCharacteristic(p, numerator, denominator rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	n := len(p) - 1
	if n < 1 || n > 8 {
		return nil, ErrConvergence
	}
	matrix, e := quotientMultiplicationMatrix(denominator, p, b)
	if e != nil {
		return nil, e
	}
	_, rhs, e := polynomialDivide(numerator, p, b)
	if e != nil {
		return nil, e
	}
	for i := 0; i < n; i++ {
		value := new(big.Rat)
		if i < len(rhs) {
			value.Set(rhs[i])
		}
		matrix[i] = append(matrix[i], value)
	}
	for k := 0; k < n; k++ {
		pivot := k
		for pivot < n && matrix[pivot][k].Sign() == 0 {
			pivot++
		}
		if pivot == n {
			return nil, ErrDegenerate
		}
		matrix[k], matrix[pivot] = matrix[pivot], matrix[k]
		divisor := new(big.Rat).Set(matrix[k][k])
		for j := k; j <= n; j++ {
			matrix[k][j].Quo(matrix[k][j], divisor)
			if e := b.check(matrix[k][j]); e != nil {
				return nil, e
			}
		}
		for i := 0; i < n; i++ {
			if i == k {
				continue
			}
			factor := new(big.Rat).Set(matrix[i][k])
			for j := k; j <= n; j++ {
				matrix[i][j].Sub(matrix[i][j], new(big.Rat).Mul(factor, matrix[k][j]))
				if e := b.check(matrix[i][j]); e != nil {
					return nil, e
				}
			}
		}
	}
	quotient := make(rationalPolynomial, n)
	for i := range quotient {
		quotient[i] = matrix[i][n]
	}
	multiplication, e := quotientMultiplicationMatrix(trimPolynomial(quotient), p, b)
	if e != nil {
		return nil, e
	}
	result, e := rationalMatrixCharacteristic(multiplication, b)
	if e != nil {
		return nil, e
	}
	gcd, e := polynomialGCD(result, polynomialDerivative(result), b)
	if e != nil {
		return nil, e
	}
	result, _, e = polynomialDivide(result, gcd, b)
	return result, e
}

// The Faddeev–LeVerrier recurrence computes det(xI-A) with exact
// rational matrix products and trace divisions. Its bounded work grows
// polynomially with matrix order.
func rationalMatrixCharacteristic(a [][]*big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	n := len(a)
	previous := make([][]*big.Rat, n)
	for i := range previous {
		previous[i] = make([]*big.Rat, n)
		for j := range previous[i] {
			previous[i][j] = new(big.Rat)
		}
		previous[i][i].SetInt64(1)
	}
	result := make(rationalPolynomial, n+1)
	result[n] = big.NewRat(1, 1)
	for k := 1; k <= n; k++ {
		product := make([][]*big.Rat, n)
		trace := new(big.Rat)
		for i := range product {
			product[i] = make([]*big.Rat, n)
			for j := range product[i] {
				sum := new(big.Rat)
				for l, x := range a[i] {
					if x.Sign() == 0 || previous[l][j].Sign() == 0 {
						continue
					}
					sum.Add(sum, new(big.Rat).Mul(x, previous[l][j]))
					if e := b.check(sum); e != nil {
						return nil, e
					}
				}
				product[i][j] = sum
			}
			trace.Add(trace, product[i][i])
			if e := b.check(trace); e != nil {
				return nil, e
			}
		}
		coefficient := new(big.Rat).Quo(trace, big.NewRat(-int64(k), 1))
		if e := b.check(coefficient); e != nil {
			return nil, e
		}
		result[n-k] = coefficient
		if k < n {
			for i := range product {
				product[i][i].Add(product[i][i], coefficient)
				if e := b.check(product[i][i]); e != nil {
					return nil, e
				}
			}
		}
		previous = product
	}
	return result, nil
}

func coordinateRootCharacteristic(ctx context.Context, source *ExactGraphLineRoot, p, n, w rationalPolynomial, b *polynomialBudget, maxSubdivisions int) (root *ExactGraphLineRoot, steps int, err error) {
	characteristic, e := rationalCoordinateCharacteristic(p, n, w, b)
	if e != nil {
		return nil, steps, e
	}
	bounds := [2]*big.Rat{new(big.Rat).Set(source.bounds[0]), new(big.Rat).Set(source.bounds[1])}
	for {
		if e := ctx.Err(); e != nil {
			return nil, steps, e
		}
		if steps >= maxSubdivisions {
			return nil, steps, ErrConvergence
		}
		interval := exactInterval{lo: bounds[0], hi: bounds[1]}
		denominator, e := polynomialInterval(w, interval, b)
		if e != nil {
			return nil, steps, e
		}
		if denominator.lo.Sign() > 0 || denominator.hi.Sign() < 0 {
			numerator, e := polynomialInterval(n, interval, b)
			if e != nil {
				return nil, steps, e
			}
			image, e := intervalDivide(numerator, denominator, b)
			if e != nil {
				return nil, steps, e
			}
			count := 0
			used, e := isolatePolynomialRoots(characteristic, image.lo, image.hi, b, maxSubdivisions-steps, func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, ctx.Err() })
			steps += used
			if e != nil {
				return nil, steps, e
			}
			if count == 1 {
				return &ExactGraphLineRoot{polynomial: characteristic, bounds: [2]*big.Rat{image.lo, image.hi}}, steps, nil
			}
			if count == 0 {
				return nil, steps, ErrConvergence
			} // contradicts exact image identity
		}
		if bounds[0].Cmp(bounds[1]) == 0 {
			return nil, steps, ErrConvergence
		}
		if steps >= maxSubdivisions {
			return nil, steps, ErrConvergence
		}
		target := new(big.Rat).Quo(new(big.Rat).Sub(bounds[1], bounds[0]), big.NewRat(2, 1))
		var next [2]*big.Rat
		used, e := isolatePolynomialRoots(p, bounds[0], bounds[1], b, maxSubdivisions-steps, func(lo, hi *big.Rat, _ int) (bool, error) {
			if new(big.Rat).Sub(hi, lo).Cmp(target) > 0 {
				return false, nil
			}
			next = [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}
			return true, nil
		})
		steps += used
		if e != nil {
			return nil, steps, e
		}
		if next[0] == nil {
			return nil, steps, ErrConvergence
		}
		bounds = next
	}
}
