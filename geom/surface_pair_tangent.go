package geom

import "math/big"

// CertifySurfacePairTangentComponent bounds one signed cofactor of the
// contact Jacobian [Au, Av, -Bu, -Bv] on two Bezier cells. The cofactor is
// the corresponding coordinate of a consistently oriented tangent to the
// regular four-parameter intersection. A strict sign proves monotonicity
// of that parameter on any regular arc in the cells. A zero sign is unknown.
func CertifySurfacePairTangentComponent(a, b *SurfacePatch, axis int, o SurfaceTransversalityOptions) (CertifiedScalarRange, error) {
	if a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 || axis < 0 || axis > 3 {
		return CertifiedScalarRange{}, ErrInvalid
	}
	if a.countU != a.degreeU+1 || a.countV != a.degreeV+1 || b.countU != b.degreeU+1 || b.countV != b.degreeV+1 {
		return CertifiedScalarRange{}, ErrDomain
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return CertifiedScalarRange{}, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	_, da, e := surfaceDifferentialIntervals(a, -1, 0, budget)
	if e != nil {
		return CertifiedScalarRange{}, e
	}
	_, db, e := surfaceDifferentialIntervals(b, -1, 0, budget)
	if e != nil {
		return CertifiedScalarRange{}, e
	}
	var columns [3][3]exactInterval
	for j, col := 0, 0; j < 4; j++ {
		if j == axis {
			continue
		}
		for k := range 3 {
			if j < 2 {
				columns[col][k] = da[k][j]
			} else {
				x := db[k][j-2]
				columns[col][k] = exactInterval{new(big.Rat).Neg(x.hi), new(big.Rat).Neg(x.lo)}
			}
		}
		col++
	}
	cross, e := intervalCross(columns[1], columns[2], budget)
	if e != nil {
		return CertifiedScalarRange{}, e
	}
	zero := new(big.Rat)
	det := exactInterval{zero, zero}
	for k := range 3 {
		x, e := intervalMultiply(columns[0][k], cross[k], budget)
		if e != nil {
			return CertifiedScalarRange{}, e
		}
		det, e = boundedExactInterval(new(big.Rat).Add(det.lo, x.lo), new(big.Rat).Add(det.hi, x.hi), budget)
		if e != nil {
			return CertifiedScalarRange{}, e
		}
	}
	if axis&1 != 0 {
		det = exactInterval{new(big.Rat).Neg(det.hi), new(big.Rat).Neg(det.lo)}
	}
	return certifiedRange(det), nil
}
