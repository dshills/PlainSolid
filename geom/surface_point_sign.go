package geom

import "math/big"

// SurfacePointSignOptions limits exact extraction/evaluation. Zero fields select
// 1M rational operations and 16384 coefficient bits.
type SurfacePointSignOptions struct{ MaxOperations, MaxCoefficientBits int }

// SurfacePointSign certifies the exact stored plane equation at represented
// parameters. Distance bounds dot(storedNormal, point-storedOrigin); normal
// normalization roundoff is not removed. Sign is exact even when bounds round
// to zero. Position
// encloses the exact real surface point; evaluations at interior knots select
// the right piece, and final endpoints select the left piece.
type SurfacePointSign struct {
	Sign            int
	Distance        CertifiedScalarRange
	Position        Box
	ExactOperations int
}

// ClassifySurfacePointPlane evaluates stored float64 parameters as exact binary
// rationals. It does not use rounded surface coordinates or dot products to
// decide the plane side. Continuous clamped SurfacePatch domains are supported,
// including arbitrary internal knots; only the containing pieces are extracted.
func ClassifySurfacePointPlane(p *SurfacePatch, plane Plane, u, v float64, o SurfacePointSignOptions) (SurfacePointSign, error) {
	if !finite(u) || !finite(v) {
		return SurfacePointSign{}, ErrDomain
	}
	return ClassifySurfacePointPlaneExact(p, plane, new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), o)
}

// ClassifySurfacePointPlaneExact also accepts exact rational parameters, retaining
// their phase/position when not representable in float64. Parameters are copied
// by arithmetic and never mutated. Their bit sizes count against the limits.
func ClassifySurfacePointPlaneExact(p *SurfacePatch, plane Plane, u, v *big.Rat, o SurfacePointSignOptions) (result SurfacePointSign, err error) {
	if p == nil || len(p.net) == 0 || !plane.valid() || u == nil || v == nil {
		return result, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	if e := b.check(u); e != nil {
		return result, e
	}
	if e := b.check(v); e != nil {
		return result, e
	}
	d := p.Domain()
	if u.Cmp(new(big.Rat).SetFloat64(d.UMin)) < 0 || u.Cmp(new(big.Rat).SetFloat64(d.UMax)) > 0 || v.Cmp(new(big.Rat).SetFloat64(d.VMin)) < 0 || v.Cmp(new(big.Rat).SetFloat64(d.VMax)) > 0 {
		return result, ErrDomain
	}
	current := p
	for {
		t, axis, knot := patchSplitChoice(current)
		if !knot {
			break
		}
		left, right, e := current.split(t, axis, b)
		if e != nil {
			return result, e
		}
		parameter := u
		if !axis {
			parameter = v
		}
		if parameter.Cmp(new(big.Rat).SetFloat64(t)) < 0 {
			current = left
		} else {
			current = right
		}
	}
	point, _, e := exactSurfaceJet(current, u, v, b)
	if e != nil {
		return result, e
	}
	distance := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		lo, hi := rationalFloatBound(point[axis], true), rationalFloatBound(point[axis], false)
		switch axis {
		case 0:
			result.Position.Min.X, result.Position.Max.X = lo, hi
		case 1:
			result.Position.Min.Y, result.Position.Max.Y = lo, hi
		case 2:
			result.Position.Min.Z, result.Position.Max.Z = lo, hi
		}
		delta := new(big.Rat).Sub(point[axis], new(big.Rat).SetFloat64(coordinate(plane.origin, axis)))
		distance.Add(distance, new(big.Rat).Mul(delta, new(big.Rat).SetFloat64(coordinate(plane.normal, axis))))
		if e := b.check(distance); e != nil {
			return result, e
		}
	}
	result.Sign = distance.Sign()
	result.Distance = certifiedRange(exactInterval{distance, distance})
	return result, nil
}
