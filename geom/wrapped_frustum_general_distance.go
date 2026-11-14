package geom

import (
	"context"
	"math/big"
)

// BoundPointWrappedFrustumDistance encloses squared distance to all boundary
// pieces of a canonical positive rational frustum, without circular reduction.
// A certified exact support-patch search covers the lateral wall. Each cap
// uses exact conic disk membership and either an exact orthogonal projection
// or an exact rational rim-curve distance search. Returned upper distance is
// recomputed from an actual rational boundary witness. Each of at most three
// searches has the supplied work limits; proof/extraction have the corresponding
// arithmetic limits. Failure returns no partial result. Uniqueness is not proved.
func BoundPointWrappedFrustumDistance(ctx context.Context, w *WrappedSurfacePatch, q Vec3, o SurfaceDistanceOptions) (CylinderDistanceBounds, error) {
	if !q.IsFinite() {
		return CylinderDistanceBounds{}, ErrInvalid
	}
	return BoundPointWrappedFrustumDistanceRational(ctx, w, rationalLineVec3(q), o)
}

// BoundPointWrappedFrustumDistanceRational retains rational cap projections,
// rim and original support witnesses even for nonbinary query coordinates.
func BoundPointWrappedFrustumDistanceRational(ctx context.Context, w *WrappedSurfacePatch, query [3]*big.Rat, o SurfaceDistanceOptions) (CylinderDistanceBounds, error) {
	empty := CylinderDistanceBounds{}
	po := SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	options, e := validateRationalPoint(ctx, query, po)
	if e != nil {
		return empty, e
	}
	proof, e := CertifyWrappedFrustumEmbedding(ctx, w, po)
	if e != nil {
		return empty, e
	}
	budget := &polynomialBudget{operations: proof.ExactOperations, maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	result := CylinderDistanceBounds{}
	add := func(lower *big.Rat, point [3]*big.Rat, part CylinderBoundaryPart) error {
		upper := new(big.Rat)
		for k := range point {
			d := new(big.Rat).Sub(point[k], query[k])
			upper.Add(upper, new(big.Rat).Mul(d, d))
			if e := budget.check(upper); e != nil {
				return e
			}
		}
		if result.Lower == nil || lower.Cmp(result.Lower) < 0 {
			result.Lower = new(big.Rat).Set(lower)
		}
		if result.Upper == nil || upper.Cmp(result.Upper) < 0 {
			result.Upper = upper
			result.Part = part
			for k := range point {
				result.Witness[k] = new(big.Rat).Set(point[k])
			}
		}
		return nil
	}
	inside := func(radius *big.Rat) (bool, error) {
		u := new(big.Rat).Quo(new(big.Rat).Sub(query[0], proof.CenterX), radius)
		v := new(big.Rat).Quo(new(big.Rat).Sub(query[1], proof.CenterY), radius)
		j := 0
		if u.Sign() < 0 {
			if v.Sign() >= 0 {
				j = 1
				u, v = new(big.Rat).Set(v), new(big.Rat).Neg(u)
			} else {
				j = 2
				u.Neg(u)
				v.Neg(v)
			}
		} else if v.Sign() < 0 {
			j = 3
			u, v = new(big.Rat).Neg(v), new(big.Rat).Set(u)
		}
		for _, r := range []*big.Rat{u, v} {
			if e := budget.check(r); e != nil {
				return false, e
			}
		}
		one := big.NewRat(1, 1)
		if u.Cmp(one) > 0 || v.Cmp(one) > 0 {
			return false, nil
		}
		sum := new(big.Rat).Sub(new(big.Rat).Add(u, v), one)
		if sum.Sign() < 0 {
			return true, nil
		}
		a, b, c := w.patch.net[4*j][3], w.patch.net[4*j+2][3], w.patch.net[4*j+4][3]
		left := new(big.Rat).Mul(new(big.Rat).Mul(a, c), new(big.Rat).Mul(sum, sum))
		right := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(b, b))
		right.Mul(right, new(big.Rat).Sub(one, u))
		right.Mul(right, new(big.Rat).Sub(one, v))
		for _, r := range []*big.Rat{left, right} {
			if e := budget.check(r); e != nil {
				return false, e
			}
		}
		return left.Cmp(right) <= 0, nil
	}
	// Outside the height slab, a valid orthogonal cap projection attains the
	// universal vertical-gap lower bound, so no lateral/rim search is needed.
	for i, z := range []*big.Rat{proof.LowerZ, proof.UpperZ} {
		beyond := query[2].Cmp(z) <= 0
		if i == 1 {
			beyond = query[2].Cmp(z) >= 0
		}
		if !beyond {
			continue
		}
		radius, part := proof.LowerRadius, CylinderLowerCap
		if i == 1 {
			radius, part = proof.UpperRadius, CylinderUpperCap
		}
		in, e := inside(radius)
		if e != nil {
			return empty, e
		}
		if in {
			d := new(big.Rat).Sub(query[2], z)
			distance := new(big.Rat).Mul(d, d)
			if e := add(distance, [3]*big.Rat{query[0], query[1], z}, part); e != nil {
				return empty, e
			}
			return result, nil
		}
	}
	domain := w.patch.Domain()
	for i, z := range []*big.Rat{proof.LowerZ, proof.UpperZ} {
		radius, parameter, part := proof.LowerRadius, domain.VMin, CylinderLowerCap
		if i == 1 {
			radius, parameter, part = proof.UpperRadius, domain.VMax, CylinderUpperCap
		}
		in, e := inside(radius)
		if e != nil {
			return empty, e
		}
		if in {
			point := [3]*big.Rat{query[0], query[1], z}
			d := new(big.Rat).Sub(query[2], z)
			lower := new(big.Rat).Mul(d, d)
			if e := add(lower, point, part); e != nil {
				return empty, e
			}
			if result.Upper.Sign() == 0 {
				return result, nil
			}
			continue
		}
		rim, e := w.patch.IsoCurve(1, parameter, po)
		if e != nil {
			return empty, e
		}
		bound, e := BoundPointExactCurveDistanceRational(ctx, query, rim, CurveDistanceOptions(o))
		if e != nil {
			return empty, e
		}
		if !bound.Complete || !bound.HasWitness || bound.Squared.Lower == nil {
			return empty, ErrConvergence
		}
		result.VisitedCells += bound.VisitedCells
		if e := add(bound.Squared.Lower, bound.Witness, part); e != nil {
			return empty, e
		}

	}
	side, e := BoundPointSurfaceDistanceRational(ctx, query, w.patch, o)
	if e != nil {
		return empty, e
	}
	if !side.Complete || !side.HasUpperPoint || side.Squared.Lower == nil {
		return empty, ErrConvergence
	}
	result.VisitedCells += side.VisitedCells
	if e := add(side.Squared.Lower, side.Witness, CylinderLateral); e != nil {
		return empty, e
	}
	return result, ctx.Err()
}
