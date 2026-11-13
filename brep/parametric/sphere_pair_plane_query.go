package parametric

import (
	"context"
	"plainsolid/geom"
)

// PlaneSupports forwards the exact original-sphere query through the verified native binding.
func (s SpherePairSet) PlaneSupports(ctx context.Context, origin, normal geom.Vec3, o geom.SurfacePatchEvaluationOptions) (geom.SpherePairPlaneSupports, error) {
	if !s.valid {
		return geom.SpherePairPlaneSupports{}, geom.ErrInvalid
	}
	return s.query.PlaneSupports(ctx, origin, normal, o)
}

// PlaneSection forwards the exact original-sphere query through the verified native binding.
func (s SpherePairSet) PlaneSection(ctx context.Context, origin, normal geom.Vec3, o geom.SurfacePatchEvaluationOptions) (geom.SpherePairPlaneSection, error) {
	if !s.valid {
		return geom.SpherePairPlaneSection{}, geom.ErrInvalid
	}
	return s.query.PlaneSection(ctx, origin, normal, o)
}

// PlaneBoundaryCycle forwards the exact original-sphere query through the verified native binding.
func (s SpherePairSet) PlaneBoundaryCycle(ctx context.Context, origin, normal geom.Vec3, o geom.SurfacePatchEvaluationOptions) (geom.SpherePairPlaneCycle, error) {
	if !s.valid {
		return geom.SpherePairPlaneCycle{}, geom.ErrInvalid
	}
	return s.query.PlaneBoundaryCycle(ctx, origin, normal, o)
}

// PlaneBoundaryRings forwards the exact original-sphere query through the verified native binding.
func (s SpherePairSet) PlaneBoundaryRings(ctx context.Context, origin, normal geom.Vec3, o geom.SurfacePatchEvaluationOptions) (geom.SpherePairPlaneRings, error) {
	if !s.valid {
		return geom.SpherePairPlaneRings{}, geom.ErrInvalid
	}
	return s.query.PlaneBoundaryRings(ctx, origin, normal, o)
}
