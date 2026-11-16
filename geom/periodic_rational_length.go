package geom

import (
	"context"
	"math"
	"math/big"
)

// PeriodicCurveLengthOptions bounds exact extraction and arc length
// refinement separately. The two stages share the exact work budget.
type PeriodicCurveLengthOptions struct {
	CurveLengthBoundOptions
	MaxKnotSpans, MaxPoles int
}

// BoundLengthRational certifies arc length over an exact physical range
// of the original cyclic curve. A window may cross multiple seams.
func (c *PeriodicBSpline) BoundLengthRational(ctx context.Context, start, end *big.Rat, o PeriodicCurveLengthOptions) (CurveLengthBounds, error) {
	return boundPeriodicCurveLength(ctx, c, start, end, o)
}

// BoundLengthRational also accepts nonuniform edited knot spans. A
// separate affine map for each extracted span preserves geometric length.
func (c *ExactPeriodicSplineCurve) BoundLengthRational(ctx context.Context, start, end *big.Rat, o PeriodicCurveLengthOptions) (CurveLengthBounds, error) {
	return boundPeriodicCurveLength(ctx, c, start, end, o)
}

func boundPeriodicCurveLength(ctx context.Context, source exactPeriodicSpanSource, start, end *big.Rat, o PeriodicCurveLengthOptions) (CurveLengthBounds, error) {
	result := CurveLengthBounds{Upper: math.Inf(1)}
	if !finite(o.Tolerance) || o.Tolerance < 0 || o.MaxDepth < 0 || o.MaxDepth > 128 || o.MaxSpans < 0 || o.MaxKnotSpans < 0 {
		return result, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4_000_000
	}
	patch := SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	clamped, _, work, err := clampedPeriodicWindow(ctx, source, start, end, ExactBezierSpanOptions{
		SurfacePatchOptions: patch, MaxSpans: o.MaxKnotSpans,
	})
	result.ExactOperations = work
	if err != nil {
		return result, err
	}
	// The underlying bound charges its exact descriptor's construction.
	// Only the work preceding that construction needs to be reserved here.
	beforeDescriptor := work - clamped.ExactOperations()
	o.MaxOperations -= beforeDescriptor
	if o.MaxOperations < 1 {
		return result, ErrConvergence
	}
	bound, err := BoundExactCurveLengthContext(ctx, clamped, o.CurveLengthBoundOptions)
	bound.ExactOperations += beforeDescriptor
	return bound, err
}
