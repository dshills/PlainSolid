package document

import (
	"context"
	"fmt"
	"io"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/exchange"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// WriteGLTFScene writes the document's assembled mesh scene in embedded
// glTF (binary=false) or GLB (binary=true) form.
func (s State) WriteGLTFScene(ctx context.Context, w io.Writer, name string, o parametric.TessellationOptions, maxTriangles int, binary bool) error {
	if w == nil {
		return fmt.Errorf("document: nil glTF writer")
	}
	r, e := s.MeshScene(ctx, name, o, maxTriangles)
	if e != nil {
		return e
	}
	if binary {
		return exchange.WriteGLBScene(w, r)
	}
	return exchange.WriteGLTFScene(w, r)
}

// MeshScene exports one mesh definition per referenced asset, preserving
// occurrences and local placement. Curved definitions use the requested
// tessellation. Scale converts document coordinates to meters; the caller
// is responsible for choosing a Y-up coordinate frame before export.
func (s State) MeshScene(ctx context.Context, name string, o parametric.TessellationOptions, maxTriangles int) (exchange.MeshScene, error) {
	if ctx == nil || maxTriangles < 1 {
		return exchange.MeshScene{}, fmt.Errorf("document: invalid mesh scene options")
	}
	if e := ctx.Err(); e != nil {
		return exchange.MeshScene{}, e
	}
	var e error
	if len(s.Features) > 0 {
		s, e = s.EvaluateFeatures(ctx)
		if e != nil {
			return exchange.MeshScene{}, e
		}
	}
	if e = s.Validate(); e != nil {
		return exchange.MeshScene{}, e
	}
	if len(s.Nodes) > 100000 {
		return exchange.MeshScene{}, fmt.Errorf("document: scene node limit")
	}
	world, e := s.worldTransforms()
	if e != nil {
		return exchange.MeshScene{}, e
	}
	ids := make([]string, 0, len(s.Nodes))
	for id := range s.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if name == "" {
		name = s.Labels["gltf.scene"].Name
	}
	r := exchange.MeshScene{Name: name}
	nodeIDs, meshIDs := map[string]int{}, map[string]int{}
	names := map[string]string{}
	labelIDs := make([]string, 0, len(s.Labels))
	for id := range s.Labels {
		labelIDs = append(labelIDs, id)
	}
	sort.Strings(labelIDs)
	for _, id := range labelIDs {
		l := s.Labels[id]
		if l.Target == AssetLabel && l.Name != "" && names[l.TargetID] == "" {
			names[l.TargetID] = l.Name
		}
	}
	scale := s.LengthUnit
	toMeters := func(x geom.Affine) geom.Affine {
		for i := range 3 {
			x[i][3] *= scale
		}
		return x
	}
	for i, id := range ids {
		if e := ctx.Err(); e != nil {
			return exchange.MeshScene{}, e
		}
		n := s.Nodes[id]
		j := -1
		if n.Asset != "" {
			var ok bool
			j, ok = meshIDs[n.Asset]
			if !ok {
				if len(r.Meshes) >= 10000 {
					return exchange.MeshScene{}, fmt.Errorf("document: scene mesh limit")
				}
				m, e := s.localMesh(ctx, n.Asset, o, maxTriangles)
				if e != nil {
					return exchange.MeshScene{}, e
				}
				if len(m.Triangles) == 0 || len(m.Triangles) > maxTriangles {
					return exchange.MeshScene{}, fmt.Errorf("document: empty mesh or scene triangle limit")
				}
				maxTriangles -= len(m.Triangles)
				// Copy caller-visible mesh definitions even when no scaling is needed.
				m = mesh.Mesh{Vertices: append([]geom.Vec3(nil), m.Vertices...), Triangles: append([][3]int(nil), m.Triangles...)}
				for i, p := range m.Vertices {
					m.Vertices[i] = p.Scale(scale)
				}
				if e = m.Validate(); e != nil {
					return exchange.MeshScene{}, e
				}
				meshName := names[n.Asset]
				if meshName == "" {
					meshName = n.Asset
				}
				j = len(r.Meshes)
				meshIDs[n.Asset] = j
				r.Meshes = append(r.Meshes, exchange.SceneMesh{Name: meshName, Geometry: m})
			}
		}
		local, placement := toMeters(n.Transform), toMeters(world[id])
		if !local.IsFinite() || !placement.IsFinite() {
			return exchange.MeshScene{}, geom.ErrInvalid
		}
		nodeIDs[id] = i
		r.Nodes = append(r.Nodes, exchange.SceneNode{Name: n.Name, Mesh: j, Local: local, World: placement})
	}
	for _, id := range ids {
		p := s.Nodes[id].Parent
		i := nodeIDs[id]
		if p == "" {
			r.Roots = append(r.Roots, i)
		} else {
			j := nodeIDs[p]
			r.Nodes[j].Children = append(r.Nodes[j].Children, i)
		}
	}
	return r, nil
}

// FromMeshScene imports an embedded glTF/GLB scene as shared mesh assets and
// named occurrences. glTF coordinates are kept in meters with Y up.
func FromMeshScene(scene exchange.MeshScene, historyLimit int) (*Document, error) {
	if len(scene.Meshes) > 10000 || len(scene.Nodes) > 100000 {
		return nil, fmt.Errorf("document: scene size limit exceeded")
	}
	s := State{LengthUnit: 1, Assets: map[string]mesh.Mesh{}, Nodes: map[string]Node{}, Labels: map[string]Label{}}
	meshID := func(i int) string { return fmt.Sprintf("mesh-%06d", i+1) }
	nodeID := func(i int) string { return fmt.Sprintf("node-%06d", i+1) }
	if scene.Name != "" {
		s.Labels["gltf.scene"] = Label{Name: scene.Name, Target: DocumentLabel}
	}
	for i, m := range scene.Meshes {
		id := meshID(i)
		s.Assets[id] = m.Geometry
		if m.Name != "" {
			s.Labels["gltf:"+id] = Label{Name: m.Name, Target: AssetLabel, TargetID: id}
		}
	}
	parents := make([]int, len(scene.Nodes))
	for i := range parents {
		parents[i] = -1
	}
	for i, n := range scene.Nodes {
		for _, j := range n.Children {
			if j < 0 || j >= len(scene.Nodes) || j == i || parents[j] >= 0 {
				return nil, fmt.Errorf("document: invalid scene child")
			}
			parents[j] = i
		}
	}
	roots := map[int]bool{}
	for _, i := range scene.Roots {
		if i < 0 || i >= len(scene.Nodes) || parents[i] >= 0 || roots[i] {
			return nil, fmt.Errorf("document: invalid scene root")
		}
		roots[i] = true
	}
	for i, n := range scene.Nodes {
		if parents[i] < 0 && !roots[i] || n.Mesh < -1 || n.Mesh >= len(scene.Meshes) {
			return nil, fmt.Errorf("document: invalid scene node")
		}
		x := Node{Name: n.Name, Transform: n.Local}
		if parents[i] >= 0 {
			x.Parent = nodeID(parents[i])
		}
		if n.Mesh >= 0 {
			x.Asset = meshID(n.Mesh)
		}
		s.Nodes[nodeID(i)] = x
	}
	if e := s.Validate(); e != nil {
		return nil, e
	}
	// The scene's World fields are descriptive; derive placements from Local.
	return New(s, historyLimit)
}
