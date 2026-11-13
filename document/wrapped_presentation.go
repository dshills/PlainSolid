package document

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/geom"
	"plainsolid/scene"
)

// PickWrappedGeometry evaluates the document's current feature graph and
// queries supported wrapped occurrences by exact boundary geometry. All
// participating assets must be supported wrapped trimmed bodies. The
// current adapter requires pure translations that preserve the canonical
// Z axis; the returned ID is the original document node.
func (s State) PickWrappedGeometry(ctx context.Context, camera scene.Camera, screen geom.Vec2,
	o geom.ToroidLineOptions) (scene.WrappedPickResult, bool, error) {
	var zero scene.WrappedPickResult
	if ctx == nil {
		return zero, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, false, e
	}
	var e error
	if len(s.Features) != 0 {
		s, e = s.EvaluateFeatures(ctx)
		if e != nil {
			return zero, false, e
		}
	}
	if e = s.Validate(); e != nil {
		return zero, false, e
	}
	var ids []string
	for id, node := range s.Nodes {
		if node.Asset != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) > 1024 {
		return zero, false, fmt.Errorf("document: wrapped picking occurrence limit")
	}
	sort.Strings(ids)
	items := make([]scene.WrappedPickItem, 0, len(ids))
	for _, id := range ids {
		if e := ctx.Err(); e != nil {
			return zero, false, e
		}
		node := s.Nodes[id]
		part, ok := s.Trimmed[node.Asset]
		if !ok {
			return zero, false, fmt.Errorf("document: wrapped picking node %q requires a trimmed asset", id)
		}
		t, e := wrappedPickWorldTranslation(ctx, s.Nodes, id)
		if e != nil {
			return zero, false, fmt.Errorf("document: wrapped picking node %q: %w", id, e)
		}
		items = append(items, scene.WrappedPickItem{ID: id, Solid: part, Translation: t})
	}
	return scene.PickWrappedMaterials(ctx, camera, screen, items, o)
}

// State.Validate has already established the parent graph is finite and
// acyclic. Compose represented translations in exact rational arithmetic.
func wrappedPickWorldTranslation(ctx context.Context, nodes map[string]Node, id string) (geom.Vec3, error) {
	p := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	for id != "" {
		if e := ctx.Err(); e != nil {
			return geom.Vec3{}, e
		}
		n := nodes[id]
		m := n.Transform
		t := geom.Vec3{X: m[0][3], Y: m[1][3], Z: m[2][3]}
		if m != geom.Translation(t) {
			return geom.Vec3{}, geom.ErrConvergence
		}
		for j, x := range [3]float64{t.X, t.Y, t.Z} {
			p[j].Add(p[j], new(big.Rat).SetFloat64(x))
		}
		id = n.Parent
	}
	return geom.RepresentedExactPoint(p)
}
