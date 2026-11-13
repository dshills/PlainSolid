package geom

import "math/big"

// polynomialBernsteinInterval bounds the polynomial on a rational interval by
// the convex hull of its exact Bernstein coefficients after affine restriction.
// Unlike Horner interval evaluation it retains cancellation between global
// powers. This is particularly important for composed rational conic curves.
func polynomialBernsteinInterval(p rationalPolynomial, interval exactInterval, b *polynomialBudget) (exactInterval, error) {
	if len(p) == 0 {
		return exactInterval{lo: new(big.Rat), hi: new(big.Rat)}, nil
	}
	if interval.lo.Cmp(interval.hi) > 0 {
		return exactInterval{}, ErrDomain
	}
	width := new(big.Rat).Sub(interval.hi, interval.lo)
	local, e := composeProjectionPolynomial(p, rationalPolynomial{interval.lo, width}, b)
	if e != nil {
		return exactInterval{}, e
	}
	if len(local) == 0 {
		return exactInterval{lo: new(big.Rat), hi: new(big.Rat)}, nil
	}
	degree := len(local) - 1
	var lo, hi *big.Rat
	for i := 0; i <= degree; i++ {
		value := new(big.Rat)
		for j := 0; j <= i; j++ {
			ratio := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(degree), int64(j)))
			value.Add(value, new(big.Rat).Mul(local[j], ratio))
			if e := b.check(value); e != nil {
				return exactInterval{}, e
			}
		}
		if lo == nil || value.Cmp(lo) < 0 {
			lo = new(big.Rat).Set(value)
		}
		if hi == nil || value.Cmp(hi) > 0 {
			hi = new(big.Rat).Set(value)
		}
	}
	return exactInterval{lo: lo, hi: hi}, nil
}
