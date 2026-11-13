package geom

import "math/big"

// RationalOutwardRange encloses a single exact rational with directed float64
// bounds. Signed underflow is enclosed by zero and the adjacent subnormal;
// overflow is enclosed by the largest finite value and infinity. No Float64
// exactness flag is trusted. The input is read-only; nil is invalid.
func RationalOutwardRange(x *big.Rat) (CertifiedScalarRange, error) {
	if x == nil {
		return CertifiedScalarRange{}, ErrInvalid
	}
	return CertifiedScalarRange{Min: rationalFloatBound(x, true), Max: rationalFloatBound(x, false)}, nil
}
