//go:build research

package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// ParabolaCircleCrossingsMeshContact retains one source node and the two
// distinct component-local vertex IDs that display it.
type ParabolaCircleCrossingsMeshContact struct {
	Source  ParabolaCircleCrossingsCutContact
	Display collision.AssemblyVertexContact
}

// CertifiedParabolaCircleCrossingsMesh retains the two original oriented
// shells and an exact assembly contact certificate. The original nodes are
// shared model positions with separate display vertex namespaces.
type CertifiedParabolaCircleCrossingsMesh struct {
	Components [2]CertifiedParabolaTwoCirclesComponentMesh
	Contacts   [2]ParabolaCircleCrossingsMeshContact
	Assembly   collision.VertexContactAssemblyReport
}

// TessellateCertified displays the original polynomial identity-XY Cut
// roofs with exact source volume bounds, two-sided distance and optional
// source-turn and facet certificates. The original two contact pairs are
// the only allowed intersection between the separately indexed meshes.
func (r *ParabolaCircleCrossingsCut) TessellateCertified(ctx context.Context,
	o ParabolaTwoCirclesMeshOptions) (CertifiedParabolaCircleCrossingsMesh, error) {
	var empty CertifiedParabolaCircleCrossingsMesh
	if ctx == nil || r == nil || r.sources[0] == nil {
		return empty, geom.ErrInvalid
	}
	o, err := checkedParabolaCircleMeshOptions(o)
	if err != nil {
		return empty, err
	}
	var height [2]parabolaCircleMeshHeight
	var patches [2]*geom.SurfacePatch
	for i := range height {
		height[i], patches[i], err = parabolaCircleDisplayHeight(ctx, r.sources[i+1])
		if err != nil {
			return empty, err
		}
		height[i].normalHessian = parabolaCircleMeshHessian(r.sources[i+1])
		height[i].hessian = height[i].normalHessian
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircleCrossings(ctx, patches[0], patches[1], r.axis, geom.GraphPointContactOptions{})
	if err != nil {
		return empty, err
	}
	volumes := parabolaCircleCrossingMeshVolumes(height, proof)
	plane, err := parabolaCircleMeshUVPlane(ctx)
	if err != nil {
		return empty, err
	}
	var result CertifiedParabolaCircleCrossingsMesh
	var meshes [2]mesh.Mesh
	triangleBudget := o.MaxTriangles
	for i, s := range r.components {
		h := r.history[i]
		if h.Region < 0 || h.Region >= len(volumes) || proof.RegionSides[h.Region] <= 0 || volumes[h.Region][0].Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
		if o.MaxTriangles < len(s.faces) {
			return empty, parametric.ErrBudget
		}
		owners := ParabolaCircleCutComponentHistory{Region: h.Region, RoofFaces: h.RoofFaces}
		for _, id := range h.WallFaces {
			if id >= 0 {
				owners.WallFaces = append(owners.WallFaces, id)
			}
		}
		result.Components[i], err = parabolaTwoCirclesComponentDisplay(ctx, s, owners, height, plane, volumes[h.Region], o, nil, true)
		if err != nil {
			return empty, fmt.Errorf("trimmed: parabola-circle crossing display region %d: %w", h.Region, err)
		}
		meshes[i] = result.Components[i].Mesh
		o.MaxTriangles -= len(meshes[i].Triangles)
	}
	declarations := make([]collision.AssemblyVertexContact, len(r.contacts))
	for i, source := range r.Contacts() {
		node := proof.Nodes[i]
		if source.Parameters[0].Cmp(node.Parameters[0]) != 0 || source.Parameters[1].Cmp(node.Parameters[1]) != 0 ||
			!exactPointCoordinatesEqual(source.Point, node.Point) {
			return empty, geom.ErrConvergence
		}
		var vertices [2]collision.AssemblyVertex
		for j, native := range source.Vertices {
			m := result.Components[j]
			if native < 0 || native >= len(m.OriginalVertices) {
				return empty, geom.ErrConvergence
			}
			vertex := m.OriginalVertices[native]
			if vertex < 0 || vertex >= len(m.Mesh.Vertices) || !exactPointCoordinatesEqual(exactBoundPoint(m.Mesh.Vertices[vertex]), source.Point) {
				return empty, geom.ErrConvergence
			}
			for _, ray := range source.Rays[j] {
				if ray < 0 || ray >= len(m.OriginalEdges) {
					return empty, geom.ErrConvergence
				}
				edge := m.OriginalEdges[ray].Vertices
				if len(edge) < 2 || edge[0] != vertex && edge[len(edge)-1] != vertex {
					return empty, geom.ErrConvergence
				}
			}
			vertices[j] = collision.AssemblyVertex{Solid: j, Vertex: vertex}
		}
		declarations[i] = collision.AssemblyVertexContact{A: vertices[0], B: vertices[1]}
		result.Contacts[i] = ParabolaCircleCrossingsMeshContact{Source: source, Display: declarations[i]}
	}
	maxPairs := o.Collision.MaxPairs
	if maxPairs == 0 {
		maxPairs = collision.DefaultMaxPairs
	}
	result.Assembly, err = collision.ValidateMeshVertexContactAssembly(ctx, meshes[:], declarations, collision.VertexContactAssemblyOptions{
		MaxSolids: 2, MaxDeclaredContacts: 2, MaxCrossPairs: maxPairs, CrossBVH: true,
		Validation: collision.LocationOptions{MaxTriangles: triangleBudget, MaxCollisionPairs: maxPairs},
	})
	if err != nil {
		return empty, fmt.Errorf("trimmed: parabola-circle crossing display assembly: %w", err)
	}
	signed := new(big.Rat).Add(result.Components[0].SignedDisplayVolume, result.Components[1].SignedDisplayVolume)
	if result.Assembly.SignedVolume.Cmp(signed) != 0 {
		return empty, geom.ErrConvergence
	}
	return result, nil
}
