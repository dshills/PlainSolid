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
	if n < 1 || n > 4 {
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
	result := make(rationalPolynomial, n+1)
	for i := range result {
		result[i] = new(big.Rat)
	}
	permutation := make([]int, n)
	used := make([]bool, n)
	var visit func(int) error
	visit = func(row int) error {
		if row == n {
			term := rationalPolynomial{big.NewRat(1, 1)}
			inversions := 0
			for i, col := range permutation {
				for j := 0; j < i; j++ {
					if permutation[j] > col {
						inversions++
					}
				}
				factor := rationalPolynomial{new(big.Rat).Neg(multiplication[i][col])}
				if i == col {
					factor = append(factor, big.NewRat(1, 1))
				}
				var e error
				term, e = polynomialProduct(term, factor, b)
				if e != nil {
					return e
				}
			}
			for i, x := range term {
				if inversions%2 == 0 {
					result[i].Add(result[i], x)
				} else {
					result[i].Sub(result[i], x)
				}
				if e := b.check(result[i]); e != nil {
					return e
				}
			}
			return nil
		}
		for col := 0; col < n; col++ {
			if used[col] {
				continue
			}
			used[col] = true
			permutation[row] = col
			if e := visit(row + 1); e != nil {
				return e
			}
			used[col] = false
		}
		return nil
	}
	if e := visit(0); e != nil {
		return nil, e
	}
	gcd, e := polynomialGCD(result, polynomialDerivative(result), b)
	if e != nil {
		return nil, e
	}
	result, _, e = polynomialDivide(result, gcd, b)
	return result, e
}

func coordinateRootCharacteristic(ctx context.Context, source *ExactGraphLineRoot, p, n, w rationalPolynomial, b *polynomialBudget, maxSubdivisions int) (*ExactGraphLineRoot, error) {
	characteristic, e := rationalCoordinateCharacteristic(p, n, w, b)
	if e != nil {
		return nil, e
	}
	bounds := [2]*big.Rat{new(big.Rat).Set(source.bounds[0]), new(big.Rat).Set(source.bounds[1])}
	steps := 0
	for {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if steps >= maxSubdivisions {
			return nil, ErrConvergence
		}
		interval := exactInterval{lo: bounds[0], hi: bounds[1]}
		denominator, e := polynomialInterval(w, interval, b)
		if e != nil {
			return nil, e
		}
		if denominator.lo.Sign() > 0 || denominator.hi.Sign() < 0 {
			numerator, e := polynomialInterval(n, interval, b)
			if e != nil {
				return nil, e
			}
			image, e := intervalDivide(numerator, denominator, b)
			if e != nil {
				return nil, e
			}
			count := 0
			used, e := isolatePolynomialRoots(characteristic, image.lo, image.hi, b, maxSubdivisions-steps, func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, ctx.Err() })
			steps += used
			if e != nil {
				return nil, e
			}
			if count == 1 {
				return &ExactGraphLineRoot{polynomial: characteristic, bounds: [2]*big.Rat{image.lo, image.hi}}, nil
			}
			if count == 0 {
				return nil, ErrConvergence
			} // contradicts exact image identity
		}
		if bounds[0].Cmp(bounds[1]) == 0 {
			return nil, ErrConvergence
		}
		if steps >= maxSubdivisions {
			return nil, ErrConvergence
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
			return nil, e
		}
		if next[0] == nil {
			return nil, ErrConvergence
		}
		bounds = next
	}
}
