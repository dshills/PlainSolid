package step

import (
	"math/big"

	"plainsolid/geom"
)

// This bound applies only where the original directed LINE endpoints have
// the same projected parameters as the independently proved repaired edge.
// The source pcurve's direction ratios are normalized by STEP semantics.
func boundFiniteConvexArcLineOwner(src, corrected *File, q ConvexArcPCurveAgreement, model int64, tolSquared *big.Rat) (FiniteConvexArcLineOwner, *big.Rat, error) {
	return boundMappedConvexArcLineOwner(src, corrected, q, model, tolSquared, nil)
}

func boundMappedConvexArcLineOwner(src, corrected *File, q ConvexArcPCurveAgreement, model int64, tolSquared, parameterScale *big.Rat) (FiniteConvexArcLineOwner, *big.Rat, error) {
	bad := func() (FiniteConvexArcLineOwner, *big.Rat, error) {
		return FiniteConvexArcLineOwner{}, nil, geom.ErrDomain
	}
	support, e := simple(src, q.SupportID, "PLANE", 2)
	if e != nil {
		return bad()
	}
	o, x, y, _, e := convexUVFrame(src, support.Args[1].Ref)
	if e != nil {
		return bad()
	}
	p, d, domain, e := convexArcOwnerLineDomain(src, corrected, q, model, tolSquared)
	if e != nil {
		return bad()
	}

	line, e := simple(src, q.GeometryID, "LINE", 3)
	if e != nil {
		return bad()
	}
	uv, e := convexUVVector(src, line.Args[1].Ref, "CARTESIAN_POINT", 2)
	if e != nil {
		return bad()
	}
	vec, e := simple(src, line.Args[2].Ref, "VECTOR", 3)
	if e != nil {
		return bad()
	}
	ratio, e := convexUVVector(src, vec.Args[1].Ref, "DIRECTION", 2)
	if e != nil {
		return bad()
	}
	magnitude, e := liftSourceRat(vec.Args[2])
	if e != nil || magnitude.Sign() <= 0 {
		return bad()
	}
	if parameterScale != nil {
		// The converted metre dialect maps a unit-speed planar pcurve to the
		// authored model speed. Both speeds are proved before applying it.
		squaredScale := new(big.Rat).Mul(parameterScale, parameterScale)
		if magnitude.Cmp(big.NewRat(1, 1)) != 0 || occtLoopDot(d, d).Cmp(squaredScale) != 0 {
			return bad()
		}
		magnitude.Mul(magnitude, parameterScale)
	}
	normSquared := new(big.Rat).Add(new(big.Rat).Mul(ratio[0], ratio[0]), new(big.Rat).Mul(ratio[1], ratio[1]))
	if normSquared.Sign() == 0 {
		return bad()
	}
	// ||r/||r|| - r|| = |1 - ||r|| | <= |1 - ||r||²|. The original
	// orthonormal plane chart preserves this error in world space.
	normalization := new(big.Rat).Abs(new(big.Rat).Sub(normSquared, big.NewRat(1, 1)))
	maxT := new(big.Rat).Abs(domain[0])
	if t := new(big.Rat).Abs(domain[1]); t.Cmp(maxT) > 0 {
		maxT = t
	}
	normalization.Mul(normalization, magnitude)
	normalization.Mul(normalization, maxT)
	liftPoint := convexUVLift(o, x, y, uv[0], uv[1])
	liftDirection := convexUVLift(convexUVZero(), x, y, new(big.Rat).Mul(magnitude, ratio[0]), new(big.Rat).Mul(magnitude, ratio[1]))
	delta, velocity := occtLoopSub(liftPoint, p), occtLoopSub(liftDirection, d)
	squared := new(big.Rat)
	for j := range delta {
		bound := new(big.Rat)
		for _, t := range domain {
			at := new(big.Rat).Abs(new(big.Rat).Add(delta[j], new(big.Rat).Mul(t, velocity[j])))
			if at.Cmp(bound) > 0 {
				bound = at
			}
		}
		bound.Add(bound, normalization)
		squared.Add(squared, new(big.Rat).Mul(bound, bound))
	}
	return FiniteConvexArcLineOwner{PCurveID: q.PCurveID, StartParameter: domain[0], EndParameter: domain[1], PCurveParameterScale: parameterScale}, squared, nil
}

func convexArcOwnerLineDomain(src, corrected *File, q ConvexArcPCurveAgreement, model int64, tolSquared *big.Rat) (p, d [3]*big.Rat, domain [2]*big.Rat, err error) {
	originalPoint, originalDirection, e := convexUVLine(src, model, 3)
	if e != nil {
		return p, d, domain, e
	}
	repairedPoint, repairedDirection, e := convexUVLine(corrected, model, 3)
	if e != nil {
		return p, d, domain, e
	}
	p, d = [3]*big.Rat(repairedPoint), [3]*big.Rat(repairedDirection)
	if !convexUVSame([3]*big.Rat(originalDirection), d) {
		return p, d, domain, geom.ErrDomain
	}
	lsq := occtLoopDot(d, d)
	for _, f := range []*File{src, corrected} {
		edge, e := simple(f, q.EdgeID, "EDGE_CURVE", 5)
		if e != nil {
			return p, d, domain, e
		}
		point, line := p, model
		if f == src {
			point, line = [3]*big.Rat(originalPoint), q.SurfaceCurveID
		}
		if edge.Args[3].Ref != line {
			return p, d, domain, geom.ErrDomain
		}
		for i := range domain {
			v, e := simple(f, edge.Args[i+1].Ref, "VERTEX_POINT", 2)
			if e != nil {
				return p, d, domain, e
			}
			world, e := occtLoopTriple(f, v.Args[1].Ref, "CARTESIAN_POINT")
			if e != nil {
				return p, d, domain, e
			}
			t, residual := projectConvexArcLine(world, point, d, lsq)
			if residual.Cmp(tolSquared) > 0 {
				return p, d, domain, geom.ErrDomain
			}
			if f == src {
				domain[i] = t
			} else if t.Cmp(domain[i]) != 0 {
				return p, d, domain, geom.ErrDomain
			}
		}
		sense, e := boolean(edge.Args[4])
		if e != nil || !convexArcDirectedLineInterval(domain, sense) {
			return p, d, domain, geom.ErrDomain
		}
	}
	return p, d, domain, nil
}
