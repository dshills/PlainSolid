package geom

import "math/big"

func checkedStationRatio(r *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	if r == nil || r.Cmp(big.NewRat(1, 1)) == 0 {
		return nil, nil
	}
	if r.Sign() <= 0 {
		return nil, ErrInvalid
	}
	return r, b.check(r)
}

// pullProjectiveInterval inverts the original B cut pair and returns the
// fractional ratio local to the pulled rectangle. Inputs lie in [0,1].
func pullProjectiveInterval(y exactInterval, r *big.Rat, b *polynomialBudget) (exactInterval, *big.Rat, error) {
	a := boundedRatArithmetic{b: b}
	one := big.NewRat(1, 1)
	k := a.sub(r, one)
	lo := a.quo(y.lo, a.sub(r, a.mul(k, y.lo)))
	hi := a.quo(y.hi, a.sub(r, a.mul(k, y.hi)))
	local := a.quo(a.add(one, a.mul(k, hi)), a.add(one, a.mul(k, lo)))
	return exactInterval{lo, hi}, local, a.err
}

func scaleProjectiveBezier(net []exactHomogeneous, ratio *big.Rat, b *polynomialBudget) ([]exactHomogeneous, error) {
	a := boundedRatArithmetic{b: b}
	current := big.NewRat(1, 1)
	result := make([]exactHomogeneous, len(net))
	for i, h := range net {
		if i > 0 {
			current = a.mul(current, ratio)
		}
		for j, x := range h {
			result[i][j] = a.mul(x, current)
		}
		if a.err != nil {
			return nil, a.err
		}
	}
	return result, nil
}
