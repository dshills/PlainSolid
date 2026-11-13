package geom

import (
	"context"
	"math/big"
)

// certifyRevolutionRationalHeight proves strict axial monotonicity on every
// nonzero source knot span from homogeneous Bernstein coefficients. It never
// accepts a sampled derivative or an inconclusive coefficient hull.
func certifyRevolutionRationalHeight(c *BSpline, axis int, sign float64) error {
	if c.degree > 16 || len(c.poles) > 256 {
		return ErrConvergence
	}
	lo, hi := c.Domain()
	spans, ops, e := c.ExactBezierSpans(context.Background(), lo, hi, ExactBezierSpanOptions{SurfacePatchOptions: SurfacePatchOptions{MaxPoles: 8192, MaxOperations: 600000, MaxCoefficientBits: 8192}, MaxSpans: 64})
	if e != nil {
		return e
	}
	budget := polynomialBudget{operations: ops, maxOperations: 600000, maxBits: 8192}
	for _, span := range spans {
		n := len(span.Control) - 1
		coeff := make([]*big.Rat, 2*n)
		for i := range coeff {
			coeff[i] = new(big.Rat)
		}
		for i := 0; i < n; i++ {
			dh := new(big.Rat).Sub(span.Control[i+1][axis], span.Control[i][axis])
			dh.Mul(dh, big.NewRat(int64(n), 1))
			dw := new(big.Rat).Sub(span.Control[i+1][3], span.Control[i][3])
			dw.Mul(dw, big.NewRat(int64(n), 1))
			if e = budget.check(dh); e != nil {
				return e
			}
			if e = budget.check(dw); e != nil {
				return e
			}
			for j := 0; j <= n; j++ {
				term := new(big.Rat).Sub(new(big.Rat).Mul(dh, span.Control[j][3]), new(big.Rat).Mul(dw, span.Control[j][axis]))
				numerator := new(big.Int).Mul(new(big.Int).Binomial(int64(n-1), int64(i)), new(big.Int).Binomial(int64(n), int64(j)))
				term.Mul(term, new(big.Rat).SetFrac(numerator, new(big.Int).Binomial(int64(2*n-1), int64(i+j))))
				if e = budget.check(term); e != nil {
					return e
				}
				coeff[i+j].Add(coeff[i+j], term)
				if e = budget.check(coeff[i+j]); e != nil {
					return e
				}
			}
		}
		for _, v := range coeff {
			if v.Sign() == 0 || (v.Sign() > 0) != (sign > 0) {
				return ErrConvergence
			}
		}
	}
	return nil
}
