package geom

import (
	"context"
	"math"
)

// CurveDistanceOptions bounds certified global squared distance. Zero values
// select squared tolerance 1e-8, depth 32, 8192 cells, 262144 queued poles,
// 4M exact operations and 16384 bits. Queue accounting includes the duplicated
// constant-direction control row used internally by the exact patch engine.
type CurveDistanceOptions struct {
	SquaredTolerance                                                      float64
	MaxDepth, MaxCells, MaxQueuedPoles, MaxOperations, MaxCoefficientBits int
}

// CurveDistanceBounds encloses global squared distance to the complete curve.
// Parameter and PointBounds identify an exact rational surface/curve sample
// supplying the upper bound when HasWitness. Complete concerns the exact
// internal bound gap; outward float conversion may widen the displayed gap.
// A resource error retains valid bounds, possibly [0,+Inf] before a witness.
type CurveDistanceBounds struct {
	SquaredDistanceMin, SquaredDistanceMax, Parameter float64
	PointBounds                                       Box
	HasWitness, Complete                              bool
	VisitedCells, ExactOperations                     int
}

// BoundPointExactCurveDistance reuses the bounded rational distance search
// on an immutable exact spline descriptor, including a surface iso-curve.
// No control point conversion or weight renormalization is performed.
func BoundPointExactCurveDistance(ctx context.Context, query Vec3, curve *ExactSplineCurve, o CurveDistanceOptions) (r CurveDistanceBounds, err error) {
	r.SquaredDistanceMax = math.Inf(1)
	if ctx == nil || curve == nil || curve.patch == nil || !query.IsFinite() {
		return r, ErrInvalid
	}
	q, e := boundPointSurfaceDistanceContext(ctx, query, curve.patch, SurfaceDistanceOptions{
		SquaredTolerance: o.SquaredTolerance, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells,
		MaxQueuedPoles: o.MaxQueuedPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	}, true)
	r.SquaredDistanceMin, r.SquaredDistanceMax, r.Parameter = q.SquaredDistanceMin, q.SquaredDistanceMax, q.UpperUV.X
	r.PointBounds, r.HasWitness, r.Complete = q.UpperPoint, q.HasUpperPoint, q.Complete
	r.VisitedCells, r.ExactOperations = q.VisitedCells, q.ExactOperations
	return r, e
}

// BoundPointCurveDistance certifies global distance for a continuous clamped
// positive-weight rational B-spline. Unclamped or discontinuous curves return
// ErrInvalid. The exact surface engine uses a constant auxiliary parameter, so
// the geometric image and distance are unchanged; subdivision is only in the
// original curve parameter. Original float poles/weights are preserved exactly.
func BoundPointCurveDistance(query Vec3, curve *BSpline, o CurveDistanceOptions) (result CurveDistanceBounds, err error) {
	return BoundPointCurveDistanceContext(context.Background(), query, curve, o)
}

// BoundPointCurveDistanceContext preserves established bounds on cooperative cancellation.
// Exact arithmetic within one bounded search step is not interrupted.
func BoundPointCurveDistanceContext(ctx context.Context, query Vec3, curve *BSpline, o CurveDistanceOptions) (result CurveDistanceBounds, err error) {
	result.SquaredDistanceMax = math.Inf(1)
	if ctx == nil {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if curve == nil || len(curve.poles) == 0 || !query.IsFinite() {
		return result, ErrInvalid
	}
	result.SquaredDistanceMax = math.Inf(1)
	queuedLimit := o.MaxQueuedPoles
	if queuedLimit == 0 {
		queuedLimit = 262144
	}
	if queuedLimit < 4 {
		return result, ErrInvalid
	}
	if len(curve.poles) > queuedLimit/2 {
		return result, ErrConvergence
	}
	poles := make([]Vec3, 2*len(curve.poles))
	weights := make([]float64, len(poles))
	for i, p := range curve.poles {
		poles[2*i], poles[2*i+1] = p, p
		weights[2*i], weights[2*i+1] = curve.weights[i], curve.weights[i]
	}
	// Direct immutable copying avoids an extra normalization pass.
	surface := &BSplineSurface{degreeU: curve.degree, degreeV: 1, countU: len(curve.poles), countV: 2, poles: poles, weights: weights, knotsU: append([]float64(nil), curve.knots...), knotsV: []float64{0, 0, 1, 1}}
	maxOperations, maxBits := o.MaxOperations, o.MaxCoefficientBits
	if maxOperations == 0 {
		maxOperations = 4000000
	}
	if maxBits == 0 {
		maxBits = 16384
	}
	maxPoles := max(4096, len(poles)+2*(curve.degree+1))
	patch, e := NewSurfacePatch(surface, SurfacePatchOptions{MaxPoles: maxPoles, MaxOperations: maxOperations, MaxCoefficientBits: maxBits})
	if e != nil {
		return result, e
	}
	result.ExactOperations = patch.ExactOperations()
	remaining := maxOperations - result.ExactOperations
	if remaining < 1 {
		return result, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	bound, e := boundPointSurfaceDistanceContext(ctx, query, patch, SurfaceDistanceOptions{SquaredTolerance: o.SquaredTolerance, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxQueuedPoles: o.MaxQueuedPoles, MaxOperations: remaining, MaxCoefficientBits: maxBits}, true)
	result.SquaredDistanceMin, result.SquaredDistanceMax = bound.SquaredDistanceMin, bound.SquaredDistanceMax
	result.Parameter, result.PointBounds = bound.UpperUV.X, bound.UpperPoint
	result.HasWitness, result.Complete = bound.HasUpperPoint, bound.Complete
	result.VisitedCells = bound.VisitedCells
	result.ExactOperations += bound.ExactOperations
	return result, e
}
