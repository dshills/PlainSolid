package offset

import (
	"context"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// InsetWrappedEccentricCircularFrustumTube applies the same independent material
// setbacks as InsetWrappedCircularFrustumTube to a native parallel-axis circular
// through-bore. Each wall retains its original center and exact circular basis.
// At both retained cap heights, the positive radial difference g must satisfy
// g² > dx²+dy². Since g is affine in height, these exact endpoint inequalities
// prove strict clearance throughout, even for irrational axis separation.
//
// The full original native tube and output sewing are independently validated.
// Blind/cavity geometry, tangent/intersecting walls, nonrepresentable native
// vertices and exhausted proof budgets fail atomically. RadialClearance in the
// returned history is the difference of radii, not the smaller eccentric gap.
// Per-call proof budgets and original face ancestry match the coaxial API.
func InsetWrappedEccentricCircularFrustumTube(ctx context.Context, source trimmed.Solid, in FrustumTubeInsets, o geom.SurfacePatchOptions) (trimmed.Solid, FrustumTubeInsetHistory, error) {
	return insetWrappedCircularFrustumTube(ctx, source, in, o, false)
}
