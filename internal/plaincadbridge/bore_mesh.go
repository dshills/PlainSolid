package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func (s stored) tessellateBore(o parametric.TessellationOptions) (mesh.Mesh, error) {
	ctx := context.Background()
	if s.bore.obliqueCrossing != nil {
		return s.bore.solid.TessellateCrossedBores345(ctx, o)
	}
	if s.bore.tripleCrossed != nil {
		return s.bore.solid.TessellateTripleCrossedBores(ctx, o)
	}
	if s.bore.crossedCompanions != nil {
		return s.bore.solid.TessellateCrossedWithDisjoint(ctx, o)
	}
	if p := s.bore.blindCrossed; p != nil {
		if p.partialSkew() {
			return s.bore.solid.TessellateFinitePartialSkewBores(ctx, o)
		}
		if p.radii[0] != p.radii[1] {
			return s.bore.solid.TessellateFiniteSkewCrossedBores(ctx, o)
		}
		if p.clippedFloor {
			switch {
			case p.depth[1] < p.crossing.Z:
				return s.bore.solid.TessellateBeforeCrossedBores(ctx, o)
			case p.depth[1] == p.crossing.Z:
				return s.bore.solid.TessellateCenterCrossedBores(ctx, o)
			default:
				return s.bore.solid.TessellateClippedCrossedBores(ctx, o)
			}
		}
		return s.bore.solid.TessellateBlindCrossedBores(ctx, o)
	}
	if s.bore.partialSkew {
		return s.bore.solid.TessellatePartialSkewBores(ctx, o)
	}
	if body := s.bore; body.crossed != nil && body.cylinders[0].radius != body.cylinders[1].radius {
		return body.solid.TessellateUnequalCrossedBores(ctx, o)
	}
	if s.bore.oblique != nil {
		return s.bore.solid.TessellateObliqueBore(ctx, o)
	}
	// Keep the established single-rim refinement when a coarser mesh
	// would lose more than the material-volume budget after STL rounding.
	fine := o
	fine.Angle = min(o.Angle, .045)
	if len(s.bore.cylinders) != 1 || fine.Angle == o.Angle {
		return s.bore.solid.Tessellate(ctx, o)
	}
	m, err := s.bore.solid.Tessellate(ctx, o)
	if err != nil {
		return mesh.Mesh{}, err
	}
	volume := s.result.GeometryAssertions.Volume
	tolerance := volume * 1e-4
	if math.Abs(float32MeshVolume(m)-volume) < tolerance {
		return m, nil
	}
	return s.bore.solid.Tessellate(ctx, fine)
}

func float32MeshVolume(m mesh.Mesh) float64 {
	vertices := make([]geom.Vec3, len(m.Vertices))
	for i, v := range m.Vertices {
		vertices[i] = geom.Vec3{X: float64(float32(v.X)), Y: float64(float32(v.Y)), Z: float64(float32(v.Z))}
	}
	return (mesh.Mesh{Vertices: vertices, Triangles: m.Triangles}).SignedVolume()
}
