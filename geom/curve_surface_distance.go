package geom

import (
	"context"
	"math"
)

// CurveSurfaceDistanceOptions shares the surface-pair work limits and absolute
// squared-gap tolerance. The constant auxiliary curve row counts toward queue
// limits. Zero values use SurfacePairDistanceOptions defaults.
type CurveSurfaceDistanceOptions = SurfacePairDistanceOptions

// CurveSurfaceDistanceBounds encloses global squared distance from a complete
// curve to a bounded surface. Witness coordinates/boxes describe actual points
// when HasWitness. Complete concerns exact internal gap before outward rounding.
type CurveSurfaceDistanceBounds struct {
	SquaredDistanceMin, SquaredDistanceMax, CurveParameter float64
	SurfaceUV                                              Vec2
	CurvePoint, SurfacePoint                               Box
	HasWitness, Complete                                   bool
	VisitedPairs, ExactOperations                          int
}

// BoundCurveSurfaceDistance uses exact branch-and-bound control hulls and rational
// witnesses. The curve must be continuous, clamped and positive-weight. Only its
// actual parameter is subdivided; both surface parameters remain active. Bounds
// survive resource exhaustion, possibly [0,+Inf] before the first witness.
func BoundCurveSurfaceDistance(curve *BSpline, surface *SurfacePatch, o CurveSurfaceDistanceOptions) (result CurveSurfaceDistanceBounds, err error) {
	return BoundCurveSurfaceDistanceContext(context.Background(), curve, surface, o)
}

// BoundCurveSurfaceDistanceContext preserves established bounds on cooperative cancellation.
// Exact arithmetic within one bounded search step is not interrupted.
func BoundCurveSurfaceDistanceContext(ctx context.Context, curve *BSpline, surface *SurfacePatch, o CurveSurfaceDistanceOptions) (result CurveSurfaceDistanceBounds, err error) {
	result.SquaredDistanceMax = math.Inf(1)
	if ctx == nil {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if curve == nil || len(curve.poles) == 0 || surface == nil || len(surface.net) == 0 {
		return result, ErrInvalid
	}
	result.SquaredDistanceMax = math.Inf(1)
	operations, bits, queued := o.MaxOperations, o.MaxCoefficientBits, o.MaxQueuedPoles
	if operations == 0 {
		operations = 4000000
	}
	if bits == 0 {
		bits = 16384
	}
	if queued == 0 {
		queued = 524288
	}
	if operations < 1 || bits < 64 || queued < 8 {
		return result, ErrInvalid
	}
	if len(surface.net) > queued || len(curve.poles) > (queued-len(surface.net))/2 {
		return result, ErrConvergence
	}
	patch, e := curveDistancePatch(curve, operations, bits)
	if e != nil {
		return result, e
	}
	result.ExactOperations = patch.ExactOperations()
	remaining := operations - result.ExactOperations
	if remaining < 1 {
		return result, ErrConvergence
	}
	options := o
	options.MaxOperations, options.MaxCoefficientBits, options.MaxQueuedPoles = remaining, bits, queued
	if e := ctx.Err(); e != nil {
		return result, e
	}
	bound, e := boundSurfacePairDistanceContext(ctx, patch, surface, options, true, false)
	result.SquaredDistanceMin, result.SquaredDistanceMax = bound.SquaredDistanceMin, bound.SquaredDistanceMax
	result.CurveParameter, result.SurfaceUV = bound.UVA.X, bound.UVB
	result.CurvePoint, result.SurfacePoint = bound.PointA, bound.PointB
	result.HasWitness, result.Complete = bound.HasWitness, bound.Complete
	result.VisitedPairs = bound.VisitedPairs
	result.ExactOperations += bound.ExactOperations
	return result, e
}
