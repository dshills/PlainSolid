package geom

import (
	"math/big"
	"slices"
)

// factorCircleQuartic reconstructs two monic circles from their exact product.
// Coordinates are still ordered (free,dependent) in this proof. It returns
// distinct centers in lexicographic order and verifies the complete quotient.
func factorCircleQuartic(q [5]rationalPolynomial, b *polynomialBudget) (circles [2]GraphContactCircle, err error) {
	one, two, four := big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(4, 1)
	if len(q[0]) != 5 || len(q[1]) > 3 || len(q[2]) != 3 || len(q[3]) > 1 ||
		len(q[4]) != 1 || q[4][0].Cmp(one) != 0 || q[0][4].Cmp(one) != 0 || q[2][2].Cmp(two) != 0 {
		return circles, ErrConvergence
	}
	at := func(col, degree int) *big.Rat {
		if degree < len(q[col]) {
			return q[col][degree]
		}
		return new(big.Rat)
	}
	// Writing R=x²+y², the quartic is R²+R(L*x+M*y)+P₂.
	// The two cubics fix L and M. The missing constant C in the sum
	// of the affine terms is determined by the rank of their difference.
	l, m := q[0][3], at(3, 0)
	if l.Cmp(q[2][1]) != 0 || m.Cmp(at(1, 2)) != 0 {
		return circles, ErrConvergence
	}
	xx := new(big.Rat).Sub(new(big.Rat).Mul(l, l), new(big.Rat).Mul(four, q[0][2]))
	yy := new(big.Rat).Sub(new(big.Rat).Mul(m, m), new(big.Rat).Mul(four, q[2][0]))
	xy := new(big.Rat).Mul(two, new(big.Rat).Mul(l, m))
	xy.Sub(xy, new(big.Rat).Mul(four, at(1, 1)))

	// If the factors are circles, the quadratic part of their squared
	// affine difference has diagonal (xx+4*C, yy+4*C) and cross term xy.
	// Its positive rank-one condition uniquely chooses the larger shift.
	diff := new(big.Rat).Sub(xx, yy)
	radicand := new(big.Rat).Add(new(big.Rat).Mul(diff, diff), new(big.Rat).Mul(xy, xy))
	root, err := graphCircleSquareRoot(radicand, b)
	if err != nil {
		return circles, err
	}
	if root.Sign() == 0 {
		return circles, ErrConvergence
	}
	c := new(big.Rat).Sub(root, new(big.Rat).Add(xx, yy))
	c.Quo(c, big.NewRat(8, 1))
	if err := b.check(c); err != nil {
		return circles, err
	}
	dxSquared := new(big.Rat).Add(xx, new(big.Rat).Mul(four, c))
	dx, err := graphCircleSquareRoot(dxSquared, b)
	if err != nil {
		return circles, err
	}
	var dy, dc *big.Rat
	if dx.Sign() > 0 {
		den := new(big.Rat).Mul(two, dx)
		dy = new(big.Rat).Quo(xy, den)
		num := new(big.Rat).Mul(two, new(big.Rat).Mul(l, c))
		num.Sub(num, new(big.Rat).Mul(four, q[0][1]))
		dc = num.Quo(num, den)
	} else {
		dySquared := new(big.Rat).Add(yy, new(big.Rat).Mul(four, c))
		dy, err = graphCircleSquareRoot(dySquared, b)
		if err != nil {
			return circles, err
		}
		if dy.Sign() == 0 {
			return circles, ErrConvergence
		}
		den := new(big.Rat).Mul(two, dy)
		num := new(big.Rat).Mul(two, new(big.Rat).Mul(m, c))
		num.Sub(num, new(big.Rat).Mul(four, at(1, 0)))
		dc = num.Quo(num, den)
	}
	for _, coefficient := range []*big.Rat{dy, dc} {
		if err := b.check(coefficient); err != nil {
			return circles, err
		}
	}

	// Split the sum using the recovered affine difference, then multiply
	// the two complete candidate factors to authenticate all coefficients.
	var factors [2][3]rationalPolynomial
	for i, sign := range [2]int64{1, -1} {
		affine := func(sum, delta *big.Rat) *big.Rat {
			v := new(big.Rat).Add(sum, new(big.Rat).Mul(big.NewRat(sign, 1), delta))
			return v.Quo(v, two)
		}
		x, y, constant := affine(l, dx), affine(m, dy), affine(c, dc)
		factors[i] = [3]rationalPolynomial{{constant, x, one}, {y}, {one}}
		cx := new(big.Rat).Quo(new(big.Rat).Neg(x), two)
		cy := new(big.Rat).Quo(new(big.Rat).Neg(y), two)
		radiusSquared := new(big.Rat).Add(new(big.Rat).Mul(cx, cx), new(big.Rat).Mul(cy, cy))
		radiusSquared.Sub(radiusSquared, constant)
		if radiusSquared.Sign() <= 0 {
			return circles, ErrConvergence
		}
		for _, value := range []*big.Rat{cx, cy, radiusSquared} {
			if err := b.check(value); err != nil {
				return circles, err
			}
		}
		circles[i] = GraphContactCircle{Center: [2]*big.Rat{cx, cy}, RadiusSquared: radiusSquared}
	}
	var product [5]rationalPolynomial
	for i, a := range factors[0] {
		for j, c := range factors[1] {
			v, err := polynomialProduct(a, c, b)
			if err != nil {
				return circles, err
			}
			product[i+j], err = graphPolynomialSum(product[i+j], v, 1, b)
			if err != nil {
				return circles, err
			}
		}
	}
	for j, col := range product {
		if !slices.EqualFunc(col, q[j], func(a, c *big.Rat) bool { return a.Cmp(c) == 0 }) {
			return circles, ErrConvergence
		}
	}
	return circles, nil
}

func graphCircleSquareRoot(square *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	if err := b.check(square); err != nil {
		return nil, err
	}
	if square.Sign() == 0 {
		return new(big.Rat), nil
	}
	root, err := graphExactPolynomialSqrt(rationalPolynomial{square}, b)
	if err != nil {
		return nil, err
	}
	return root[0], nil
}
