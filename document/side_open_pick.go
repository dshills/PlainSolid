package document

import (
	"context"
	"fmt"

	"plainsolid/geom"
	"plainsolid/scene"
)

// PickAuthoredSideOpenDualDiskNormals selects the first exact boundary among
// the requested authored occurrences and returns original material normals.
// Asset recipes are regenerated, then world placements are applied by the
// exact scene query, so no display mesh is needed. Only supported authored
// insertions with a retained side-cut recipe participate in this query.
func (s State) PickAuthoredSideOpenDualDiskNormals(ctx context.Context, camera scene.Camera, screen geom.Vec2, nodes []string, o geom.ToroidLineOptions) (scene.SideOpenDualDiskNormalPick, bool, error) {
	var empty scene.SideOpenDualDiskNormalPick
	if ctx == nil || len(nodes) == 0 || len(nodes) > 1024 {
		return empty, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, false, e
	}
	seen := make(map[string]bool, len(nodes))
	for _, id := range nodes {
		node, ok := s.Nodes[id]
		f := s.Features[node.Asset]
		if !ok || seen[id] || f.Kind != SideOpenDualDiskCutFeature || f.SideOpenDualDisk == nil {
			return empty, false, fmt.Errorf("document: invalid authored side-cut occurrence %q: %w", id, geom.ErrInvalid)
		}
		seen[id] = true
	}
	built, e := s.EvaluateFeatures(ctx)
	if e != nil {
		return empty, false, e
	}
	world, e := built.worldTransforms()
	if e != nil {
		return empty, false, e
	}
	assets := make(map[string]scene.SideOpenDualDiskNormalBoundary)
	items := make([]scene.SideOpenDualDiskNormalItem, 0, len(nodes))
	for _, id := range nodes {
		asset := built.Nodes[id].Asset
		b := assets[asset]
		if b == nil {
			f := s.Features[asset]
			_, b, e = buildAuthoredSideOpenDualDisk(ctx, built.Planar[f.Input], f.SideOpenDualDisk)
			if e != nil {
				return empty, false, fmt.Errorf("document: regenerate side-cut asset %q: %w", asset, e)
			}
			assets[asset] = b
		}
		items = append(items, scene.SideOpenDualDiskNormalItem{ID: id, Boundary: b, Transform: world[id]})
	}
	return scene.PickSideOpenDualDiskNormals(ctx, camera, screen, items, o)
}
