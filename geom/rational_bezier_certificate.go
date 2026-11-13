package geom

import "math/big"

// RationalBezierCoefficients returns owned exact homogeneous power coefficients
// on normalized local [0,1]^2, with dimensions (DegreeU+1)x(DegreeV+1). The
// positive minimum control weight bounds the denominator over that square.
// Single-span degrees up to maxDegree<=6 are supported. Arithmetic uses the
// immutable patch's operation/bit limits; false means unsupported or exhausted.
func (p *SurfacePatch) RationalBezierCoefficients(maxDegree int) (coefficients [][][4]*big.Rat, minimumWeight *big.Rat, ok bool) {
	if p == nil || maxDegree < 1 || maxDegree > 6 || p.degreeU > maxDegree || p.degreeV > maxDegree || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || len(p.net) == 0 {
		return nil, nil, false
	}
	options, e := patchOptions(p.options)
	if e != nil {
		return nil, nil, false
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	minimumWeight = new(big.Rat).Set(p.net[0][3])
	for _, h := range p.net {
		if h[3].Sign() <= 0 {
			return nil, nil, false
		}
		if h[3].Cmp(minimumWeight) < 0 {
			minimumWeight.Set(h[3])
		}
		for _, q := range h {
			if budget.check(q) != nil {
				return nil, nil, false
			}
		}
	}
	coefficients = make([][][4]*big.Rat, p.countU)
	for i := range coefficients {
		coefficients[i] = make([][4]*big.Rat, p.countV)
		for j := range coefficients[i] {
			for k := 0; k < 4; k++ {
				coefficients[i][j][k] = new(big.Rat)
			}
		}
	}
	for coordinate := 0; coordinate < 4; coordinate++ {
		rows := make([]rationalPolynomial, p.countU)
		for i := range rows {
			values := make([]*big.Rat, p.countV)
			for j := range values {
				values[j] = p.net[i*p.countV+j][coordinate]
			}
			rows[i], e = bernsteinToPower(values, budget)
			if e != nil {
				return nil, nil, false
			}
		}
		for j := 0; j < p.countV; j++ {
			values := make([]*big.Rat, p.countU)
			for i := range values {
				values[i] = new(big.Rat)
				if j < len(rows[i]) {
					values[i].Set(rows[i][j])
				}
			}
			column, e := bernsteinToPower(values, budget)
			if e != nil {
				return nil, nil, false
			}
			for i, q := range column {
				coefficients[i][j][coordinate] = new(big.Rat).Set(q)
			}
		}
	}
	return coefficients, minimumWeight, true
}

// RationalBezierDerivativeBounds bounds the original-parameter 3x2 Jacobian.
// SquaredFrobeniusUpper is the sum of squared component absolute maxima;
// SumAbsoluteJacobianUpper is their sum. Thus sqrt(first) bounds the Euclidean
// Lipschitz constant on the convex rectangle. No normalized-UV scaling is
// implicit. False means degree/representation/resource limits prevent a proof.
func (p *SurfacePatch) RationalBezierDerivativeBounds(maxDegree int) (squaredFrobeniusUpper, sumAbsoluteJacobianUpper *big.Rat, ok bool) {
	if p == nil || maxDegree < 1 || maxDegree > 6 || p.degreeU > maxDegree || p.degreeV > maxDegree || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return nil, nil, false
	}
	options, e := patchOptions(p.options)
	if e != nil {
		return nil, nil, false
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	_, d, e := surfaceDifferentialIntervals(p, -1, 0, budget)
	if e != nil {
		return nil, nil, false
	}
	squaredFrobeniusUpper = new(big.Rat)
	sumAbsoluteJacobianUpper = new(big.Rat)
	for i := 0; i < 3; i++ {
		for j := 0; j < 2; j++ {
			m := new(big.Rat).Abs(d[i][j].lo)
			hi := new(big.Rat).Abs(d[i][j].hi)
			if hi.Cmp(m) > 0 {
				m = hi
			}
			sumAbsoluteJacobianUpper.Add(sumAbsoluteJacobianUpper, m)
			squaredFrobeniusUpper.Add(squaredFrobeniusUpper, new(big.Rat).Mul(m, m))
			if budget.check(sumAbsoluteJacobianUpper) != nil || budget.check(squaredFrobeniusUpper) != nil {
				return nil, nil, false
			}
		}
	}
	return squaredFrobeniusUpper, sumAbsoluteJacobianUpper, true
}
