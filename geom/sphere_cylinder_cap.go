package geom

import (
	"context"
	"math/big"
)

// SphereCylinderCapPoint is an exact cap point Base+RootSign*sqrt(Radicand)*
// Vector. CapParameter is the caller's raw axis parameter and belongs to this
// cap, independently of numerical center/height displays. Tangent means the
// two planar circles touch, not necessarily that the original surfaces do.
type SphereCylinderCapPoint struct {
	CapParameter *big.Rat
	Base, Vector [3]*big.Rat
	Radicand     *big.Rat
	RootSign     int
	Bounds       [3][2]*big.Rat
	Tangent      bool
}
type SphereCylinderCapIntersection struct {
	Complete, EntireCircle bool
	CapParameter           *big.Rat
	Points                 []SphereCylinderCapPoint
	ExactOperations        int
}

// IntersectSphereCylinderCap finds all contacts on one finite cylinder rim
// axisOrigin+axis*capParameter+radius*unitRay. Axis must be coordinate-aligned
// but may have arbitrary represented scale/sign; capParameter stays rational.
// EntireCircle identifies exact coincidence of the complete rim and sphere.
// Calling this for each axial cap gives complete cap-event ownership; no
// unproved finite-branch connectivity is inferred between the caps.
func IntersectSphereCylinderCap(ctx context.Context, center Vec3, sphereRadius float64, axisOrigin, axis Vec3, cylinderRadius float64, capParameter *big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderCapIntersection, err error) {
	if ctx == nil || capParameter == nil || !center.IsFinite() || !axisOrigin.IsFinite() || !axis.IsFinite() || !finite(sphereRadius) || !finite(cylinderRadius) || sphereRadius <= 0 || cylinderRadius <= 0 {
		return out, ErrInvalid
	}
	ax := -1
	for i := 0; i < 3; i++ {
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
			out.EntireCircle = false
			out.Points = nil
		}
	}()
	if e := b.check(capParameter); e != nil {
		return out, e
	}
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	base, v := [3]*big.Rat{}, [3]*big.Rat{}
	h := new(big.Rat)
	for i := 0; i < 3; i++ {
		base[i] = new(big.Rat).Add(r(coordinate(axisOrigin, i)), new(big.Rat).Mul(r(coordinate(axis, i)), capParameter))
		v[i] = new(big.Rat)
		if i != ax {
			v[i].Sub(r(coordinate(center, i)), r(coordinate(axisOrigin, i)))
			h.Add(h, new(big.Rat).Mul(v[i], v[i]))
		}
		if e := b.check(base[i]); e != nil {
			return out, e
		}
	}
	dz := new(big.Rat).Sub(base[ax], r(coordinate(center, ax)))
	radius := r(cylinderRadius)
	numerator := new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Add(new(big.Rat).Mul(radius, radius), h), new(big.Rat).Mul(dz, dz)), new(big.Rat).Mul(r(sphereRadius), r(sphereRadius)))
	K := new(big.Rat).Quo(numerator, new(big.Rat).Mul(big.NewRat(2, 1), radius))
	for _, q := range []*big.Rat{h, dz, numerator, K} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out.CapParameter = new(big.Rat).Set(capParameter)
	if h.Sign() == 0 {
		out.EntireCircle = K.Sign() == 0
		out.Complete = true
		return out, nil
	}
	disc := new(big.Rat).Sub(h, new(big.Rat).Mul(K, K))
	if e := b.check(disc); e != nil {
		return out, e
	}
	if disc.Sign() < 0 {
		out.Complete = true
		return out, nil
	}
	rad := new(big.Rat).Quo(disc, new(big.Rat).Mul(h, h))
	lo, hi, e := rationalSqrtBounds(rad, 96, b)
	if e != nil {
		return out, e
	}
	signs := []int{-1, 1}
	if rad.Sign() == 0 {
		signs = []int{0}
	}
	j, k := (ax+1)%3, (ax+2)%3
	orientation := int64(1)
	if coordinate(axis, ax) < 0 {
		orientation = -1
	}
	for _, sign := range signs {
		q := SphereCylinderCapPoint{CapParameter: new(big.Rat).Set(capParameter), Radicand: new(big.Rat).Set(rad), RootSign: sign, Tangent: sign == 0}
		for i := 0; i < 3; i++ {
			q.Base[i] = new(big.Rat).Add(base[i], new(big.Rat).Quo(new(big.Rat).Mul(radius, new(big.Rat).Mul(K, v[i])), h))
			q.Vector[i] = new(big.Rat)
		}
		q.Vector[j].Mul(new(big.Rat).Mul(radius, big.NewRat(-orientation, 1)), v[k])
		q.Vector[k].Mul(new(big.Rat).Mul(radius, big.NewRat(orientation, 1)), v[j])
		for i := 0; i < 3; i++ {
			d := new(big.Rat).Mul(big.NewRat(int64(sign), 1), q.Vector[i])
			l, u := lo, hi
			if d.Sign() < 0 {
				l, u = u, l
			}
			q.Bounds[i] = [2]*big.Rat{new(big.Rat).Add(q.Base[i], new(big.Rat).Mul(d, l)), new(big.Rat).Add(q.Base[i], new(big.Rat).Mul(d, u))}
			for _, value := range []*big.Rat{q.Base[i], q.Vector[i], q.Bounds[i][0], q.Bounds[i][1]} {
				if e = b.check(value); e != nil {
					return out, e
				}
			}
		}
		out.Points = append(out.Points, q)
	}
	out.Complete = true
	return out, nil
}
