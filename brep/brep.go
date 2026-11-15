// Package brep represents closed solids bounded by simple planar polygon faces.
// It preserves shared vertices and edges independently of tessellation. Curved
// surfaces and general Boolean operations are not implemented.
package brep

import (
	"fmt"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Edge joins two distinct vertex indices. Its stored direction is arbitrary.
type Edge struct{ Start, End int }

// EdgeUse orients a shared edge along a face boundary.
type EdgeUse struct {
	Edge     int
	Reversed bool
}

// Face has one simple outer boundary and zero or more disjoint inner wires.
// Holes must have the opposite winding, be strictly inside the outer boundary,
// and neither touch nor contain one another. A strictly intermediate collinear
// vertex subdivides its edge; consecutive edges cannot retrace one another.
// Boundary winding determines the face's outward direction.
type Face struct {
	Boundary []EdgeUse
	Holes    [][]EdgeUse
}

func faceLoops(f Face) [][]EdgeUse { return append([][]EdgeUse{f.Boundary}, f.Holes...) }

// Solid is immutable after construction. Its zero value is the empty solid.
// Accessors return copies, so concurrent readers require no synchronization.
// Validation proves topological manifoldness and individual face validity, not
// freedom from intersections between different faces or correct cavity nesting.
type Solid struct {
	vertices  []geom.Vec3
	edges     []Edge
	faces     []Face
	triangles [][3]int
}

// NewSolid validates and copies an oriented closed planar boundary graph.
// Every edge must have exactly two uses with opposite orientation and every
// vertex and edge must be referenced. Multiple disconnected shells are allowed.
func NewSolid(vertices []geom.Vec3, edges []Edge, faces []Face) (Solid, error) {
	s := Solid{vertices: append([]geom.Vec3(nil), vertices...), edges: append([]Edge(nil), edges...), faces: cloneFaces(faces)}
	m, err := s.buildMesh()
	if err != nil {
		return Solid{}, err
	}
	s.triangles = m.Triangles
	return s, nil
}
func cloneFaces(faces []Face) []Face {
	r := make([]Face, len(faces))
	for i, f := range faces {
		r[i].Boundary = append([]EdgeUse(nil), f.Boundary...)
		r[i].Holes = make([][]EdgeUse, len(f.Holes))
		for j, h := range f.Holes {
			r[i].Holes[j] = append([]EdgeUse(nil), h...)
		}
	}
	return r
}
func (s Solid) Vertices() []geom.Vec3 { return append([]geom.Vec3(nil), s.vertices...) }
func (s Solid) Edges() []Edge         { return append([]Edge(nil), s.edges...) }
func (s Solid) Faces() []Face         { return cloneFaces(s.faces) }

// Tessellate returns a copied mesh with shared vertices. Each polygon face is
// triangulated without approximation; no curved surfaces are represented.
func (s Solid) Tessellate() mesh.Mesh {
	return mesh.Mesh{Vertices: s.Vertices(), Triangles: append([][3]int(nil), s.triangles...)}
}

// TriangleFaces maps each triangle returned by Tessellate to its original
// face index. The map is independent of face winding.
func (s Solid) TriangleFaces() []int {
	result := make([]int, 0, len(s.triangles))
	for i, f := range s.faces {
		count := len(f.Boundary) - 2
		for _, h := range f.Holes {
			count += len(h) + 2
		}
		for range count {
			result = append(result, i)
		}
	}
	return result
}

func (s Solid) Validate() error                          { _, err := s.buildMesh(); return err }
func (s Solid) Bounds() (geom.Box, error)                { return geom.Bounds(s.vertices) }
func (s Solid) MassProperties() (mesh.Properties, error) { return s.Tessellate().MassProperties() }

func (s Solid) faceIndices(face Face) ([]int, error) { return s.wireIndices(face.Boundary) }
func (s Solid) wireIndices(boundary []EdgeUse) ([]int, error) {
	if len(boundary) < 3 {
		return nil, fmt.Errorf("brep: face has fewer than three edges")
	}
	ids := make([]int, len(boundary))
	ends := make([]int, len(ids))
	used := map[int]bool{}
	for i, use := range boundary {
		if use.Edge < 0 || use.Edge >= len(s.edges) {
			return nil, fmt.Errorf("brep: invalid edge reference")
		}
		edge := s.edges[use.Edge]
		a, b := edge.Start, edge.End
		if use.Reversed {
			a, b = b, a
		}
		if used[a] {
			return nil, fmt.Errorf("brep: repeated face vertex")
		}
		used[a] = true
		ids[i], ends[i] = a, b
	}
	for i, b := range ends {
		if b != ids[(i+1)%len(ids)] {
			return nil, fmt.Errorf("brep: disconnected face boundary")
		}
	}
	return ids, nil
}

func (s Solid) buildMesh() (mesh.Mesh, error) {
	m := mesh.Mesh{Vertices: s.vertices}
	if len(s.vertices) == 0 && len(s.edges) == 0 && len(s.faces) == 0 {
		return m, nil
	}
	for _, v := range s.vertices {
		if !v.IsFinite() {
			return mesh.Mesh{}, fmt.Errorf("brep: nonfinite vertex")
		}
	}
	vertexUses := make([]int, len(s.vertices))
	edgeUses := make([]int, len(s.edges))
	edgeSigns := make([]int, len(s.edges))
	pairs := map[[2]int]bool{}
	for _, e := range s.edges {
		if e.Start < 0 || e.End < 0 || e.Start >= len(s.vertices) || e.End >= len(s.vertices) || e.Start == e.End {
			return mesh.Mesh{}, fmt.Errorf("brep: invalid edge")
		}
		if s.vertices[e.Start] == s.vertices[e.End] {
			return mesh.Mesh{}, fmt.Errorf("brep: zero length edge")
		}
		a, b := e.Start, e.End
		if a > b {
			a, b = b, a
		}
		key := [2]int{a, b}
		if pairs[key] {
			return mesh.Mesh{}, fmt.Errorf("brep: duplicate topological edge")
		}
		pairs[key] = true
		vertexUses[e.Start]++
		vertexUses[e.End]++
	}
	for i, n := range vertexUses {
		if n == 0 {
			return mesh.Mesh{}, fmt.Errorf("brep: unused vertex %d", i)
		}
	}
	for i, face := range s.faces {
		var ids []int
		var rings [][]geom.Vec3
		seen := make(map[int]bool)
		for _, loop := range faceLoops(face) {
			ringIDs, err := s.wireIndices(loop)
			if err != nil {
				return mesh.Mesh{}, fmt.Errorf("brep: face %d: %w", i, err)
			}
			ring := make([]geom.Vec3, len(ringIDs))
			for j, id := range ringIDs {
				if seen[id] {
					return mesh.Mesh{}, fmt.Errorf("brep: face %d shares a vertex between wires", i)
				}
				seen[id] = true
				ring[j] = s.vertices[id]
			}
			ids = append(ids, ringIDs...)
			rings = append(rings, ring)
			for _, u := range loop {
				edgeUses[u.Edge]++
				if u.Reversed {
					edgeSigns[u.Edge]--
				} else {
					edgeSigns[u.Edge]++
				}
			}
		}
		triangles, err := triangulateFaceWires(rings)
		if err != nil {
			return mesh.Mesh{}, fmt.Errorf("brep: face %d: %w", i, err)
		}
		for _, t := range triangles {
			m.Triangles = append(m.Triangles, [3]int{ids[t[0]], ids[t[1]], ids[t[2]]})
		}
	}
	for i, n := range edgeUses {
		if n != 2 || edgeSigns[i] != 0 {
			return mesh.Mesh{}, fmt.Errorf("brep: edge %d must have two opposite uses", i)
		}
	}
	if err := m.ValidateSolid(); err != nil {
		return mesh.Mesh{}, fmt.Errorf("brep: %w", err)
	}
	return m, nil
}

// triangulate reuses the mesh polygon validator and ear-clipping implementation.
// A temporary scale-sized extrusion supplies a cap with the original winding.
func triangulate(p []geom.Vec3) ([][3]int, error) {
	origin := p[0]
	scale := 0.0
	for _, v := range p {
		d := v.Sub(origin)
		scale = math.Max(scale, math.Max(math.Abs(d.X), math.Max(math.Abs(d.Y), math.Abs(d.Z))))
	}
	if scale == 0 || math.IsInf(scale, 0) {
		return nil, fmt.Errorf("degenerate face")
	}
	normal := geom.Vec3{}
	for i, v := range p {
		a := v.Sub(origin).Scale(1 / scale)
		b := p[(i+1)%len(p)].Sub(origin).Scale(1 / scale)
		normal = normal.Add(a.Cross(b))
	}
	unit, err := normal.Unit()
	if err != nil {
		return nil, err
	}
	prism, err := mesh.Extrude(p, unit.Scale(scale))
	if err != nil {
		return nil, err
	}
	result := make([][3]int, len(p)-2)
	for i := range result {
		t := prism.Triangles[2*i]
		result[i] = [3]int{t[0], t[2], t[1]}
	}
	return result, nil
}

// FacePlane returns the oriented supporting plane of a face.
func (s Solid) FacePlane(index int) (geom.Plane, error) {
	if index < 0 || index >= len(s.faces) {
		return geom.Plane{}, fmt.Errorf("brep: face index out of range")
	}
	ids, err := s.faceIndices(s.faces[index])
	if err != nil {
		return geom.Plane{}, err
	}
	polygon := make([]geom.Vec3, len(ids))
	for i, id := range ids {
		polygon[i] = s.vertices[id]
	}
	origin := polygon[0]
	normal := geom.Vec3{}
	scale := 0.0
	for _, v := range polygon {
		d := v.Sub(origin)
		scale = math.Max(scale, math.Max(math.Abs(d.X), math.Max(math.Abs(d.Y), math.Abs(d.Z))))
	}
	for i, v := range polygon {
		normal = normal.Add(v.Sub(origin).Scale(1 / scale).Cross(polygon[(i+1)%len(polygon)].Sub(origin).Scale(1 / scale)))
	}
	return geom.NewPlane(origin, normal)
}

// Topology summarizes the polygon graph, not its triangulation. EulerCharacteristic
// is V-E+sum(1-numberOfHoles); each closed orientable shell contributes 2-2*genus.
type Topology struct{ Vertices, Edges, Faces, Shells, EulerCharacteristic int }

func (s Solid) Topology() Topology {
	t := Topology{Vertices: len(s.vertices), Edges: len(s.edges), Faces: len(s.faces)}
	t.EulerCharacteristic = t.Vertices - t.Edges + t.Faces
	incident := make([][]int, len(s.edges))
	for f, face := range s.faces {
		t.EulerCharacteristic -= len(face.Holes)
		for _, loop := range faceLoops(face) {
			for _, u := range loop {
				incident[u.Edge] = append(incident[u.Edge], f)
			}
		}
	}
	visited := make([]bool, len(s.faces))
	for start := range s.faces {
		if visited[start] {
			continue
		}
		t.Shells++
		queue := []int{start}
		for len(queue) > 0 {
			f := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			if visited[f] {
				continue
			}
			visited[f] = true
			for _, loop := range faceLoops(s.faces[f]) {
				for _, u := range loop {
					for _, g := range incident[u.Edge] {
						if !visited[g] {
							queue = append(queue, g)
						}
					}
				}
			}
		}
	}
	return t
}

// Reversed returns an independent solid with all face orientations reversed.
// It is useful for representing cavity shells. Signed volume changes sign.
func (s Solid) Reversed() Solid {
	r := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), triangles: append([][3]int(nil), s.triangles...)}
	for _, face := range r.faces {
		for _, boundary := range faceLoops(face) {
			for j, k := 0, len(boundary)-1; j < k; j, k = j+1, k-1 {
				boundary[j], boundary[k] = boundary[k], boundary[j]
			}
			for j := range boundary {
				boundary[j].Reversed = !boundary[j].Reversed
			}
		}
	}
	for i := range r.triangles {
		r.triangles[i][1], r.triangles[i][2] = r.triangles[i][2], r.triangles[i][1]
	}
	return r
}

// Transformed preserves shared topology and planar polygon faces. Reflections
// reverse face winding so outward boundaries remain outward. Singular transforms
// and transformed geometry outside the supported numerical range are rejected.
func (s Solid) Transformed(transform geom.Affine) (Solid, error) {
	det := transform.Determinant()
	if !transform.IsFinite() || math.IsInf(det, 0) || math.IsNaN(det) || det == 0 {
		return Solid{}, fmt.Errorf("brep: singular or nonfinite transform")
	}
	vertices := s.Vertices()
	for i, v := range vertices {
		vertices[i] = transform.Apply(v)
	}
	faces := s.Faces()
	if det < 0 {
		faces = s.Reversed().faces
	}
	return NewSolid(vertices, s.edges, faces)
}
