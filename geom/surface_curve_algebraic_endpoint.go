package geom

import (
	"context"
	"math"
	"math/big"
)

// ComposedCurveEndpointOptions bounds algebraic endpoint refinement. Tolerance
// defaults to 1e-9 and bounds the total Euclidean representative error, including
// float64 rounding. Smaller tolerances than representability permits can remain
// unresolved even for rational endpoints.
type ComposedCurveEndpointOptions struct {
	ExactCurvePlaneEventOptions
	Tolerance float64
}

// AlgebraicCurveEndpointBounds retains conservative bounds even when refinement
// stops at a budget or cancellation. DerivativeBounds/SpeedUpper apply over the
// entire returned parameter bracket to this cell's rational continuation, in
// original global parameter units. They do not claim cross-knot smoothness.
type AlgebraicCurveEndpointBounds struct {
	HasBounds, HasRepresentative, Converged bool
	Parameter                               ExactParameterInterval
	Coordinates                             [3]ExactParameterInterval
	Bounds, DerivativeBounds                Box
	Point                                   Vec3
	ErrorUpper, SpeedUpper                  float64
	ExactOperations, Subdivisions           int
}

// EncloseEndpoint narrows one private algebraic domain endpoint (0=start,1=end).
// Homogeneous interval evaluation proves model and derivative ranges once the
// denominator is strictly positive. The represented midpoint's error includes
// exact roundtrip displacement, not just an unrounded interval radius.
func (c *ExactComposedCurveCell) EncloseEndpoint(ctx context.Context, endpoint int, o ComposedCurveEndpointOptions) (r AlgebraicCurveEndpointBounds, err error) {
	if ctx == nil || c == nil || endpoint < 0 || endpoint > 1 || c.endpoints[endpoint] == nil || len(c.power[3]) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-9
	}
	if !finite(o.Tolerance) || o.Tolerance <= 0 {
		return r, ErrInvalid
	}
	options, e := graphRootOptions(o.ExactCurvePlaneEventOptions)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	root := c.endpoints[endpoint]
	for _, x := range root.bounds {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	for _, polynomial := range c.power {
		for _, x := range polynomial {
			if e := b.check(x); e != nil {
				return r, e
			}
		}
	}
	tolerance := new(big.Rat).SetFloat64(o.Tolerance)
	toleranceSquared := new(big.Rat).Mul(tolerance, tolerance)
	var derivative [3]rationalPolynomial
	wd := polynomialDerivative(c.power[3])
	for axis := 0; axis < 3; axis++ {
		left, e := polynomialProduct(polynomialDerivative(c.power[axis]), c.power[3], b)
		if e != nil {
			return r, e
		}
		right, e := polynomialProduct(c.power[axis], wd, b)
		if e != nil {
			return r, e
		}
		derivative[axis], e = subtractScaledPolynomial(left, right, big.NewRat(1, 1), b)
		if e != nil {
			return r, e
		}
	}
	var bestError *big.Rat
	steps, e := isolatePolynomialRoots(root.polynomial, root.bounds[0], root.bounds[1], b, options.MaxSubdivisions, func(lo, hi *big.Rat, _ int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		parameter := exactInterval{lo: lo, hi: hi}
		denominator, e := polynomialInterval(c.power[3], parameter, b)
		if e != nil {
			return false, e
		}
		if denominator.lo.Sign() <= 0 {
			return false, nil
		}
		square, e := intervalMultiply(denominator, denominator, b)
		if e != nil {
			return false, e
		}
		var position, velocity [3]exactInterval
		var point [3]float64
		represented := true
		errorSquared, speedSquared := new(big.Rat), new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			n, e := polynomialInterval(c.power[axis], parameter, b)
			if e != nil {
				return false, e
			}
			position[axis], e = intervalDivide(n, denominator, b)
			if e != nil {
				return false, e
			}
			n, e = polynomialInterval(derivative[axis], parameter, b)
			if e != nil {
				return false, e
			}
			velocity[axis], e = intervalDivide(n, square, b)
			if e != nil {
				return false, e
			}
			midpoint := new(big.Rat).Quo(new(big.Rat).Add(position[axis].lo, position[axis].hi), big.NewRat(2, 1))
			point[axis], _ = midpoint.Float64()
			if !finite(point[axis]) {
				represented = false
			} else {
				rounded := new(big.Rat).SetFloat64(point[axis])
				deltaLo := new(big.Rat).Abs(new(big.Rat).Sub(rounded, position[axis].lo))
				deltaHi := new(big.Rat).Abs(new(big.Rat).Sub(rounded, position[axis].hi))
				if deltaHi.Cmp(deltaLo) > 0 {
					deltaLo = deltaHi
				}
				errorSquared.Add(errorSquared, new(big.Rat).Mul(deltaLo, deltaLo))
			}
			maximum := new(big.Rat).Abs(velocity[axis].lo)
			other := new(big.Rat).Abs(velocity[axis].hi)
			if other.Cmp(maximum) > 0 {
				maximum = other
			}
			speedSquared.Add(speedSquared, new(big.Rat).Mul(maximum, maximum))
			for _, x := range []*big.Rat{errorSquared, speedSquared} {
				if e := b.check(x); e != nil {
					return false, e
				}
			}
		}
		converged := represented && errorSquared.Cmp(toleranceSquared) <= 0
		if !r.HasBounds || represented && (!r.HasRepresentative || bestError == nil || errorSquared.Cmp(bestError) < 0) || converged {
			r.HasBounds = true
			r.HasRepresentative = represented
			r.Converged = converged
			r.Parameter = ExactParameterInterval{Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi)}
			box := func(values [3]exactInterval) Box {
				return Box{Min: Vec3{rationalFloatBound(values[0].lo, true), rationalFloatBound(values[1].lo, true), rationalFloatBound(values[2].lo, true)}, Max: Vec3{rationalFloatBound(values[0].hi, false), rationalFloatBound(values[1].hi, false), rationalFloatBound(values[2].hi, false)}}
			}
			r.Bounds = box(position)
			r.DerivativeBounds = box(velocity)
			r.SpeedUpper = outwardSqrtRat(speedSquared, false)
			for axis := 0; axis < 3; axis++ {
				r.Coordinates[axis] = ExactParameterInterval{Lower: new(big.Rat).Set(position[axis].lo), Upper: new(big.Rat).Set(position[axis].hi)}
			}
			if represented {
				r.Point = Vec3{point[0], point[1], point[2]}
				r.ErrorUpper = outwardSqrtRat(errorSquared, false)
				bestError = new(big.Rat).Set(errorSquared)
			} else {
				r.ErrorUpper = math.Inf(1)
			}
		}
		return converged, nil
	})
	r.Subdivisions = steps
	if e != nil {
		return r, e
	}
	if !r.Converged {
		return r, ErrConvergence
	}
	return r, nil
}
