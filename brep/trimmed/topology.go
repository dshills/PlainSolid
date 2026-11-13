package trimmed

import (
	"context"
	"fmt"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Shell is an edge-connected component of the oriented boundary. Indices
// refer to the solid and are returned in increasing order.
// A pole edge counts as an auxiliary side, not as a physical edge in Euler.
type Shell struct {
	Faces, Edges, Vertices []int
	PoleEdges              []int
	EulerCharacteristic    int
}

// Topology summarizes shared identity, shells and face holes. It does not
// infer cavity nesting or classify inter-shell contacts.
type Topology struct {
	Vertices, Edges, Faces, CollapsedEdges, InnerWires int
	EulerCharacteristic                                int
	Shells                                             []Shell
}

// Topology returns a caller-owned incidence report. Every collapsed pole is
// recorded, and its auxiliary edge is excluded from the Euler characteristic.
func (s Solid) Topology() Topology {
	t := Topology{Vertices: len(s.vertices), Edges: len(s.edges), Faces: len(s.faces)}
	incident := make([][]int, len(s.edges))
	for i, f := range s.faces {
		t.InnerWires += len(f.Holes)
		for _, l := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range l {
				incident[u.Edge] = append(incident[u.Edge], i)
			}
		}
	}
	seen := make([]bool, len(s.faces))
	for i := range s.faces {
		if seen[i] {
			continue
		}
		seen[i] = true
		stack := []int{i}
		faceSet, edgeSet, vertexSet := []int{}, map[int]bool{}, map[int]bool{}
		holes := 0
		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			faceSet = append(faceSet, f)
			holes += len(s.faces[f].Holes)
			for _, l := range append([][]Use{s.faces[f].Outer}, s.faces[f].Holes...) {
				for _, u := range l {
					edgeSet[u.Edge] = true
					e := s.edges[u.Edge]
					vertexSet[e.Start], vertexSet[e.End] = true, true
					for _, j := range incident[u.Edge] {
						if !seen[j] {
							seen[j] = true
							stack = append(stack, j)
						}
					}
				}
			}
		}
		h := Shell{Faces: faceSet}
		for e := range edgeSet {
			h.Edges = append(h.Edges, e)
			if s.edges[e].Curve == nil {
				h.PoleEdges = append(h.PoleEdges, e)
			}
		}
		for v := range vertexSet {
			h.Vertices = append(h.Vertices, v)
		}
		sort.Ints(h.Faces)
		sort.Ints(h.Edges)
		sort.Ints(h.PoleEdges)
		sort.Ints(h.Vertices)
		h.EulerCharacteristic = len(h.Vertices) - len(h.Edges) + len(h.PoleEdges) + len(h.Faces) - holes
		t.EulerCharacteristic += h.EulerCharacteristic
		t.CollapsedEdges += len(h.PoleEdges)
		t.Shells = append(t.Shells, h)
	}
	return t
}

// Tolerance returns the model-space edge agreement tolerance.
func (s Solid) Tolerance() float64 { return s.tolerance }

// Shell returns one shell as an independent oriented boundary. Orientation is
// unchanged; a cavity shell remains inward and may have negative signed volume.
func (s Solid) Shell(index int) (Solid, error) {
	return s.ShellContext(context.Background(), index)
}

// ShellContext extracts one indexed shell with cooperative cancellation.
func (s Solid) ShellContext(ctx context.Context, index int) (Solid, error) {
	if ctx == nil {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	t := s.Topology()
	if index < 0 || index >= len(t.Shells) {
		return Solid{}, fmt.Errorf("trimmed: shell index out of range")
	}
	x := t.Shells[index]
	vertices := make(map[int]int, len(x.Vertices))
	edges := make(map[int]int, len(x.Edges))
	v := make([]geom.Vec3, len(x.Vertices))
	e := make([]parametric.Edge, len(x.Edges))
	f := make([]Face, len(x.Faces))
	for j, i := range x.Vertices {
		vertices[i], v[j] = j, s.vertices[i]
	}
	for j, i := range x.Edges {
		edges[i], e[j] = j, s.edges[i]
		e[j].Start, e[j].End = vertices[e[j].Start], vertices[e[j].End]
	}
	for j, i := range x.Faces {
		f[j] = cloneFace(s.faces[i])
		for _, l := range append([][]Use{f[j].Outer}, f[j].Holes...) {
			for k := range l {
				l[k].Edge = edges[l[k].Edge]
			}
		}
	}
	return NewSolidContext(ctx, v, e, f, s.tolerance)
}
