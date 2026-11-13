package geom

import (
	"context"
	"math/big"
)

// CoaxialSphereCylinderCircle is an exact circle in the plane perpendicular to
// Axis at origin+Axis*(ParameterOffset+RootSign*sqrt(ParameterRadicand)).
// The raw axis parameter preserves the caller's scale. CenterBounds enclose
// the algebraic center; they are not a replacement for its exact expression.
// Tangent denotes coincident surface tangent planes along the entire circle.
type CoaxialSphereCylinderCircle struct {
	ParameterOffset, ParameterRadicand, RadiusSquared *big.Rat
	RootSign                                          int
	ParameterBounds                                   [2]*big.Rat
	CenterBounds                                      [3][2]*big.Rat
	Tangent, Transverse                               bool
}
type CoaxialSphereCylinderIntersection struct {
	Complete        bool
	Circles         []CoaxialSphereCylinderCircle
	ExactOperations int
}

// IntersectCoaxialSphereCylinder exactly intersects a represented sphere with
// a circular cylinder whose raw axis is axisOrigin+t*axis. No unit-vector
// rounding is performed. Sphere center must lie exactly on that line; an
// off-axis input returns ErrDomain, not an empty intersection. Optional window
// is a closed interval of raw t and clips whole circles without approximating
// their algebraic heights. Nil requests the infinite cylindrical support.
// Positive radii are required; tangent circles are retained distinctly from
// transverse circles. Exact work/bit limits use SurfacePatchEvaluationOptions.
func IntersectCoaxialSphereCylinder(ctx context.Context, sphereCenter Vec3, sphereRadius float64, axisOrigin, axis Vec3, cylinderRadius float64, window *[2]*big.Rat, o SurfacePatchEvaluationOptions) (out CoaxialSphereCylinderIntersection, err error) {
	if ctx == nil || !sphereCenter.IsFinite() || !axisOrigin.IsFinite() || !axis.IsFinite() || axis == (Vec3{}) || !finite(sphereRadius) || !finite(cylinderRadius) || sphereRadius <= 0 || cylinderRadius <= 0 {
		return out, ErrInvalid
	}
	if window != nil && (window[0] == nil || window[1] == nil || window[0].Cmp(window[1]) > 0) {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Circles = nil
		}
	}()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	v, d, origin := [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}
	norm, dot := new(big.Rat), new(big.Rat)
	for i := 0; i < 3; i++ {
		v[i] = r(coordinate(axis, i))
		origin[i] = r(coordinate(axisOrigin, i))
		d[i] = new(big.Rat).Sub(r(coordinate(sphereCenter, i)), origin[i])
		norm.Add(norm, new(big.Rat).Mul(v[i], v[i]))
		dot.Add(dot, new(big.Rat).Mul(v[i], d[i]))
		for _, q := range []*big.Rat{v[i], origin[i], d[i], norm, dot} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	offset := new(big.Rat).Quo(dot, norm)
	for i := 0; i < 3; i++ {
		q := new(big.Rat).Sub(d[i], new(big.Rat).Mul(v[i], offset))
		if e := b.check(q); e != nil {
			return out, e
		}
		if q.Sign() != 0 {
			return out, ErrDomain
		}
	}
	cr2 := new(big.Rat).Mul(r(cylinderRadius), r(cylinderRadius))
	rad := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(r(sphereRadius), r(sphereRadius)), cr2), norm)
	for _, q := range []*big.Rat{offset, cr2, rad} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	if window != nil {
		for _, q := range *window {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	if rad.Sign() < 0 {
		out.Complete = true
		return out, nil
	}
	cmp := func(sign int, q *big.Rat) int {
		delta := new(big.Rat).Sub(q, offset)
		if sign == 0 {
			return -delta.Sign()
		}
		if sign < 0 {
			delta.Neg(delta)
		}
		k := 1
		if delta.Sign() >= 0 {
			k = rad.Cmp(new(big.Rat).Mul(delta, delta))
		}
		if sign < 0 {
			k = -k
		}
		return k
	}
	low, high, e := rationalSqrtBounds(rad, 96, b)
	if e != nil {
		return out, e
	}
	signs := []int{-1, 1}
	if rad.Sign() == 0 {
		signs = []int{0}
	}
	for _, sign := range signs {
		if window != nil && (cmp(sign, window[0]) < 0 || cmp(sign, window[1]) > 0) {
			continue
		}
		q := CoaxialSphereCylinderCircle{ParameterOffset: new(big.Rat).Set(offset), ParameterRadicand: new(big.Rat).Set(rad), RadiusSquared: new(big.Rat).Set(cr2), RootSign: sign, Tangent: sign == 0, Transverse: sign != 0}
		if sign < 0 {
			q.ParameterBounds = [2]*big.Rat{new(big.Rat).Sub(offset, high), new(big.Rat).Sub(offset, low)}
		} else if sign > 0 {
			q.ParameterBounds = [2]*big.Rat{new(big.Rat).Add(offset, low), new(big.Rat).Add(offset, high)}
		} else {
			q.ParameterBounds = [2]*big.Rat{new(big.Rat).Set(offset), new(big.Rat).Set(offset)}
		}
		for i := 0; i < 3; i++ {
			lo, hi := q.ParameterBounds[0], q.ParameterBounds[1]
			if v[i].Sign() < 0 {
				lo, hi = hi, lo
			}
			q.CenterBounds[i] = [2]*big.Rat{new(big.Rat).Add(origin[i], new(big.Rat).Mul(v[i], lo)), new(big.Rat).Add(origin[i], new(big.Rat).Mul(v[i], hi))}
			for _, value := range q.CenterBounds[i] {
				if e = b.check(value); e != nil {
					return out, e
				}
			}
		}
		out.Circles = append(out.Circles, q)
	}
	out.Complete = true
	return out, nil
}
