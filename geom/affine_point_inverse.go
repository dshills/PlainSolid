package geom

import (
	"context"
	"math/big"
)

// InvertRationalPoint solves m(local)=point using the exact coefficients of
// the represented affine transform. Singular transforms return ErrDegenerate.
func (m Affine) InvertRationalPoint(ctx context.Context, point [3]*big.Rat, o AffineChartInverseOptions) ([3]*big.Rat, error) {
	var out [3]*big.Rat
	if ctx == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 100000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	var matrix [3][3]*big.Rat
	var delta [3]*big.Rat
	for i, row := range m {
		if point[i] == nil || !finite(row[3]) {
			return out, ErrInvalid
		}
		if e := b.check(point[i]); e != nil {
			return out, e
		}
		delta[i] = new(big.Rat).Sub(point[i], new(big.Rat).SetFloat64(row[3]))
		if e := b.check(delta[i]); e != nil {
			return out, e
		}
		for j := range matrix[i] {
			if !finite(row[j]) {
				return out, ErrInvalid
			}
			matrix[i][j] = new(big.Rat).SetFloat64(row[j])
			if e := b.check(matrix[i][j]); e != nil {
				return out, e
			}
		}
	}
	inverse, regular, e := inverseExact3(matrix, b)
	if e != nil {
		return out, e
	}
	if !regular {
		return out, ErrDegenerate
	}
	var result [3]*big.Rat
	for i, row := range inverse {
		sum := new(big.Rat)
		for j, value := range row {
			sum.Add(sum, new(big.Rat).Mul(value, delta[j]))
			if e := b.check(sum); e != nil {
				return out, e
			}
		}
		result[i] = sum
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	return result, nil
}
