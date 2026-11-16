package geom

import "math/big"

func nonzeroSweepSeparatorCovector(covector [3]*big.Rat) bool {
	for _, q := range covector {
		if q != nil && q.Sign() != 0 {
			return true
		}
	}
	return false
}

func checkSweepSeparatorCovector(covector [3]*big.Rat, budget *polynomialBudget) error {
	for _, q := range covector {
		if q != nil {
			if err := budget.check(q); err != nil {
				return err
			}
		}
	}
	return nil
}

func sweepSeparatorPolynomial(components [3]rationalPolynomial, covector [3]*big.Rat,
	budget *polynomialBudget) (rationalPolynomial, error) {
	var projection rationalPolynomial
	for i, q := range covector {
		if q == nil || q.Sign() == 0 {
			continue
		}
		term, err := polynomialProduct(components[i], rationalPolynomial{q}, budget)
		if err != nil {
			return nil, err
		}
		projection, err = graphPolynomialSum(projection, term, 1, budget)
		if err != nil {
			return nil, err
		}
	}
	return projection, nil
}

func sweepSeparatorProjectedDifferential(d *offsetCellJacobian, fixed int, covector [3]*big.Rat) (exactInterval, exactInterval) {
	a := &d.arithmetic
	var direction [3]exactInterval
	for i, q := range covector {
		if q == nil {
			direction[i] = offsetZeroInterval()
		} else {
			direction[i] = exactInterval{q, q}
		}
	}
	projectedNormal := a.dot(direction, d.normal)
	projectedPartial := a.dot(direction, d.normalPartials[fixed])
	// Differentiate the projected normalized vector using the same
	// cell-wide scaling and directed length bounds as the axis path.
	numerator := a.sub(a.mul(projectedPartial, d.length2), a.mul(projectedNormal, d.normalDot[fixed]))
	return a.dot(direction, d.tangent[fixed]), a.div(numerator, d.length3)
}
