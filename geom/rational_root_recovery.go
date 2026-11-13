package geom

import "math/big"

// intervalRationalConvergents proposes convergents of the exact interval
// midpoint, restricted to the open interval. Candidate generation is bounded by
// the same operation/coefficient limits as isolation. It is deliberately not a
// rational-root completeness test: callers must verify every proposal exactly.
func intervalRationalConvergents(lo, hi *big.Rat, b *polynomialBudget) ([]*big.Rat, error) {
	midpoint := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
	if e := b.check(midpoint); e != nil {
		return nil, e
	}
	numerator, denominator := new(big.Int).Set(midpoint.Num()), new(big.Int).Set(midpoint.Denom())
	p0, p1 := big.NewInt(0), big.NewInt(1)
	q0, q1 := big.NewInt(1), big.NewInt(0)
	var result []*big.Rat
	for denominator.Sign() != 0 {
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(numerator, denominator, remainder)
		p := new(big.Int).Add(new(big.Int).Mul(quotient, p1), p0)
		q := new(big.Int).Add(new(big.Int).Mul(quotient, q1), q0)
		if q.Sign() != 0 {
			candidate := new(big.Rat).SetFrac(p, q)
			if e := b.check(candidate); e != nil {
				return nil, e
			}
			if candidate.Cmp(lo) > 0 && candidate.Cmp(hi) < 0 {
				result = append(result, candidate)
			}
		}
		numerator, denominator = denominator, remainder
		p0, p1 = p1, p
		q0, q1 = q1, q
	}
	return result, nil
}
