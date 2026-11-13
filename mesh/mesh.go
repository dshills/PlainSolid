// Package mesh provides indexed triangle meshes and faceted solid construction.
// Curved primitives are polygonal approximations; they are not analytic B-reps.
package mesh

import (
	"fmt"
	"math"
	"plainsolid/geom"
)

// Mesh is an indexed, oriented triangle mesh. Callers own its mutable slices.
// An empty mesh represents the empty solid. Vertices need not all be referenced.
type Mesh struct {
	Vertices  []geom.Vec3
	Triangles [][3]int
}

// Validate checks finite coordinates, valid indices and nondegenerate triangles.
// It permits open surfaces and does not detect geometric self-intersections.
func (m Mesh) Validate() error {
	for i, v := range m.Vertices {
		if !v.IsFinite() {
			return fmt.Errorf("mesh: vertex %d is nonfinite", i)
		}
	}
	for i, t := range m.Triangles {
		for _, j := range t {
			if j < 0 || j >= len(m.Vertices) {
				return fmt.Errorf("mesh: triangle %d index out of range", i)
			}
		}
		if t[0] == t[1] || t[1] == t[2] || t[2] == t[0] {
			return fmt.Errorf("mesh: triangle %d repeats a vertex", i)
		}
		n := m.Vertices[t[1]].Sub(m.Vertices[t[0]]).Cross(m.Vertices[t[2]].Sub(m.Vertices[t[0]]))
		if !n.IsFinite() || !finite(n.Length()) || n.Length() == 0 {
			return fmt.Errorf("mesh: triangle %d is degenerate or overflows", i)
		}
	}
	return nil
}

type edge struct{ a, b int }
type use struct{ face, sign int }

// ValidateSolid checks a closed, consistently oriented topological 2-manifold,
// including connected vertex fans. Disconnected shells are permitted. It does
// not prove absence of self-intersections, outward orientation, or cavity nesting.
func (m Mesh) ValidateSolid() error {
	if err := m.Validate(); err != nil {
		return err
	}
	edges := make(map[edge][]use)
	incident := make(map[int][]int)
	for f, t := range m.Triangles {
		for i, a := range t {
			b := t[(i+1)%3]
			k := edge{a, b}
			s := 1
			if a > b {
				k = edge{b, a}
				s = -1
			}
			edges[k] = append(edges[k], use{f, s})
			incident[a] = append(incident[a], f)
		}
	}
	adjacent := make(map[int][]int)
	for e, u := range edges {
		if len(u) != 2 || u[0].sign == u[1].sign {
			return fmt.Errorf("mesh: edge (%d,%d) is not closed and consistently oriented", e.a, e.b)
		}
		adjacent[u[0].face] = append(adjacent[u[0].face], u[1].face)
		adjacent[u[1].face] = append(adjacent[u[1].face], u[0].face)
	}
	for v, faces := range incident {
		allowed := make(map[int]bool, len(faces))
		for _, f := range faces {
			allowed[f] = true
		}
		visited := make(map[int]bool)
		stack := []int{faces[0]}
		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[f] {
				continue
			}
			visited[f] = true
			for _, g := range adjacent[f] {
				if allowed[g] && !visited[g] {
					stack = append(stack, g)
				}
			}
		}
		if len(visited) != len(allowed) {
			return fmt.Errorf("mesh: disconnected fan at vertex %d", v)
		}
	}
	return nil
}

// Area returns the total triangle area. Validate before calling on untrusted data.
func (m Mesh) Area() float64 {
	sum, c := 0.0, 0.0
	for _, t := range m.Triangles {
		a, b, d := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		x := b.Sub(a).Cross(d.Sub(a)).Length()/2 - c
		y := sum + x
		c = (y - sum) - x
		sum = y
	}
	return sum
}

// SignedVolume returns oriented enclosed volume. The input must be closed;
// outward faces contribute positive volume. No validation is performed.
func (m Mesh) SignedVolume() float64 {
	if len(m.Triangles) == 0 {
		return 0
	}
	origin := m.Vertices[m.Triangles[0][0]]
	sum, c := 0.0, 0.0
	for _, t := range m.Triangles {
		a, b, d := m.Vertices[t[0]].Sub(origin), m.Vertices[t[1]].Sub(origin), m.Vertices[t[2]].Sub(origin)
		x := a.Dot(b.Cross(d))/6 - c
		y := sum + x
		c = (y - sum) - x
		sum = y
	}
	return sum
}

// Reversed copies the mesh and reverses all triangle orientations.
func (m Mesh) Reversed() Mesh {
	r := Mesh{Vertices: append([]geom.Vec3(nil), m.Vertices...), Triangles: append([][3]int(nil), m.Triangles...)}
	for i := range r.Triangles {
		r.Triangles[i][1], r.Triangles[i][2] = r.Triangles[i][2], r.Triangles[i][1]
	}
	return r
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// Bounds returns bounds of all vertices, including any unused ones. Empty meshes
// return an error because they have no finite bounding box.
func (m Mesh) Bounds() (geom.Box, error) { return geom.Bounds(m.Vertices) }

// Transformed creates a transformed copy. Singular transforms are rejected;
// reflections reverse winding to preserve the orientation of solid boundaries.
func (m Mesh) Transformed(transform geom.Affine) (Mesh, error) {
	if err := m.Validate(); err != nil {
		return Mesh{}, err
	}
	det := transform.Determinant()
	if !transform.IsFinite() || !finite(det) || det == 0 {
		return Mesh{}, fmt.Errorf("mesh: singular or nonfinite transform")
	}
	r := Mesh{Vertices: make([]geom.Vec3, len(m.Vertices)), Triangles: append([][3]int(nil), m.Triangles...)}
	for i, v := range m.Vertices {
		r.Vertices[i] = transform.Apply(v)
	}
	if det < 0 {
		for i := range r.Triangles {
			r.Triangles[i][1], r.Triangles[i][2] = r.Triangles[i][2], r.Triangles[i][1]
		}
	}
	return r, r.Validate()
}
