package parametric

import (
	"context"
	"plainsolid/geom"
)

// PlaneMaterial retains the verified native sources in an immutable rational
// plane query. Global material semantics and original source masks are retained.
func (s SpherePairSet) PlaneMaterial(ctx context.Context, origin, normal geom.Vec3, o geom.SurfacePatchEvaluationOptions) (geom.SpherePairPlaneMaterial, error) {
	if !s.valid {
		return geom.SpherePairPlaneMaterial{}, geom.ErrInvalid
	}
	return s.query.PlaneMaterial(ctx, origin, normal, o)
}
