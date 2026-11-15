package geom

import (
	"context"
	"math"
	"math/big"
)

// PeriodicCurveDistanceOptions bounds exact window extraction and the global
// distance search. A zero MaxSpans allows 256 original knot pieces. The
// operation budget is shared between extraction, search and witness mapping.
type PeriodicCurveDistanceOptions struct {
	CurveDistanceOptions
	MaxSpans int
}

// PeriodicCurveDistanceBounds adds the exact original physical parameter and
// its knot owner to a certified distance interval. HasWitness implies that
// OriginalParameter and Cycle are available. Parameter in the embedded result
// is the nearest binary64 rendering of OriginalParameter. An internal knot
// belongs to the piece immediately to its right.
type PeriodicCurveDistanceBounds struct {
	RationalCurveDistanceBounds
	OriginalParameter *big.Rat
	Cycle             *big.Int
	Span              int
}

// BoundPointPeriodicCurveDistanceRational bounds squared distance from a
// rational point to the full closed rational window of a periodic spline.
// Its search uses outward bounds of the exact Bernstein control hulls across
// every original span and maps the witness to the original parameter without
// rounding. If the work limit is reached, any reported bounds are still valid.
func BoundPointPeriodicCurveDistanceRational(ctx context.Context, query [3]*big.Rat, curve *PeriodicBSpline, start, end *big.Rat, options PeriodicCurveDistanceOptions) (result PeriodicCurveDistanceBounds, err error) {
	result.SquaredDistanceMax = math.Inf(1)
	if ctx == nil || curve == nil {
		return result, ErrInvalid
	}
	for _, q := range query {
		if q == nil {
			return result, ErrInvalid
		}
	}
	o := options.CurveDistanceOptions
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxQueuedPoles == 0 {
		o.MaxQueuedPoles = 262144
	}
	if !finite(o.SquaredTolerance) || o.SquaredTolerance < 0 || o.MaxDepth < 0 || o.MaxDepth > 128 || o.MaxCells < 0 {
		return result, ErrInvalid
	}
	limits, err := patchOptions(SurfacePatchOptions{
		MaxPoles: o.MaxQueuedPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if err != nil {
		return result, err
	}
	clamped, spans, work, err := curve.clampedPeriodicWindow(ctx, start, end, ExactBezierSpanOptions{
		SurfacePatchOptions: limits, MaxSpans: options.MaxSpans,
	})
	result.ExactOperations = work
	if err != nil {
		return result, err
	}
	// Leave one operation for the exact physical parameter of a witness.
	searchOptions := o
	searchOptions.MaxOperations -= work + 1
	if searchOptions.MaxOperations < 1 {
		return result, ErrConvergence
	}
	bound, err := BoundPointExactCurveDistanceRational(ctx, query, clamped, searchOptions)
	result.RationalCurveDistanceBounds = bound
	result.ExactOperations += work
	if !bound.HasWitness {
		return result, err
	}
	index := min(int(bound.Parameter), len(spans)-1)
	piece := spans[index]
	parameter := periodicWindowParameter(piece, index, bound.Parameter)
	b := &polynomialBudget{operations: result.ExactOperations, maxOperations: o.MaxOperations, maxBits: limits.MaxCoefficientBits}
	if mappingError := b.check(parameter); mappingError != nil {
		result.HasWitness, result.Complete = false, false
		result.Parameter, result.PointBounds, result.Witness = 0, Box{}, [3]*big.Rat{}
		result.ExactOperations = b.operations
		if err == nil {
			err = mappingError
		}
		return result, err
	}
	result.ExactOperations = b.operations
	result.OriginalParameter, result.Cycle, result.Span = parameter, new(big.Int).Set(piece.Cycle), piece.Span
	result.Parameter, _ = parameter.Float64()
	return result, err
}
