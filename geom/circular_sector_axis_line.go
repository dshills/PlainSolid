package geom

import (
	"context"
	"math/big"
)

// CircularSectorAxisInterval is the complete closed material intersection of
// a finite line with a sector. Parameters and endpoint coordinates are owned
// exact rationals. Empty has no endpoints. Singleton denotes a point contact,
// not an interior interval. OnAxis and AlongBoundary distinguish lines on the
// axis or radial walls from lines through the material interior.
type CircularSectorAxisInterval struct {
	Complete, Empty, Singleton, OnAxis, AlongBoundary bool
	Parameters                                        [2]*big.Rat
	Points                                            [2][3]*big.Rat
	EndpointBoundary                                  [2]CircularSectorBoundary
	InteriorBoundary                                  CircularSectorBoundary
	ExactOperations                                   int
}

// IntersectCircularSectorAxisLine intersects p(t)=origin+t*axisScale*sector.axis
// with both the closed sector and the caller's finite closed parameter range.
// axisScale is a signed model-distance scale, not a normalized line parameter.
// Tangent radial placement produces a boundary interval rather than a false
// isolated hit. It does not support directions transverse to the rotation axis.
func IntersectCircularSectorAxisLine(ctx context.Context, s CircularSector, origin Vec3, axisScale float64, parameterRange [2]float64, o SurfacePatchEvaluationOptions) (out CircularSectorAxisInterval, err error) {
	if ctx == nil || !origin.IsFinite() || !finite(axisScale) || axisScale == 0 || !finite(parameterRange[0]) || !finite(parameterRange[1]) || parameterRange[0] > parameterRange[1] {
		return out, ErrInvalid
	}
	// Probe at the lower cap without rounding an axial coordinate replacement:
	// choose represented origin at axial zero (subtracting stored floats is not
	// exact), and classify radial/angular predicates at sector.origin axial value.
	axis := 0
	if s.axis.Y != 0 {
		axis = 1
	} else if s.axis.Z != 0 {
		axis = 2
	}
	probe := origin
	switch axis {
	case 0:
		probe.X = s.origin.X
	case 1:
		probe.Y = s.origin.Y
	case 2:
		probe.Z = s.origin.Z
	}
	radialSector := s
	radialSector.lo = -1
	radialSector.hi = 1
	q, e := ClassifyCircularSectorPoint(ctx, radialSector, probe, o)
	out.ExactOperations = q.ExactOperations
	if e != nil {
		return out, e
	}
	if q.Location == SectorOutside {
		out.Complete = true
		out.Empty = true
		return out, nil
	}
	maxOps := o.MaxOperations
	if maxOps == 0 {
		maxOps = 10000
	}
	maxBits := o.MaxCoefficientBits
	if maxBits == 0 {
		maxBits = 16384
	}
	b := &polynomialBudget{maxOperations: maxOps, maxBits: maxBits, operations: q.ExactOperations}
	defer func() { out.ExactOperations = b.operations }()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sign := coordinate(s.axis, axis)
	z := new(big.Rat).Mul(new(big.Rat).Sub(r(coordinate(origin, axis)), r(coordinate(s.origin, axis))), r(sign))
	scale := r(axisScale)
	a := new(big.Rat).Quo(new(big.Rat).Sub(r(s.lo), z), scale)
	c := new(big.Rat).Quo(new(big.Rat).Sub(r(s.hi), z), scale)
	if a.Cmp(c) > 0 {
		a, c = c, a
	}
	if a.Cmp(r(parameterRange[0])) < 0 {
		a = r(parameterRange[0])
	}
	if c.Cmp(r(parameterRange[1])) > 0 {
		c = r(parameterRange[1])
	}
	for _, v := range []*big.Rat{z, scale, a, c} {
		if e = b.check(v); e != nil {
			return out, e
		}
	}
	if a.Cmp(c) > 0 {
		out.Complete = true
		out.Empty = true
		return out, nil
	}
	out.Parameters = [2]*big.Rat{a, c}
	out.Singleton = a.Cmp(c) == 0
	out.InteriorBoundary = q.Boundary
	out.OnAxis = q.Boundary&SectorAxis != 0
	out.AlongBoundary = q.Boundary != 0
	for i, t := range out.Parameters {
		height := new(big.Rat).Add(z, new(big.Rat).Mul(t, scale))
		out.EndpointBoundary[i] = q.Boundary
		if height.Cmp(r(s.lo)) == 0 {
			out.EndpointBoundary[i] |= SectorLowerCap
		}
		if height.Cmp(r(s.hi)) == 0 {
			out.EndpointBoundary[i] |= SectorUpperCap
		}
		for k := 0; k < 3; k++ {
			v := new(big.Rat).Add(r(coordinate(origin, k)), new(big.Rat).Mul(new(big.Rat).Mul(t, scale), r(coordinate(s.axis, k))))
			if e = b.check(v); e != nil {
				return CircularSectorAxisInterval{ExactOperations: b.operations}, e
			}
			out.Points[i][k] = v
		}
	}
	out.Complete = true
	return out, nil
}
