package geom

import (
	"context"
	"math/big"
)

// PointSurfaceSupportOptions bounds exact directional hull separation. Zero
// selects 8 hull-witness updates, 2M operations and 16384 coefficient bits.
// Directions are optional represented covectors; normalization is unnecessary.
type PointSurfaceSupportOptions struct {
	Directions                                       []Vec3
	MaxIterations, MaxOperations, MaxCoefficientBits int
}

// PointSurfaceSupportBound is a lower bound on distance to the whole patch.
// HullWitness lies in its Euclidean control hull, not necessarily on the surface;
// it must never be used as a surface-distance upper witness. Complete means the
// requested finite search finished, not that the bound equals the true distance.
// Direction is absent if the initial AABB bound remains strongest.
// Valid bounds survive resource/cancellation errors when HasBound is true.
type PointSurfaceSupportBound struct {
	SquaredLower                                  *big.Rat
	SquaredLowerBound, LowerBound                 float64
	Direction, HullWitness                        [3]*big.Rat
	HasBound, Complete                            bool
	Iterations, DirectionsTested, ExactOperations int
}

// BoundPointSurfaceSupport uses exact control-hull support projections. For
// covector d and positive gap g separating the query from the hull, g²/(d·d)
// bounds squared Euclidean distance below. Automatic directions follow exact
// convex-hull witness updates; every resulting support inequality is rechecked
// against every rational control point. No floating nearest-hull solver is trusted.
func BoundPointSurfaceSupport(ctx context.Context, query Vec3, p *SurfacePatch, o PointSurfaceSupportOptions) (r PointSurfaceSupportBound, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || !query.IsFinite() {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxIterations == 0 {
		o.MaxIterations = 8
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxIterations < 1 || o.MaxIterations > 64 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || len(o.Directions) > 256 {
		return r, ErrInvalid
	}
	for _, d := range o.Directions {
		if !d.IsFinite() || (d.X == 0 && d.Y == 0 && d.Z == 0) {
			return r, ErrInvalid
		}
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	r.SquaredLower = new(big.Rat)
	r.HasBound = true
	defer func() {
		r.ExactOperations = budget.operations
		r.SquaredLowerBound = rationalFloatBound(r.SquaredLower, true)
		r.LowerBound = outwardSqrtRat(r.SquaredLower, true)
	}()
	q := [3]*big.Rat{new(big.Rat).SetFloat64(query.X), new(big.Rat).SetFloat64(query.Y), new(big.Rat).SetFloat64(query.Z)}
	lower, e := exactPointBoxDistance(query, p.bounds, budget)
	if e != nil {
		return r, e
	}
	r.SquaredLower.Set(lower)
	points := make([][3]*big.Rat, len(p.net))
	nearest := 0
	var bestDistance *big.Rat
	for i, h := range p.net {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		distance := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			points[i][axis] = new(big.Rat).Quo(h[axis], h[3])
			if e := budget.check(points[i][axis]); e != nil {
				return r, e
			}
			delta := new(big.Rat).Sub(points[i][axis], q[axis])
			distance.Add(distance, new(big.Rat).Mul(delta, delta))
			if e := budget.check(distance); e != nil {
				return r, e
			}
		}
		if bestDistance == nil || distance.Cmp(bestDistance) < 0 {
			nearest = i
			bestDistance = new(big.Rat).Set(distance)
		}
	}
	var witness [3]*big.Rat
	for i := 0; i < 3; i++ {
		witness[i] = new(big.Rat).Set(points[nearest][i])
	}
	evaluate := func(direction [3]*big.Rat) (int, error) {
		norm := new(big.Rat)
		for _, x := range direction {
			norm.Add(norm, new(big.Rat).Mul(x, x))
			if e := budget.check(norm); e != nil {
				return 0, e
			}
		}
		if norm.Sign() == 0 {
			return nearest, nil
		}
		var low, high *big.Rat
		lowest := 0
		for i, point := range points {
			if e := ctx.Err(); e != nil {
				return 0, e
			}
			value := new(big.Rat)
			for axis := 0; axis < 3; axis++ {
				value.Add(value, new(big.Rat).Mul(direction[axis], new(big.Rat).Sub(point[axis], q[axis])))
				if e := budget.check(value); e != nil {
					return 0, e
				}
			}
			if low == nil || value.Cmp(low) < 0 {
				low = new(big.Rat).Set(value)
				lowest = i
			}
			if high == nil || value.Cmp(high) > 0 {
				high = new(big.Rat).Set(value)
			}
		}
		r.DirectionsTested++
		gap := new(big.Rat)
		sign := int64(1)
		if low.Sign() > 0 {
			gap.Set(low)
		} else if high.Sign() < 0 {
			gap.Neg(high)
			sign = -1
		}
		candidate := new(big.Rat).Quo(new(big.Rat).Mul(gap, gap), norm)
		if e := budget.check(candidate); e != nil {
			return 0, e
		}
		if candidate.Cmp(r.SquaredLower) > 0 {
			r.SquaredLower.Set(candidate)
			for axis, x := range direction {
				r.Direction[axis] = new(big.Rat).Mul(big.NewRat(sign, 1), x)
			}
		}
		return lowest, nil
	}
	for _, d := range o.Directions {
		if _, e := evaluate([3]*big.Rat{new(big.Rat).SetFloat64(d.X), new(big.Rat).SetFloat64(d.Y), new(big.Rat).SetFloat64(d.Z)}); e != nil {
			return r, e
		}
	}
	for iteration := 0; iteration <= o.MaxIterations; iteration++ {
		var direction [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			direction[axis] = new(big.Rat).Sub(witness[axis], q[axis])
			r.HullWitness[axis] = new(big.Rat).Set(witness[axis])
		}
		index, e := evaluate(direction)
		if e != nil {
			return r, e
		}
		if iteration == o.MaxIterations {
			break
		}
		numerator, denominator := new(big.Rat), new(big.Rat)
		var step [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			step[axis] = new(big.Rat).Sub(points[index][axis], witness[axis])
			numerator.Sub(numerator, new(big.Rat).Mul(direction[axis], step[axis]))
			denominator.Add(denominator, new(big.Rat).Mul(step[axis], step[axis]))
			if e := budget.check(numerator); e != nil {
				return r, e
			}
			if e := budget.check(denominator); e != nil {
				return r, e
			}
		}
		if denominator.Sign() == 0 || numerator.Sign() <= 0 {
			break
		}
		fraction := new(big.Rat).Quo(numerator, denominator)
		if fraction.Cmp(big.NewRat(1, 1)) > 0 {
			fraction.SetInt64(1)
		}
		if e := budget.check(fraction); e != nil {
			return r, e
		}
		for axis := 0; axis < 3; axis++ {
			witness[axis].Add(witness[axis], new(big.Rat).Mul(fraction, step[axis]))
			if e := budget.check(witness[axis]); e != nil {
				return r, e
			}
		}
		r.Iterations++
	}
	r.Complete = true
	return r, nil
}
