package trimmed

import (
	"context"

	"plainsolid/geom"
)

type parabolaCircleDisplayPlacement struct {
	chart parabolaCircleRigidChart
	part  Solid
}

// TessellateCertified proves the same distance, volume, and optional
// continuous source normal turn in the original world placement. Exact
// isometry preserves these source quantities. The final world vertices
// are rounded once and independently audited for incidence and collision.
func (r *PlacedParabolaTwoCirclesGraphCut) TessellateCertified(ctx context.Context,
	o ParabolaTwoCirclesMeshOptions) ([]CertifiedParabolaTwoCirclesComponentMesh, error) {
	if r == nil || r.local == nil || len(r.world) != len(r.local.components) {
		return nil, geom.ErrInvalid
	}
	return r.local.tessellateCertified(ctx, o, r)
}

func roundParabolaCircleMeshPoint(p exactBoundVector, placement *parabolaCircleDisplayPlacement) (geom.Vec3, exactBoundVector, error) {
	if placement != nil {
		p = parabolaCircleRigidPoint(placement.chart.world, p)
	}
	var x [3]float64
	for i, q := range p {
		x[i], _ = q.Float64()
	}
	world := geom.Vec3{X: x[0], Y: x[1], Z: x[2]}
	if !world.IsFinite() {
		return geom.Vec3{}, exactBoundVector{}, geom.ErrConvergence
	}
	represented := exactBoundPoint(world)
	if placement != nil {
		// The inverse local coordinates of the actual stored world point
		// retain the signed chart volume, including all world rounding.
		represented = parabolaCircleRigidPoint(placement.chart.inverse, represented)
	}
	return world, represented, nil
}
