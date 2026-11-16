package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/geom"
)

// ParabolaTwoCirclesGraphCut retains independently embedded positive
// native layers, with the original roof and wall coedges in each region.
type ParabolaTwoCirclesGraphCut struct {
	sources    [3]*geom.BSplineSurface
	axis       int
	tolerance  float64
	components []Solid
	history    []ParabolaCircleCutComponentHistory
}

// Components returns the separate native solids in original region order.
func (r *ParabolaTwoCirclesGraphCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return slices.Clone(r.components)
}

// History returns caller-owned copies of the component-local roof, parabola,
// circle and wall indices.
func (r *ParabolaTwoCirclesGraphCut) History() []ParabolaCircleCutComponentHistory {
	if r == nil {
		return nil
	}
	return cloneParabolaCircleCutHistory(r.history)
}

// CutParabolaTwoCirclesSplineGraphs subtracts B from A using their complete
// original parabola and two circles. The positive open region yields a
// wall layer with its proved holes. Each positive disk yields a separate lens.
func CutParabolaTwoCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, freeAxis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaTwoCirclesGraphCut, error) {
	stock := [3]*geom.BSplineSurface{base, a, b}
	regions, _, err := prepareParabolaTwoCirclesGraphRegions(ctx, stock, freeAxis, tolerance, o)
	if err != nil {
		return nil, err
	}
	result := &ParabolaTwoCirclesGraphCut{sources: stock, axis: freeAxis, tolerance: tolerance}
	eval := exactGraphVertexEvaluator(ctx, o.Proof)
	roofs := [2]*geom.BSplineSurface{a, b}
	for i, region := range regions {
		if region.sign < 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		part, assembly, err := assembleGraphRegionDifference(a, b, region, tolerance, eval)
		if err != nil {
			return nil, fmt.Errorf("trimmed: parabola and two circles Cut region %d: %w", i, err)
		}
		if err := validateExactGraphPartitionEdges(ctx, part, o.Proof); err != nil {
			return nil, err
		}
		h, err := bindParabolaCircleCutComponent(ctx, part, assembly, roofs, i, len(region.holes))
		if err != nil {
			return nil, err
		}
		result.components = append(result.components, part)
		result.history = append(result.history, h)
	}
	// There is one positive open region. Each positive disk lies on
	// the opposite open side, separated from the wall layer. The circles
	// are also strictly separated from one another.
	want := 1
	for _, region := range regions[2:] {
		if region.sign > 0 {
			want++
		}
	}
	if len(result.components) != want {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
