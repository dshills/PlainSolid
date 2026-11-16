package trimmed

import (
	"context"

	"plainsolid/geom"
	"plainsolid/mesh"
)

type parabolaCircleDisplayPlacement struct {
	chart parabolaCircleSimilarity
	part  Solid
}

// TessellateCertified proves physical distance and signed volume in the
// original world placement. The exact similarity preserves both optional
// normal-angle bounds. The final world vertices are rounded once and
// independently audited for incidence and collision.
func (r *PlacedParabolaTwoCirclesGraphCut) TessellateCertified(ctx context.Context,
	o ParabolaTwoCirclesMeshOptions) ([]CertifiedParabolaTwoCirclesComponentMesh, error) {
	if r == nil || r.local == nil || len(r.world) != len(r.local.components) {
		return nil, geom.ErrInvalid
	}
	return r.local.tessellateCertified(ctx, o, r)
}

func roundParabolaCircleMeshPoint(p exactBoundVector, placement *parabolaCircleDisplayPlacement) (geom.Vec3, exactBoundVector, error) {
	if placement != nil {
		p = parabolaCircleApplyRational(placement.chart.forward, p)
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
		// Measure rounding of the actual stored world point in the same
		// exact source chart as the footprint and angular proof.
		represented = parabolaCircleApplyRational(placement.chart.inverse, represented)
	}
	return world, represented, nil
}

func (p *parabolaCircleDisplayPlacement) scaled() bool {
	return p != nil && !p.chart.signedAxisUnit
}

func (c parabolaCircleSimilarity) separatesDisplayedComponents(ctx context.Context, meshes []mesh.Mesh) (bool, error) {
	type projectedBounds struct{ low, high exactBoundVector }
	boxes := make([]projectedBounds, len(meshes))
	for i, m := range meshes {
		for j, v := range m.Vertices {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			qs := parabolaCircleApplyRational(c.inverse, exactBoundPoint(v))
			for axis, q := range qs {
				if j == 0 || q.Cmp(boxes[i].low[axis]) < 0 {
					boxes[i].low[axis] = q
				}
				if j == 0 || q.Cmp(boxes[i].high[axis]) > 0 {
					boxes[i].high[axis] = q
				}
			}
		}
	}
	for i, a := range boxes {
		for _, b := range boxes[:i] {
			separate := false
			for axis := range a.low {
				if a.high[axis].Cmp(b.low[axis]) < 0 || b.high[axis].Cmp(a.low[axis]) < 0 {
					separate = true
					break
				}
			}
			if !separate {
				return false, nil
			}
		}
	}
	return true, nil
}
