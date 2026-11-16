package geom

import "math/big"

// separatedParabolaCircleRegion binds the entire disk to the original lower
// or upper open region, numbered 0 and 1. A strict global ordinate hull is
// sufficient. Otherwise a local Sturm count checks the complete circle
// polynomial after substitution of the original parabola.
func separatedParabolaCircleRegion(p rationalPolynomial, minimum, maximum *big.Rat, center [2]*big.Rat,
	radiusSquared *big.Rat, b *polynomialBudget) (int, error) {
	one := big.NewRat(1, 1)
	x, y := center[0], center[1]
	right := new(big.Rat).Sub(one, x)
	for _, distance := range [4]*big.Rat{x, right, y, new(big.Rat).Sub(one, y)} {
		square := new(big.Rat).Mul(distance, distance)
		if err := b.check(square); err != nil {
			return 0, err
		}
		if distance.Sign() <= 0 || square.Cmp(radiusSquared) <= 0 {
			return 0, ErrConvergence
		}
	}
	gap := new(big.Rat).Sub(minimum, y)
	region := 0
	if y.Cmp(maximum) > 0 {
		gap.Sub(y, maximum)
		region = 1
	}
	square := new(big.Rat).Mul(gap, gap)
	if err := b.check(square); err != nil {
		return 0, err
	}
	if gap.Sign() > 0 && square.Cmp(radiusSquared) > 0 {
		return region, nil
	}

	// d is strictly greater than the radius by containment. The rational
	// bound (d + r²/d)/2 lies strictly between r and d, including for an
	// irrational radius. Outside [x-bound,x+bound] the horizontal distance
	// alone rules out an intersection.
	d := x
	if right.Cmp(x) < 0 {
		d = right
	}
	bound := new(big.Rat).Quo(radiusSquared, d)
	bound.Add(bound, d).Quo(bound, big.NewRat(2, 1))
	lo, hi := new(big.Rat).Sub(x, bound), new(big.Rat).Add(x, bound)
	for _, value := range [3]*big.Rat{bound, lo, hi} {
		if err := b.check(value); err != nil {
			return 0, err
		}
	}

	offset := clonePolynomial(p)
	offset[0].Sub(offset[0], y)
	if err := b.check(offset[0]); err != nil {
		return 0, err
	}
	vertical, err := polynomialProduct(offset, offset, b)
	if err != nil {
		return 0, err
	}
	horizontal := rationalPolynomial{
		new(big.Rat).Sub(new(big.Rat).Mul(x, x), radiusSquared),
		new(big.Rat).Mul(big.NewRat(-2, 1), x),
		one,
	}
	quartic, err := graphPolynomialSum(vertical, horizontal, 1, b)
	if err != nil {
		return 0, err
	}
	for _, end := range [2]*big.Rat{lo, hi} {
		value, err := polynomialValue(quartic, end, b)
		if err != nil {
			return 0, err
		}
		if value.Sign() <= 0 {
			return 0, ErrConvergence
		}
	}
	sturm, err := polynomialSturm(quartic, b)
	if err != nil {
		return 0, err
	}
	count, err := sturmCount(sturm, lo, hi, b)
	if err != nil {
		return 0, err
	}
	if count != 0 {
		return 0, ErrConvergence
	}

	// Positivity everywhere excludes the closed disk from the parabola.
	// The connected disk therefore has the same side as its exact center.
	centerGap, err := polynomialValue(offset, x, b)
	if err != nil {
		return 0, err
	}
	switch centerGap.Sign() {
	case 1:
		return 0, nil
	case -1:
		return 1, nil
	default:
		return 0, ErrConvergence
	}
}
