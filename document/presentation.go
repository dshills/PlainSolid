package document

import (
	"context"
	"fmt"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
	"plainsolid/scene"
)

// DrawingOptions limits tessellation and projection of exact assembly edges.
// Visibility uses a tessellated occluder and sampled curve chords.
type DrawingOptions struct {
	Tessellation                           parametric.TessellationOptions
	Curves                                 geom.CurveTessellationOptions
	Occlusion                              scene.HiddenLineOptions
	MaxVertices, MaxTriangles, MaxSegments int
}

// DrawVisibleEdges projects topological edges from planar, patch and trimmed
// assets. Mesh-only assets occlude but have no exact edges. Reused assets
// retain occurrence-qualified IDs in the form <node length>:<node>:<edge>.
func (s State) DrawVisibleEdges(ctx context.Context, c scene.Camera, o DrawingOptions) ([]scene.VisibleSpan, error) {
	if ctx == nil || o.MaxSegments < 0 || o.MaxVertices < 0 || o.MaxTriangles < 0 {
		return nil, fmt.Errorf("document: invalid drawing options")
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 100000
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 3000000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	var e error
	if len(s.Features) > 0 {
		s, e = s.EvaluateFeatures(ctx)
		if e != nil {
			return nil, e
		}
	}
	m, e := s.FlattenGeometry(ctx, o.MaxVertices, o.MaxTriangles, o.Tessellation)
	if e != nil {
		return nil, e
	}
	world, e := s.worldTransforms()
	if e != nil {
		return nil, e
	}
	ids := make([]string, 0, len(s.Nodes))
	for id, n := range s.Nodes {
		if n.Asset != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	cache := map[string][]scene.DrawingEdge{}
	var edges []scene.DrawingEdge
	for _, id := range ids {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		asset := s.Nodes[id].Asset
		chords, ok := cache[asset]
		left := o.MaxSegments - len(edges)
		if !ok {
			if p, ok := s.Planar[asset]; ok {
				chords = scene.PlanarEdges(p)
			} else if p, ok := s.Parametric[asset]; ok {
				chords, e = scene.ParametricEdges(p, o.Curves, left)
			} else if p, ok := s.Trimmed[asset]; ok {
				chords, e = scene.TrimmedEdges(p, o.Curves, left)
			}
			if e != nil {
				return nil, e
			}
			cache[asset] = chords
		}
		if len(chords) > left {
			return nil, fmt.Errorf("document: drawing segment limit exceeded")
		}
		for _, x := range chords {
			edges = append(edges, scene.DrawingEdge{ID: fmt.Sprintf("%d:%s:%s", len(id), id, x.ID), Start: world[id].Apply(x.Start), End: world[id].Apply(x.End)})
		}
	}
	return scene.HiddenLines(ctx, c, m, edges, o.Occlusion)
}

// Presentation builds a headless picking scene with document node IDs. Each
// asset is tessellated once in local coordinates; placements stay as instance
// transforms. MaxTriangles limits the sum of the distinct cached asset meshes.
// The tessellation options control curved assets.
func (s State) Presentation(ctx context.Context, opts parametric.TessellationOptions, maxTriangles int) (*scene.Scene, error) {
	if ctx == nil || maxTriangles <= 0 {
		return nil, fmt.Errorf("document: invalid presentation options")
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(s.Features) > 0 {
		m, e := s.EvaluateFeatures(ctx)
		if e != nil {
			return nil, e
		}
		s = m
	}
	if e := s.Validate(); e != nil {
		return nil, e
	}
	transforms, e := s.worldTransforms()
	if e != nil {
		return nil, e
	}
	ids := make([]string, 0, len(s.Nodes))
	for id, n := range s.Nodes {
		if n.Asset != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	m := map[string]mesh.Mesh{}
	items := make([]scene.Item, 0, len(ids))
	for _, id := range ids {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		n := s.Nodes[id]
		if _, ok := m[n.Asset]; !ok {
			p, err := s.localMesh(ctx, n.Asset, opts, maxTriangles)
			if err != nil {
				return nil, err
			}
			if len(p.Triangles) > maxTriangles {
				return nil, fmt.Errorf("document: presentation triangle limit")
			}
			maxTriangles -= len(p.Triangles)
			m[n.Asset] = p
		}
		tr := transforms[id]
		items = append(items, scene.Item{ID: id, Asset: n.Asset, Transform: tr})
	}
	return scene.New(m, items)
}

func (s State) localMesh(ctx context.Context, id string, o parametric.TessellationOptions, limit int) (mesh.Mesh, error) {
	if m, ok := s.Assets[id]; ok {
		return m, nil
	}
	if m, ok := s.Planar[id]; ok {
		return m.Tessellate(), nil
	}
	if limit <= 0 {
		return mesh.Mesh{}, fmt.Errorf("document: triangle limit")
	}
	if o.MaxTriangles == 0 || o.MaxTriangles > limit {
		o.MaxTriangles = limit
	}
	var m mesh.Mesh
	var err error
	if v, ok := s.Trimmed[id]; ok {
		m, err = v.Tessellate(ctx, o)
	} else if v, ok := s.Parametric[id]; ok {
		m, err = v.Tessellate(ctx, o)
	} else {
		return mesh.Mesh{}, fmt.Errorf("document: missing asset %q", id)
	}
	if err != nil {
		return mesh.Mesh{}, fmt.Errorf("document: tessellate %q: %w", id, err)
	}
	return m, nil
}
