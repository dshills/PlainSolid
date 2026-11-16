package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativePlacedParabolaTwoCirclesCut = "trimmed-placed-spline-graph-parabola-two-circles-cut"

// WriteJSON stores the original world controls and the component boundary
// ledger. The similarity chart and all display quantities are rederived on demand.
func (r *PlacedParabolaTwoCirclesGraphCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.local == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleCutRecipe{
		Version: 1, Kind: nativePlacedParabolaTwoCirclesCut, Sources: src, Axis: &r.local.axis, Tolerance: r.tolerance,
		Ledger: parabolaCircleCutRecipeLedger(r.local.history),
	})
}

// ReadPlacedParabolaTwoCirclesGraphCutJSON reconstructs and checks the exact
// shared frame, world owner restrictions, and native component ledger.
func ReadPlacedParabolaTwoCirclesGraphCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*PlacedParabolaTwoCirclesGraphCut, error) {
	var recipe parabolaCircleCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativePlacedParabolaTwoCirclesCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Ledger) == 0 || len(recipe.Ledger) > 3 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleCutRecipeLedger(cut.local.history)) ||
		limits.MaxEntities > 0 && polynomialGraphEntityCount(cut.world) > limits.MaxEntities {
		return nil, geom.ErrConvergence
	}
	return cut, ctx.Err()
}
