package geom

import "math/big"

// The original height difference is the factored numerator divided by the
// homogeneous denominator. The factors are monic in the dependent variable,
// so the unmodified highest power coefficient gives their signed scale.
func pocketDiskHeightScale(diff *SurfacePatch, leading *big.Rat, budget *polynomialBudget) (*big.Rat, error) {
	denominator := diff.net[0][3]
	if denominator.Sign() <= 0 {
		return nil, ErrUnrepresentable
	}
	for _, h := range diff.net {
		if h[3].Cmp(denominator) != 0 {
			return nil, ErrUnrepresentable
		}
	}
	scale := new(big.Rat).Quo(leading, denominator)
	return scale, budget.check(scale)
}

// pocketDiskVolumePiCoefficient integrates the authenticated height
// difference over the complete original inner disk. Its physical chart
// is unit XY (possibly written with the free/dependent order exchanged).
func pocketDiskVolumePiCoefficient(scale *big.Rat, p rationalPolynomial, crossing, disk *GraphContactCircle,
	height *big.Rat, budget *polynomialBudget) (*big.Rat, error) {
	cx, cy := disk.Center[0], disk.Center[1]
	dx, dy := new(big.Rat).Sub(cx, crossing.Center[0]), new(big.Rat).Sub(cy, crossing.Center[1])
	constant := new(big.Rat).Sub(cy, height)
	linear := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(p[2], cx))
	linear.Add(linear, p[1]).Neg(linear)
	quadratic := new(big.Rat).Neg(p[2])
	outer := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	outer.Sub(outer, crossing.RadiusSquared)
	r2 := disk.RadiusSquared

	// In centered coordinates, all odd monomials integrate to zero.
	// The remaining terms of (y-p)Qcross are constant*outer, the
	// summed h² and k² coefficient below, and quadratic*(h⁴+h²k²).
	// Weighted by Qdisk, their integrals divided by π are -r⁴/2,
	// -r⁶/12 for each quadratic monomial and -r⁸/24 for the last sum.
	second := new(big.Rat).Add(constant, new(big.Rat).Mul(linear, dx))
	second.Add(second, dy).Mul(second, big.NewRat(2, 1))
	second.Add(second, new(big.Rat).Mul(quadratic, outer))
	r4 := new(big.Rat).Mul(r2, r2)
	r6 := new(big.Rat).Mul(r4, r2)
	r8 := new(big.Rat).Mul(r6, r2)
	constantTerm := new(big.Rat).Mul(constant, outer)
	constantTerm.Mul(constantTerm, new(big.Rat).Quo(r4, big.NewRat(2, 1)))
	quadraticTerm := new(big.Rat).Mul(second, new(big.Rat).Quo(r6, big.NewRat(12, 1)))
	quarticTerm := new(big.Rat).Mul(quadratic, new(big.Rat).Quo(r8, big.NewRat(24, 1)))
	coefficient := new(big.Rat).Add(constantTerm, quadraticTerm)
	coefficient.Add(coefficient, quarticTerm).Mul(coefficient, scale).Neg(coefficient)
	for _, value := range []*big.Rat{dx, dy, constant, linear, quadratic, outer, second,
		r4, r6, r8, constantTerm, quadraticTerm, quarticTerm, coefficient} {
		if err := budget.check(value); err != nil {
			return nil, err
		}
	}
	return coefficient, nil
}
