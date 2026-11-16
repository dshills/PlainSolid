package geom

import "math/big"

type unreducedFraction struct {
	numerator, denominator big.Int
}

// exactFractionRange holds the extrema of checked ratios with positive
// denominators. Each ratio uses the original work charge. A ratio near its
// coefficient limit is reduced before checking so cancellation is honored.
type exactFractionRange struct {
	minimum, maximum unreducedFraction
	left, right      big.Int
	normalized       big.Rat
}

func (r *exactFractionRange) extend(n, d *big.Int, budget *polynomialBudget) error {
	if n.BitLen() > budget.maxBits || d.BitLen() > budget.maxBits {
		value := r.normalized.SetFrac(n, d)
		n, d = value.Num(), value.Denom()
	}
	if err := budget.checkBits(n.BitLen(), d.BitLen()); err != nil {
		return err
	}
	lo, hi := &r.minimum, &r.maximum
	if lo.denominator.Sign() == 0 || r.compare(n, d, lo) < 0 {
		lo.numerator.Set(n)
		lo.denominator.Set(d)
	}
	if hi.denominator.Sign() == 0 || r.compare(n, d, hi) > 0 {
		hi.numerator.Set(n)
		hi.denominator.Set(d)
	}
	return nil
}

func (r *exactFractionRange) compare(n, d *big.Int, q *unreducedFraction) int {
	if n.Sign() != q.numerator.Sign() {
		return n.Sign() - q.numerator.Sign()
	}
	if d.Cmp(&q.denominator) == 0 {
		return n.Cmp(&q.numerator)
	}
	r.left.Mul(n, &q.denominator)
	r.right.Mul(&q.numerator, d)
	return r.left.Cmp(&r.right)
}

func (r *exactFractionRange) interval() exactInterval {
	return exactInterval{
		new(big.Rat).SetFrac(&r.minimum.numerator, &r.minimum.denominator),
		new(big.Rat).SetFrac(&r.maximum.numerator, &r.maximum.denominator),
	}
}
