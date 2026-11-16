package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// Both native graphs have already been proved to have a uniform positive
// projective weight on one clamped unit cell. Their original Z poles are
// therefore ordinary tensor Bernstein coefficients. Derivative control
// values are finite differences of those exact stored heights.
func parabolaCircleMeshHessian(s *geom.BSplineSurface) [3]*big.Rat {
	d := s.Data()
	height := make([]*big.Rat, len(d.Poles))
	for i, p := range d.Poles {
		height[i] = ratInset(p.Z)
	}
	coefficients := [3][]int64{{1}, {-1, 1}, {1, -2, 1}}
	orders := [3][2]int{{2, 0}, {1, 1}, {0, 2}}
	var bounds [3]*big.Rat
	for k, order := range orders {
		du, dv := order[0], order[1]
		bound := new(big.Rat)
		bounds[k] = bound
		if du > d.DegreeU || dv > d.DegreeV {
			continue
		}
		scale := int64(1)
		for i := range du {
			scale *= int64(d.DegreeU - i)
		}
		for j := range dv {
			scale *= int64(d.DegreeV - j)
		}
		for i := range d.CountU - du {
			for j := range d.CountV - dv {
				var coefficient, term big.Rat
				for u, a := range coefficients[du] {
					for v, b := range coefficients[dv] {
						term.SetInt64(a * b * scale)
						term.Mul(&term, height[(i+u)*d.CountV+j+v])
						coefficient.Add(&coefficient, &term)
					}
				}
				coefficient.Abs(&coefficient)
				if coefficient.Cmp(bound) > 0 {
					bound.Set(&coefficient)
				}
			}
		}
	}
	return bounds
}

func parabolaCircleMeshSourceTurnSquared(height [2]parabolaCircleMeshHeight, width [2]*big.Rat, pad *big.Rat) *big.Rat {
	// Each footprint triangle covers its padded chart neighborhood. Any
	// two source positions there differ by at most the padded widths.
	// Coordinatewise Hessian bounds enclose their gradient difference
	// along the straight segment in the unit chart. The map from gradient
	// to the unit ray (-z_u,-z_v,1)/|(-z_u,-z_v,1)| has differential norm
	// at most one, since every raw ray has length at least one. Applying
	// this map to the straight segment in gradient space bounds spherical
	// angle in radians by the Euclidean gradient difference.
	pad2 := new(big.Rat).Add(pad, pad)
	var span [2]*big.Rat
	for axis := range span {
		span[axis] = new(big.Rat).Add(width[axis], pad2)
		if span[axis].Cmp(big.NewRat(1, 1)) > 0 {
			span[axis].SetInt64(1)
		}
	}
	maximum := new(big.Rat)
	for _, h := range height {
		m := h.normalHessian
		u := new(big.Rat).Add(new(big.Rat).Mul(m[0], span[0]), new(big.Rat).Mul(m[1], span[1]))
		v := new(big.Rat).Add(new(big.Rat).Mul(m[1], span[0]), new(big.Rat).Mul(m[2], span[1]))
		u.Mul(u, u)
		v.Mul(v, v)
		q := u.Add(u, v)
		if q.Cmp(maximum) > 0 {
			maximum.Set(q)
		}
	}
	return maximum
}
