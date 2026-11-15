package geom

import (
	"context"
	"math/big"
)

// SphereLensDilationPoint describes exact membership in a dilated original
// spherical lens. Owners names the original cap or pair of rim owners when
// Location is BoreBoundary. SquaredDistance encloses distance squared to the
// original lens, not distance to the new boundary.
type SphereLensDilationPoint struct {
	Location        BorePointLocation
	Owners          BoreBoundaryMask
	SquaredDistance [2]*big.Rat
	Operations      int
}

// ClassifyDilatedLensRational classifies a rational point in the convex parallel
// body of a transverse sphere intersection. Displacement is nonnegative.
// Outside the original lens, the distance to that lens equals the distance to
// its boundary. The original cap/rim proof supplies an exact signed radical
// comparison against the displacement squared. Irrational radii of the rim,
// arbitrary source axis directions and on-axis minimizers are supported. Bounds
// are diagnostic; neither a tolerance nor their overlap decides membership.
func (s SpherePairMaterial) ClassifyDilatedLensRational(ctx context.Context, p [3]*big.Rat, displacement *big.Rat, o SphereFullBoundaryDistanceOptions) (SphereLensDilationPoint, error) {
	var zero SphereLensDilationPoint
	if ctx == nil || s.radius2[0] == nil || displacement == nil || displacement.Sign() < 0 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if !s.transverse || s.op != SphereSetIntersection {
		return zero, ErrDomain
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil || (o.SqrtBits != 0 && (o.SqrtBits < 8 || o.SqrtBits > 256)) {
		return zero, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := sphereOverlapArithmetic{ctx: ctx, b: b}
	a.check(displacement)
	for _, value := range p {
		if value == nil {
			return zero, ErrInvalid
		}
		a.check(value)
	}
	var signs [2]int
	for i, c := range s.center {
		for _, v := range c {
			a.check(v)
		}
		a.check(s.radius2[i])
		signs[i] = a.sub(a.distance(p, c), s.radius2[i]).Sign()
	}
	if a.err != nil {
		return zero, a.err
	}
	location, owners := s.location(signs[:])
	if location != BoreOutside {
		if displacement.Sign() > 0 {
			location, owners = BoreInside, 0
		}
		return SphereLensDilationPoint{location, owners, [2]*big.Rat{new(big.Rat), new(big.Rat)}, b.operations}, nil
	}
	radius2 := a.mul(displacement, displacement)
	if a.err != nil {
		return zero, a.err
	}
	nearest, e := s.nearestBoundaryRational(ctx, p, o, b)
	if e != nil {
		return zero, e
	}
	var constant, coefficient, radicand *big.Rat
	switch nearest.Kind {
	case SphereNearestTransverseIsolated:
		for _, c := range nearest.Isolated.Candidates {
			if c.Nearest {
				constant, coefficient, radicand = c.Constant, c.Coefficient, c.Radicand
				break
			}
		}
	case SphereNearestTransverseAxis:
		for _, f := range nearest.Axis.Families {
			if f.Nearest {
				constant, coefficient, radicand = f.Constant, f.Coefficient, f.Radicand
				break
			}
		}
	}
	if constant == nil || coefficient == nil || radicand == nil {
		return zero, ErrConvergence
	}
	difference := a.sub(constant, radius2)
	if a.err != nil {
		return zero, a.err
	}
	sign, e := sphereCavityRadicalSign(difference, coefficient, radicand, b)
	if e != nil {
		return zero, e
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	location, owners = BoreOutside, 0
	if sign < 0 {
		location = BoreInside
	} else if sign == 0 {
		location, owners = BoreBoundary, nearest.Owners
	}
	return SphereLensDilationPoint{location, owners, [2]*big.Rat{nearest.Lower, nearest.Upper}, b.operations}, nil
}
