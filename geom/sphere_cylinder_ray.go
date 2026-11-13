package geom

import (
	"context"
	"math/big"
)

// SphereCylinderRayContact denotes one exact point on the selected rational
// cylinder meridian. t=Offset+RootSign*sqrt(Radicand), and model point is
// axisOrigin+radius*ray+axis*t. MeridianTangent is a double root along that
// generator; SurfaceTangent additionally proves parallel surface normals.
type SphereCylinderRayContact struct {
	Offset, Radicand                *big.Rat
	RootSign                        int
	ParameterBounds                 [2]*big.Rat
	PointBounds                     [3][2]*big.Rat
	MeridianTangent, SurfaceTangent bool
}
type SphereCylinderRayIntersection struct {
	Complete        bool
	Contacts        []SphereCylinderRayContact
	ExactOperations int
}

// IntersectSphereCylinderRationalRay intersects a sphere with one generator
// of a circular cylinder. Axis must be nonzero and coordinate-aligned, with
// arbitrary represented scale/sign. ray is an exact rational unit vector
// perpendicular to it; it is not normalized or converted to float. This admits
// off-axis sphere centers and nonbinary directions such as (3/5,4/5,0).
// Window clips the closed raw-axis t range exactly; nil means unbounded.
// No global angular branch count or cylinder topology is asserted.
func IntersectSphereCylinderRationalRay(ctx context.Context, center Vec3, sphereRadius float64, axisOrigin, axis Vec3, cylinderRadius float64, ray [3]*big.Rat, window *[2]*big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderRayIntersection, err error) {
	if ctx == nil || !center.IsFinite() || !axisOrigin.IsFinite() || !axis.IsFinite() || !finite(sphereRadius) || !finite(cylinderRadius) || sphereRadius <= 0 || cylinderRadius <= 0 {
		return out, ErrInvalid
	}
	ax := -1
	for i := 0; i < 3; i++ {
		if ray[i] == nil {
			return out, ErrInvalid
		}
		if coordinate(axis, i) != 0 {
			if ax >= 0 {
				return out, ErrDomain
			}
			ax = i
		}
	}
	if ax < 0 {
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
			out.Contacts = nil
		}
	}()
	rr := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	norm := new(big.Rat)
	for _, v := range ray {
		if e := b.check(v); e != nil {
			return out, e
		}
		norm.Add(norm, new(big.Rat).Mul(v, v))
	}
	if norm.Cmp(big.NewRat(1, 1)) != 0 || ray[ax].Sign() != 0 {
		return out, ErrDomain
	}
	base, d := [3]*big.Rat{}, [3]*big.Rat{}
	radialDistance := new(big.Rat)
	for i := 0; i < 3; i++ {
		base[i] = new(big.Rat).Add(rr(coordinate(axisOrigin, i)), new(big.Rat).Mul(rr(cylinderRadius), ray[i]))
		d[i] = new(big.Rat).Sub(rr(coordinate(center, i)), base[i])
		if i != ax {
			radialDistance.Add(radialDistance, new(big.Rat).Mul(d[i], d[i]))
		}
		for _, q := range []*big.Rat{base[i], d[i], radialDistance} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	scale := rr(coordinate(axis, ax))
	offset := new(big.Rat).Quo(d[ax], scale)
	rad := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(rr(sphereRadius), rr(sphereRadius)), radialDistance), new(big.Rat).Mul(scale, scale))
	for _, q := range []*big.Rat{offset, rad} {
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
	cmp := func(sign int, v *big.Rat) int {
		delta := new(big.Rat).Sub(v, offset)
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
	lo, hi, e := rationalSqrtBounds(rad, 96, b)
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
		q := SphereCylinderRayContact{Offset: new(big.Rat).Set(offset), Radicand: new(big.Rat).Set(rad), RootSign: sign, MeridianTangent: sign == 0}
		if sign < 0 {
			q.ParameterBounds = [2]*big.Rat{new(big.Rat).Sub(offset, hi), new(big.Rat).Sub(offset, lo)}
		} else if sign > 0 {
			q.ParameterBounds = [2]*big.Rat{new(big.Rat).Add(offset, lo), new(big.Rat).Add(offset, hi)}
		} else {
			q.ParameterBounds = [2]*big.Rat{new(big.Rat).Set(offset), new(big.Rat).Set(offset)}
		}
		if sign == 0 {
			j, k := (ax+1)%3, (ax+2)%3
			q.SurfaceTangent = new(big.Rat).Sub(new(big.Rat).Mul(d[j], ray[k]), new(big.Rat).Mul(d[k], ray[j])).Sign() == 0
		}
		for i := 0; i < 3; i++ {
			v := rr(coordinate(axis, i))
			low, high := q.ParameterBounds[0], q.ParameterBounds[1]
			if v.Sign() < 0 {
				low, high = high, low
			}
			q.PointBounds[i] = [2]*big.Rat{new(big.Rat).Add(base[i], new(big.Rat).Mul(v, low)), new(big.Rat).Add(base[i], new(big.Rat).Mul(v, high))}
			for _, value := range q.PointBounds[i] {
				if e = b.check(value); e != nil {
					return out, e
				}
			}
		}
		out.Contacts = append(out.Contacts, q)
	}
	out.Complete = true
	return out, nil
}
