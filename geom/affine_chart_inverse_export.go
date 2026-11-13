package geom

import "math/big"

// AffineChartInverseOptions bounds rational inversion independently of chart
// construction. Zero selects 100000 operations and 16384 coefficient bits.
type AffineChartInverseOptions struct{ MaxOperations, MaxCoefficientBits int }

// RepresentedUV converts exact original parameters without rounding. This is a
// conversion of result data, not a revalidation of mutable OnPlane/Inside flags.
func (r AffineChartInverse) RepresentedUV() (Vec2, error) {
	if r.U == nil || r.V == nil {
		return Vec2{}, ErrInvalid
	}
	u, e := exactRepresentableFloat(r.U)
	if e != nil {
		return Vec2{}, e
	}
	v, e := exactRepresentableFloat(r.V)
	if e != nil {
		return Vec2{}, e
	}
	return Vec2{u, v}, nil
}

// RepresentedExactPoint converts a rational point without rounding. It returns
// ErrUnrepresentable unless all three coordinates are finite exact float64s.
// It does not assert that the supplied point belongs to any geometry.
func RepresentedExactPoint(p [3]*big.Rat) (Vec3, error) {
	var x [3]float64
	for i, q := range p {
		if q == nil {
			return Vec3{}, ErrInvalid
		}
		v, e := exactRepresentableFloat(q)
		if e != nil {
			return Vec3{}, e
		}
		x[i] = v
	}
	return Vec3{x[0], x[1], x[2]}, nil
}
