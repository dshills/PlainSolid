package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"slices"
)

// PeriodicSurfaceDistanceOptions bounds exact knot extraction and the global
// search. MaxKnotCells and MaxPoles bound the input grid of exact patches.
// SurfaceDistanceOptions.MaxCells bounds the total number of visited search
// cells. All stages share MaxOperations and MaxCoefficientBits.
type PeriodicSurfaceDistanceOptions struct {
	SurfaceDistanceOptions
	MaxKnotCells, MaxPoles int
}

// PeriodicSurfaceDistanceBounds reports global squared-distance bounds and,
// when HasUpperPoint, a sample in original physical parameters. The U and V
// spans retain the original one-sided knot owners of that sample. UpperUV is
// the binary64 rendering of OriginalUV.
type PeriodicSurfaceDistanceBounds struct {
	RationalSurfaceDistanceBounds
	OriginalUV [2]*big.Rat
	U, V       ExactSurfaceKnotSpan
}

type periodicSurfaceDistanceCell struct {
	cell  ExactPeriodicSurfaceBezierCell
	patch *SurfacePatch
	lower *big.Rat
}

// BoundPointPeriodicSurfaceDistanceRational certifies distance to the full
// union of closed original knot cells in a rational parameter rectangle. Each
// cell is searched in normalized coordinates using its exact homogeneous net.
// The certificate includes every active cycle and keeps conservative partial
// bounds when the caller's search budget is exhausted.
func BoundPointPeriodicSurfaceDistanceRational(ctx context.Context, query [3]*big.Rat, surface *PeriodicBSplineSurface, u, v ExactParameterInterval, options PeriodicSurfaceDistanceOptions) (result PeriodicSurfaceDistanceBounds, err error) {
	result.SquaredDistanceMax = math.Inf(1)
	if ctx == nil || surface == nil {
		return result, ErrInvalid
	}
	o := options.SurfaceDistanceOptions
	if o.SquaredTolerance == 0 {
		o.SquaredTolerance = 1e-8
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if !finite(o.SquaredTolerance) || o.SquaredTolerance < 0 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxDepth > 128 || o.MaxQueuedPoles < 0 || o.MaxQueuedPoles > 0 && o.MaxQueuedPoles < 4 {
		return result, ErrInvalid
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxPoles: options.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return result, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	for _, q := range query {
		if q == nil {
			return result, ErrInvalid
		}
		if err := b.check(q); err != nil {
			return result, err
		}
	}
	limits.MaxOperations -= b.operations
	if limits.MaxOperations < 1 {
		return result, ErrConvergence
	}
	cells, work, err := surface.ExactBezierCells(ctx, u, v, PeriodicSurfaceBezierOptions{SurfacePatchOptions: limits, MaxCells: options.MaxKnotCells})
	b.operations += work
	if err != nil {
		return result, err
	}
	queue := make([]periodicSurfaceDistanceCell, 0, len(cells))
	for _, cell := range cells {
		limits.MaxOperations = b.maxOperations - b.operations
		if limits.MaxOperations < 1 {
			return result, ErrConvergence
		}
		patch, err := cell.NormalizedPatch(ctx, limits)
		if err != nil {
			return result, err
		}
		b.operations += patch.ExactOperations()
		lower, err := exactRationalPointBoxDistance(query, patch.bounds, b)
		if err != nil {
			return result, err
		}
		queue = append(queue, periodicSurfaceDistanceCell{cell: cell, patch: patch, lower: lower})
	}
	slices.SortStableFunc(queue, func(a, b periodicSurfaceDistanceCell) int { return a.lower.Cmp(b.lower) })
	return searchPeriodicSurfaceDistance(ctx, query, queue, o, b)
}

func searchPeriodicSurfaceDistance(ctx context.Context, query [3]*big.Rat, queue []periodicSurfaceDistanceCell, options SurfaceDistanceOptions, b *polynomialBudget) (result PeriodicSurfaceDistanceBounds, err error) {
	result.SquaredDistanceMax = math.Inf(1)
	tolerance := new(big.Rat).SetFloat64(options.SquaredTolerance)
	var exploredLower *big.Rat
	var searchError error
	for i := 0; ; i++ {
		lower := exploredLower
		if i < len(queue) && (lower == nil || queue[i].lower.Cmp(lower) < 0) {
			lower = queue[i].lower
		}
		result.Squared.Lower = new(big.Rat).Set(lower)
		result.SquaredDistanceMin = rationalFloatBound(lower, true)
		if err := ctx.Err(); err != nil {
			return result, err
		}
		upper := result.Squared.Upper
		if upper != nil && new(big.Rat).Sub(upper, lower).Cmp(tolerance) <= 0 {
			result.Complete = true
			return result, nil
		}
		if i == len(queue) {
			if searchError == nil {
				searchError = ErrConvergence
			}
			return result, searchError
		}
		search := options
		search.MaxCells -= result.VisitedCells
		// Reserve the checks for the two original parameter coordinates.
		search.MaxOperations = b.maxOperations - b.operations - 2
		if search.MaxCells < 1 || search.MaxOperations < 1 {
			return result, ErrConvergence
		}
		current := queue[i]
		bound, err := BoundPointSurfaceDistanceRational(ctx, query, current.patch, search)
		b.operations += bound.ExactOperations
		result.VisitedCells += bound.VisitedCells
		cellLower := current.lower
		if bound.Squared.Lower != nil && bound.Squared.Lower.Cmp(cellLower) > 0 {
			cellLower = bound.Squared.Lower
		}
		if exploredLower == nil || cellLower.Cmp(exploredLower) < 0 {
			exploredLower = cellLower
		}
		if bound.HasUpperPoint && (upper == nil || bound.Squared.Upper.Cmp(upper) < 0) {
			if updateError := recordPeriodicSurfaceDistanceWitness(&result, current.cell, bound, b); updateError != nil {
				return result, updateError
			}
		}
		if err != nil {
			if !errors.Is(err, ErrConvergence) {
				return result, err
			}
			searchError = err
		}
	}
}

func recordPeriodicSurfaceDistanceWitness(result *PeriodicSurfaceDistanceBounds, cell ExactPeriodicSurfaceBezierCell, bound RationalSurfaceDistanceBounds, budget *polynomialBudget) error {
	var uv [2]*big.Rat
	for axis, x := range [2]float64{bound.UpperUV.X, bound.UpperUV.Y} {
		span := [2]ExactSurfaceKnotSpan{cell.U, cell.V}[axis]
		width := new(big.Rat).Sub(span.Domain.Upper, span.Domain.Lower)
		uv[axis] = new(big.Rat).Add(span.Domain.Lower, new(big.Rat).Mul(new(big.Rat).SetFloat64(x), width))
		if err := budget.check(uv[axis]); err != nil {
			return err
		}
	}
	x, _ := uv[0].Float64()
	y, _ := uv[1].Float64()
	result.UpperUV = Vec2{X: x, Y: y}
	result.OriginalUV, result.U, result.V = uv, cell.U, cell.V
	result.HasUpperPoint, result.UpperPoint, result.Witness = true, bound.UpperPoint, bound.Witness
	result.SquaredDistanceMax, result.Squared.Upper = bound.SquaredDistanceMax, bound.Squared.Upper
	return nil
}
