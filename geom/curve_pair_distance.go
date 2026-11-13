package geom

import (
	"context"
	"math"
)

// CurvePairDistanceOptions bounds exact curve-pair distance search. Zero fields
// select squared tolerance 1e-6, depth 32, 8192 pairs, 524288 queued poles,
// 4M operations and 16384 coefficient bits. Queue counts include duplicated
// auxiliary rows. Tolerance is an absolute squared-distance gap.
type CurvePairDistanceOptions struct {
	SquaredTolerance                                                      float64
	MaxDepth, MaxPairs, MaxQueuedPoles, MaxOperations, MaxCoefficientBits int
}

// CurvePairDistanceBounds encloses the exact global squared distance. Parameters
// and point boxes supply an actual pair witness when HasWitness. Complete means
// the exact internal gap meets tolerance, before outward float conversion.
// Equal or multiple minima require no uniqueness assumption.
type CurvePairDistanceBounds struct {
	SquaredDistanceMin, SquaredDistanceMax, ParameterA, ParameterB float64
	PointA, PointB                                                 Box
	HasWitness, Complete                                           bool
	VisitedPairs, ExactOperations                                  int
}

func curveDistancePatch(curve *BSpline, maxOperations, maxBits int) (*SurfacePatch, error) {
	poles := make([]Vec3, 2*len(curve.poles))
	weights := make([]float64, len(poles))
	for i, p := range curve.poles {
		poles[2*i], poles[2*i+1] = p, p
		weights[2*i], weights[2*i+1] = curve.weights[i], curve.weights[i]
	}
	surface := &BSplineSurface{degreeU: curve.degree, degreeV: 1, countU: len(curve.poles), countV: 2, poles: poles, weights: weights, knotsU: append([]float64(nil), curve.knots...), knotsV: []float64{0, 0, 1, 1}}
	return NewSurfacePatch(surface, SurfacePatchOptions{MaxPoles: max(4096, len(poles)+2*(curve.degree+1)), MaxOperations: maxOperations, MaxCoefficientBits: maxBits})
}

// BoundCurvePairDistance certifies distance over two complete continuous clamped
// positive-weight rational curves. The exact surface-pair engine uses constant
// auxiliary coordinates and only splits the two actual curve parameters.
// Bounds combine control boxes and exact separating projections, with rational
// center/endpoint witnesses. Unclamped or discontinuous curves are unsupported.
func BoundCurvePairDistance(a, b *BSpline, o CurvePairDistanceOptions) (result CurvePairDistanceBounds, err error) {
	return BoundCurvePairDistanceContext(context.Background(), a, b, o)
}

// BoundCurvePairDistanceContext preserves established bounds on cooperative cancellation.
// Exact arithmetic within one bounded search step is not interrupted.
func BoundCurvePairDistanceContext(ctx context.Context, a, b *BSpline, o CurvePairDistanceOptions) (result CurvePairDistanceBounds, err error) {
	result.SquaredDistanceMax = math.Inf(1)
	if ctx == nil {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if a == nil || b == nil || len(a.poles) == 0 || len(b.poles) == 0 {
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
	if len(a.poles) > queued/2 || len(b.poles) > queued/2-len(a.poles) {
		return result, ErrConvergence
	}
	pa, e := curveDistancePatch(a, operations, bits)
	if e != nil {
		return result, e
	}
	result.ExactOperations = pa.ExactOperations()
	remaining := operations - result.ExactOperations
	if remaining < 1 {
		return result, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	pb, e := curveDistancePatch(b, remaining, bits)
	if e != nil {
		return result, e
	}
	result.ExactOperations += pb.ExactOperations()
	remaining = operations - result.ExactOperations
	if remaining < 1 {
		return result, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	bound, e := boundSurfacePairDistanceContext(ctx, pa, pb, SurfacePairDistanceOptions{SquaredTolerance: o.SquaredTolerance, MaxDepth: o.MaxDepth, MaxPairs: o.MaxPairs, MaxQueuedPoles: queued, MaxOperations: remaining, MaxCoefficientBits: bits}, true, true)
	result.SquaredDistanceMin, result.SquaredDistanceMax = bound.SquaredDistanceMin, bound.SquaredDistanceMax
	result.ParameterA, result.ParameterB = bound.UVA.X, bound.UVB.X
	result.PointA, result.PointB = bound.PointA, bound.PointB
	result.HasWitness, result.Complete = bound.HasWitness, bound.Complete
	result.VisitedPairs = bound.VisitedPairs
	result.ExactOperations += bound.ExactOperations
	return result, e
}
