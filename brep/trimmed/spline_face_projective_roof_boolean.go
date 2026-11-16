package trimmed

import (
	"context"
	"errors"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BuildProjectiveSplineRoofBoolean builds a regularized material result
// from the complete original graphs over one common rectangular
// footprint and shared planar base. It tries an exact strict height
// order (including equality), then one complete open analytic section
// if strict ordering could not be proved. Each proof uses its supplied
// limits; if neither completes, no solid is published.
func BuildProjectiveSplineRoofBoolean(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
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
