package geom

import (
	"context"
	"math"
	"math/big"
)

// CurvePlaneExtrema bounds the extrema of normal dot (C-origin). Witnesses
// are represented curve parameters evaluated exactly. DistanceLower/Upper bound
// the Euclidean minimum distance to the plane, dividing by the exact norm of
// its stored normal. A certified crossing gives exactly zero distance by
// continuity, even if its parameter is irrational. These are adaptive hull
// bounds, not a stationary-root enumeration. Complete refers to the functional
// extrema tolerance; a subsequent distance-arithmetic limit can still return
// an error with the default conservative distance interval [0,+Inf].
type CurvePlaneExtrema struct {
	Minimum, Maximum               CertifiedScalarRange
	MinimumWitness, MaximumWitness float64
	DistanceLower, DistanceUpper   float64
	StrictSign                     int
	Complete                       bool
	Cells, ExactOperations         int
}

// BoundCurvePlaneRange uses SurfacePlaneRangeOptions for the functional extrema
// gap (not a distance gap). Only continuous clamped positive-weight splines are
// supported. On cancellation or limits valid established bounds survive.
func BoundCurvePlaneRange(c *BSpline, plane Plane, o SurfacePlaneRangeOptions) (CurvePlaneExtrema, error) {
	return BoundCurvePlaneRangeContext(context.Background(), c, plane, o)
}

// BoundCurvePlaneRangeContext checks cancellation between bounded exact steps.
func BoundCurvePlaneRangeContext(ctx context.Context, c *BSpline, plane Plane, o SurfacePlaneRangeOptions) (r CurvePlaneExtrema, err error) {
	r.Minimum = CertifiedScalarRange{Min: math.Inf(-1), Max: math.Inf(1)}
	r.Maximum = r.Minimum
	r.DistanceUpper = math.Inf(1)
	if ctx == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	operations, bits := o.MaxOperations, o.MaxCoefficientBits
	if operations == 0 {
		operations = 4000000
	}
	if bits == 0 {
		bits = 16384
	}
	if operations < 1 || bits < 64 {
		return r, ErrInvalid
	}
	p, e := curveDistancePatch(c, operations, bits)
	if e != nil {
		return r, e
	}
	r.ExactOperations = p.operations
	remaining := operations - p.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	o.MaxOperations = remaining
	s, e := boundSurfacePlaneRangeContext(ctx, p, plane, o, true)
	r.Minimum, r.Maximum = s.Minimum, s.Maximum
	r.MinimumWitness, r.MaximumWitness = s.MinimumWitness.X, s.MaximumWitness.X
	r.StrictSign, r.Complete, r.Cells, r.ExactOperations = s.StrictSign, s.Complete, s.Cells, p.operations+s.ExactOperations
	if !finite(s.Minimum.Min) || !finite(s.Minimum.Max) || !finite(s.Maximum.Min) || !finite(s.Maximum.Max) {
		return r, e
	}
	// The image is an interval [m,M]. Its distance from zero is monotone in
	// each endpoint; evaluate both enclosing endpoint pairs exactly.
	low, high := 0.0, 0.0
	if s.Minimum.Min > 0 {
		low = s.Minimum.Min
	} else if s.Maximum.Max < 0 {
		low = -s.Maximum.Max
	}
	if s.Minimum.Max > 0 {
		high = s.Minimum.Max
	}
	if -s.Maximum.Min > high {
		high = -s.Maximum.Min
	}
	budget := &polynomialBudget{maxOperations: operations - r.ExactOperations, maxBits: bits}
	defer func() { r.ExactOperations += budget.operations }()
	norm := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		n := new(big.Rat).SetFloat64(coordinate(plane.normal, axis))
		norm.Add(norm, new(big.Rat).Mul(n, n))
		if failure := budget.check(norm); failure != nil {
			return r, failure
		}
	}
	if norm.Sign() == 0 {
		return r, ErrInvalid
	}
	distance := func(value float64, lower bool) (float64, error) {
		x := new(big.Rat).SetFloat64(value)
		x.Mul(x, x)
		x.Quo(x, norm)
		if failure := budget.check(x); failure != nil {
			return 0, failure
		}
		return outwardSqrtRat(x, lower), nil
	}
	lower, failure := distance(low, true)
	if failure != nil {
		return r, failure
	}
	upper, failure := distance(high, false)
	if failure != nil {
		return r, failure
	}
	r.DistanceLower, r.DistanceUpper = lower, upper
	return r, e
}
