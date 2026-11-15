package step

import (
	"math/big"

	"plainsolid/geom"
)

// This bound applies only where the original directed LINE endpoints have
// the same projected parameters as the independently proved repaired edge.
// The source pcurve's direction ratios are normalized by STEP semantics.
func boundFiniteConvexArcLineOwner(src, corrected *File, q ConvexArcPCurveAgreement, model int64, tolSquared *big.Rat) (FiniteConvexArcLineOwner, *big.Rat, error) {
	return boundMappedConvexArcLineOwner(src, corrected, q, model, tolSquared, nil, nil)
}

func boundMappedConvexArcLineOwner(src, corrected *File, q ConvexArcPCurveAgreement, model int64, tolSquared *big.Rat, conversion *convertedConvexArcLength, trim *ConvexArcAxialTrimRepair) (FiniteConvexArcLineOwner, *big.Rat, error) {
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
	p, d, intervals, e := convexArcOwnerLineIntervals(src, corrected, q, model, tolSquared, trim)
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
	var parameterScale *big.Rat
	if conversion != nil {
		// The file unit defines the owner map. The affine lift bound below
		// still compares against the exact source LINE speed, including drift.
		squaredSpeed := new(big.Rat).Mul(conversion.modelSpeed, conversion.modelSpeed)
		if magnitude.Cmp(big.NewRat(1, 1)) != 0 || occtLoopDot(d, d).Cmp(squaredSpeed) != 0 {
			return bad()
		}
		parameterScale = conversion.ownerScale
		magnitude.Mul(magnitude, parameterScale)
	}
	normSquared := new(big.Rat).Add(new(big.Rat).Mul(ratio[0], ratio[0]), new(big.Rat).Mul(ratio[1], ratio[1]))
	if normSquared.Sign() == 0 {
		return bad()
	}
	// ||r/||r|| - r|| = |1 - ||r|| | <= |1 - ||r||²|. The original
	// orthonormal plane chart preserves this error in world space.
	normalization := new(big.Rat).Abs(new(big.Rat).Sub(normSquared, big.NewRat(1, 1)))
	// The planar LINE is defined for every parameter. Bound its lift over
	// both finite intervals when the model's axial endpoints were changed.
	domain := intervals[0][:]
	if trim != nil {
		domain = append(domain, intervals[1][:]...)
	}
	maxT := new(big.Rat)
	for _, endpoint := range domain {
		if t := new(big.Rat).Abs(endpoint); t.Cmp(maxT) > 0 {
			maxT = t
		}
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
	return FiniteConvexArcLineOwner{PCurveID: q.PCurveID, StartParameter: intervals[0][0], EndParameter: intervals[0][1], PCurveParameterScale: parameterScale}, squared, nil
}

// The rows retain the original and repaired directed parameters.
// A difference requires an independently proved axial trim for this edge.
func convexArcOwnerLineIntervals(src, corrected *File, q ConvexArcPCurveAgreement, model int64, tolSquared *big.Rat, trim *ConvexArcAxialTrimRepair) (p, d [3]*big.Rat, intervals [2][2]*big.Rat, err error) {
	originalPoint, originalDirection, e := convexUVLine(src, model, 3)
	if e != nil {
		return p, d, intervals, e
	}
	repairedPoint, repairedDirection, e := convexUVLine(corrected, model, 3)
	if e != nil {
		return p, d, intervals, e
	}
	p, d = [3]*big.Rat(repairedPoint), [3]*big.Rat(repairedDirection)
	if !convexUVSame([3]*big.Rat(originalDirection), d) {
		return p, d, intervals, geom.ErrDomain
	}
	var want [2][2]*big.Rat
	if trim != nil {
		if trim.EdgeID != q.EdgeID || trim.LineID != model {
			return p, d, intervals, geom.ErrDomain
		}
		want = [2][2]*big.Rat{trim.OriginalParameters, trim.RepairedParameters}
	}
	lsq := occtLoopDot(d, d)
	var vertices [2]int64
	for row, f := range []*File{src, corrected} {
		edge, e := simple(f, q.EdgeID, "EDGE_CURVE", 5)
		if e != nil {
			return p, d, intervals, e
		}
		point, line := p, model
		if row == 0 {
			point, line = [3]*big.Rat(originalPoint), q.SurfaceCurveID
		}
		if edge.Args[3].Ref != line {
			return p, d, intervals, geom.ErrDomain
		}
		for i := range vertices {
			id := edge.Args[i+1].Ref
			if row == 0 {
				vertices[i] = id
			}
			if id != vertices[i] || (trim != nil && id != trim.VertexIDs[i]) {
				return p, d, intervals, geom.ErrDomain
			}
			v, e := simple(f, id, "VERTEX_POINT", 2)
			if e != nil {
				return p, d, intervals, e
			}
			world, e := occtLoopTriple(f, v.Args[1].Ref, "CARTESIAN_POINT")
			if e != nil {
				return p, d, intervals, e
			}
			t, residual := projectConvexArcLine(world, point, d, lsq)
			if residual.Cmp(tolSquared) > 0 {
				return p, d, intervals, geom.ErrDomain
			}
			intervals[row][i] = t
			expected := intervals[0][i]
			if trim != nil {
				expected = want[row][i]
			}
			if t.Cmp(expected) != 0 {
				return p, d, intervals, geom.ErrDomain
			}
		}
		sense, e := boolean(edge.Args[4])
		if e != nil || !convexArcDirectedLineInterval(intervals[row], sense) {
			return p, d, intervals, geom.ErrDomain
		}
	}
	if intervals[0][1].Cmp(intervals[0][0]) != intervals[1][1].Cmp(intervals[1][0]) {
		return p, d, intervals, geom.ErrDomain
	}
	return p, d, intervals, nil
}
