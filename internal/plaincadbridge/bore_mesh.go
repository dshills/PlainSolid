package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/chartmesh"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func (s stored) tessellateBore(o parametric.TessellationOptions) (mesh.Mesh, error) {
	ctx := context.Background()
	if s.bore.disjointOblique != nil {
		return s.bore.solid.TessellateDisjointObliqueBores(ctx, o)
	}
	if s.bore.obliqueCrossing != nil {
		return s.bore.solid.TessellateCrossedObliqueBores(ctx, o)
	}
	if s.bore.tripleCrossed != nil {
		return s.bore.solid.TessellateTripleCrossedBores(ctx, o)
	}
	if s.bore.crossedCompanions != nil {
		return s.bore.solid.TessellateCrossedWithDisjoint(ctx, o)
	}
	if p := s.bore.blindCrossed; p != nil {
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
	if s.bore.oblique != nil {
		return s.bore.solid.TessellateObliqueBore(ctx, o)
	}
	// Keep the established single-rim refinement when a coarser mesh
	// would lose more than the material-volume budget after STL rounding.
	fine := o
	fine.Angle = min(o.Angle, fineMeshAngle)
	if len(s.bore.cylinders) != 1 || fine.Angle == o.Angle {
		return s.bore.solid.Tessellate(ctx, o)
	}
	m, err := s.bore.solid.Tessellate(ctx, o)
	if err != nil {
		return mesh.Mesh{}, err
	}
	volume := s.result.GeometryAssertions.Volume
	tolerance := volume * meshVolumeBudget
	if math.Abs(float32MeshVolume(m)-volume) < tolerance {
		return m, nil
	}
	return s.bore.solid.Tessellate(ctx, fine)
}

// meshVolumeBudget is the part of the volume of a solid by which its mesh
// may differ from it, after the rounding of an STL, before the solid is
// meshed again at fineMeshAngle, the facet angle of that second mesh.
const (
	meshVolumeBudget = 1e-4
	fineMeshAngle    = .045
)

// tessellateGeneral meshes a result of the general Boolean and holds the
// mesh to the budget that tessellateBore keeps for a single bore.
//
// A mesh over the budget is made again with a smaller facet angle. The
// volume a faceted round loses goes as the square of that angle, so the
// angle that the budget needs follows from what the first mesh lost, and
// the second mesh is made at four fifths of it, and no finer than
// fineMeshAngle. Only if that mesh is over the budget too is one made at
// fineMeshAngle. Meshing straight at fineMeshAngle, as tessellateBore
// does for its one bore, took ten times as long for a plate with eight
// bores as the mesh the budget needs: the planar faces are triangulated
// in time that grows with the square of their vertices.
//
// Where a finer mesh cannot be built within its limits, the finest that
// was built is the answer.
func (s stored) tessellateGeneral(o parametric.TessellationOptions) (mesh.Mesh, error) {
	build, finest := s.analytic.tessellate, fineMeshAngle
	if meetingTrimmed(s.analytic.solid) {
		// The facets of the chart mesher span 1.8 times the facet angle
		// about an axis, so the budget asks a finer angle of it.
		build = func(o parametric.TessellationOptions) (mesh.Mesh, error) {
			return chartmesh.Tessellate(context.Background(), s.analytic.solid, o)
		}
		finest = fineMeshAngle / 4
	}
	m, err := build(o)
	if err != nil {
		return m, err
	}
	volume := s.result.GeometryAssertions.Volume
	budget := volume * meshVolumeBudget
	angle := o.Angle
	for angle > finest {
		off := math.Abs(float32MeshVolume(m) - volume)
		if off < budget {
			break
		}
		next := max(finest, .8*angle*math.Sqrt(budget/off))
		if angle != o.Angle {
			// The angle worked out from the first mesh was not enough.
			next = finest
		}
		fine := o
		fine.Angle, angle = next, next
		refined, err := build(fine)
		if err != nil {
			if next == finest {
				break
			}
			continue
		}
		m = refined
	}
	return m, nil
}

func float32MeshVolume(m mesh.Mesh) float64 {
	vertices := make([]geom.Vec3, len(m.Vertices))
	for i, v := range m.Vertices {
		vertices[i] = geom.Vec3{X: float64(float32(v.X)), Y: float64(float32(v.Y)), Z: float64(float32(v.Z))}
	}
	return (mesh.Mesh{Vertices: vertices, Triangles: m.Triangles}).SignedVolume()
}

// meetingTrimmed reports whether a face of the solid is trimmed along the
// curve in which two cylinders of unequal radius or with skew axes meet
// (specs/general-boolean-quartic). The mesher of brep/trimmed refines
// every face evenly, and on such a face that folds where the curve runs
// along the chart and then exceeds its limit of triangles. A body with
// such a face is meshed by brep/chartmesh, face by face in its charts.
func meetingTrimmed(s trimmed.Solid) bool {
	for _, f := range s.Faces() {
		for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if _, ok := u.Curve.(trimmed.MonotoneUV); ok {
					return true
				}
			}
		}
	}
	return false
}
