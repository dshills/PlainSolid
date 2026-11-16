package trimmed

import (
	"context"
	"errors"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BuildProjectiveSplineRoofBoolean builds a regularized material result
// from two complete original graphs above a shared planar base. A Common
// uses the exact rectangular overlap. A Fuse or Cut requires the same
// projected footprint on both originals. Material uses proven exact
// height order (including equality) or one complete open analytic
// section. If no applicable proof completes, no solid is published.
func BuildProjectiveSplineRoofBoolean(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	op parametric.BooleanOp, o ProjectiveRoofSolidOptions) (ProjectiveRoofSolid, error) {
	if op == parametric.Intersection {
		return BuildProjectiveSplineRoofOverlapCommon(ctx, a, b, axes, baseHeight, o)
	}
	return buildProjectiveSplineRoofBooleanOnEqualFootprints(ctx, a, b, axes, baseHeight, op, o)
}

func buildProjectiveSplineRoofBooleanOnEqualFootprints(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	op parametric.BooleanOp, o ProjectiveRoofSolidOptions) (ProjectiveRoofSolid, error) {
	r, err := BuildOrderedProjectiveSplineRoofSolid(ctx, a, b, axes, baseHeight, op, o)
	if err == nil || !errors.Is(err, geom.ErrConvergence) {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return ProjectiveRoofSolid{}, err
	}
	if op == parametric.Difference {
		return BuildProjectiveSplineRoofContourDifference(ctx, a, b, axes, baseHeight, o)
	}
	return BuildProjectiveSplineRoofContourSolid(ctx, a, b, axes, baseHeight, op, o)
}
