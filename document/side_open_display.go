package document

import (
	"context"
	"fmt"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/mesh"
	"plainsolid/scene"
)

// SideOpenDualDiskPresentation builds a headless display for explicitly
// selected occurrences of the bounded two-circular-wall family. It re-proves
// each distinct local asset from its retained supports, including native
// solids reloaded without a document feature recipe. Smoothness and deflection
// are defined in asset coordinates; scene placements remain separate.
// MaxTriangles caps the sum of triangle counts across distinct local assets.
func (s State) SideOpenDualDiskPresentation(ctx context.Context, nodes []string, o trimmed.SideOpenDualDiskMeshOptions) (*scene.Scene, error) {
	if ctx == nil || len(nodes) == 0 || len(nodes) > 1024 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 {
		return nil, geom.ErrInvalid
	}
	if len(s.Features) > 0 {
		built, e := s.EvaluateFeatures(ctx)
		if e != nil {
			return nil, e
		}
		s = built
	}
	if e := s.Validate(); e != nil {
		return nil, e
	}
	placements, e := s.worldTransforms()
	if e != nil {
		return nil, e
	}
	meshes := make(map[string]mesh.Mesh)
	items := make([]scene.Item, 0, len(nodes))
	seen := make(map[string]bool, len(nodes))
	for _, id := range nodes {
		node, ok := s.Nodes[id]
		part, exact := s.Trimmed[node.Asset]
		if !ok || !exact || seen[id] {
			return nil, fmt.Errorf("document: invalid side-opening occurrence %q: %w", id, geom.ErrInvalid)
		}
		seen[id] = true
		if _, ok := meshes[node.Asset]; !ok {
			if o.MaxTriangles <= 0 {
				return nil, geom.ErrConvergence
			}
			proof, e := part.TessellateDualCircularWallCertified(ctx, o)
			if e != nil {
				return nil, fmt.Errorf("document: certify side-opening asset %q: %w", node.Asset, e)
			}
			o.MaxTriangles -= len(proof.Mesh.Triangles)
			meshes[node.Asset] = proof.Mesh
		}
		items = append(items, scene.Item{ID: id, Asset: node.Asset, Transform: placements[id]})
	}
	return scene.New(meshes, items)
}
