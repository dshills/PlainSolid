package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type WrappedOpenVertexSource struct{ Face, Vertex int }
type WrappedOpenEdgeSource struct {
	Face, Edge, Loop, LocalEdge int
	// Reversed maps original source edge parameter to the stored global edge.
	Reversed bool
}

// WrappedOpenBoundaryUse retains an original face-local pcurve. Start/End
// are global vertices in oriented boundary traversal. If ReversedTraversal,
// evaluate Use.Curve at1-t (UV endpoints remain in authored order).
type WrappedOpenBoundaryUse struct {
	Face, LocalEdge   int
	Use               Use
	ReversedTraversal bool
	Start, End        int
}

// WrappedAlgebraicOpenShell is an immutable oriented native incidence shell
// assembled from two embedded source disks and one exact complete edge mate.
// It proves local manifold incidence and one combinatorial exterior cycle.
// It does NOT prove global nonoverlap of the face interiors or other edges,
// and is not a closed Solid or a material union/volume certificate.
type WrappedAlgebraicOpenShell struct {
	vertices      []geom.Vec3
	edges         []parametric.Edge
	faces         []Face
	vertexSources [][]WrappedOpenVertexSource
	edgeSources   [][]WrappedOpenEdgeSource
	boundary      []WrappedOpenBoundaryUse
	pair          *WrappedAlgebraicEdgePair
	operations    int
}

func (s *WrappedAlgebraicOpenShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return append([]geom.Vec3(nil), s.vertices...)
}
func (s *WrappedAlgebraicOpenShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return append([]parametric.Edge(nil), s.edges...)
}
func (s *WrappedAlgebraicOpenShell) Faces() []Face {
	if s == nil {
		return nil
	}
	out := make([]Face, len(s.faces))
	for i, f := range s.faces {
		out[i] = cloneFace(f)
	}
	return out
}
func (s *WrappedAlgebraicOpenShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return append([]WrappedOpenBoundaryUse(nil), s.boundary...)
}
func (s *WrappedAlgebraicOpenShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil || i < 0 || i >= len(s.vertexSources) {
		return nil, geom.ErrInvalid
	}
	return append([]WrappedOpenVertexSource(nil), s.vertexSources[i]...), nil
}
func (s *WrappedAlgebraicOpenShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil || i < 0 || i >= len(s.edgeSources) {
		return nil, geom.ErrInvalid
	}
	return append([]WrappedOpenEdgeSource(nil), s.edgeSources[i]...), nil
}
func (s *WrappedAlgebraicOpenShell) Pair() *WrappedAlgebraicEdgePair {
	if s == nil {
		return nil
	}
	return s.pair
}
func (s *WrappedAlgebraicOpenShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// NewWrappedAlgebraicOpenShell supports exactly two single-loop source faces,
// at most32 edges each, and one declared shared edge. Only independently proved
// endpoint mates are identified. Additional equal native vertices reject as
// undeclared contact, even if another construction could validly join them.
// Original supports, lifted pcurves and source indices are retained. MaxWork
// covers arithmetic proofs; incidence work is additionally bounded by64 uses.
func NewWrappedAlgebraicOpenShell(ctx context.Context, a, b *WrappedAlgebraicQuadraticFace, edgeA, edgeB [2]int, reverseB bool, o edgecheck.Options) (*WrappedAlgebraicOpenShell, error) {
	if ctx == nil || a == nil || b == nil || a.face == nil || b.face == nil {
		return nil, geom.ErrInvalid
	}
	if edgeA[0] != 0 || edgeB[0] != 0 || len(a.face.loops) != 1 || len(b.face.loops) != 1 {
		return nil, geom.ErrConvergence
	}
	if o.MaxWork == 0 {
		o.MaxWork = 8000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 4096
	}
	pair, e := CertifyWrappedAlgebraicEdgePair(ctx, a, b, edgeA, edgeB, reverseB, o)
	if e != nil {
		return nil, e
	}
	used := pair.ExactOperations()
	source := [2]SewFace{a.NativeFace(), b.NativeFace()}
	out := &WrappedAlgebraicOpenShell{pair: pair}
	for _, f := range source {
		if len(f.Edges) < 3 || len(f.Edges) > 32 || len(f.Vertices) != len(f.Edges) || len(f.Face.Outer) != len(f.Edges) || len(f.Face.Holes) != 0 {
			return nil, geom.ErrConvergence
		}
	}
	mate := map[int]int{}
	for side := 0; side < 2; side++ {
		_, vertices, e := pair.Endpoint(side)
		if e != nil {
			return nil, e
		}
		mate[vertices[1]] = vertices[0]
	}
	if len(mate) != 2 {
		return nil, geom.ErrConvergence
	}
	// Exact represented coordinates are used only to reject undeclared identity,
	// never to merge geometry without the original complete edge certificate.
	for owner, f := range source {
		for i, v := range f.Vertices {
			for j := 0; j < i; j++ {
				if v == f.Vertices[j] {
					return nil, geom.ErrConvergence
				}
			}
			if owner == 1 {
				for j, w := range source[0].Vertices {
					if v == w {
						expected, ok := mate[i]
						if !ok || expected != j {
							return nil, geom.ErrConvergence
						}
					}
				}
			}
		}
	}
	maps := [2][]int{make([]int, len(source[0].Vertices)), make([]int, len(source[1].Vertices))}
	for owner, f := range source {
		for i, v := range f.Vertices {
			index := -1
			if owner == 1 {
				if other, ok := mate[i]; ok {
					index = maps[0][other]
				}
			}
			if index < 0 {
				index = len(out.vertices)
				out.vertices = append(out.vertices, v)
				out.vertexSources = append(out.vertexSources, nil)
			} else if out.vertices[index] != v {
				return nil, geom.ErrDomain
			}
			maps[owner][i] = index
			out.vertexSources[index] = append(out.vertexSources[index], WrappedOpenVertexSource{owner, i})
		}
	}
	oa, _ := pair.Owner(0)
	ob, _ := pair.Owner(1)
	edgeMaps := [2][]int{make([]int, len(source[0].Edges)), make([]int, len(source[1].Edges))}
	for owner, f := range source {
		for i, edge := range f.Edges {
			if edge.Start < 0 || edge.End < 0 || edge.Start >= len(maps[owner]) || edge.End >= len(maps[owner]) {
				return nil, geom.ErrInvalid
			}
			index := -1
			reversed := false
			if owner == 1 && i == ob.NativeEdge {
				index = edgeMaps[0][oa.NativeEdge]
				reversed = reverseB
			}
			if index < 0 {
				index = len(out.edges)
				edge.Start = maps[owner][edge.Start]
				edge.End = maps[owner][edge.End]
				out.edges = append(out.edges, edge)
				out.edgeSources = append(out.edgeSources, nil)
			}
			edgeMaps[owner][i] = index
			out.edgeSources[index] = append(out.edgeSources[index], WrappedOpenEdgeSource{Face: owner, Edge: i, Loop: 0, LocalEdge: i, Reversed: reversed})
		}
		face := cloneFace(f.Face)
		for i := range face.Outer {
			use := &face.Outer[i]
			if use.Edge < 0 || use.Edge >= len(edgeMaps[owner]) {
				return nil, geom.ErrInvalid
			}
			old := use.Edge
			use.Edge = edgeMaps[owner][old]
			if owner == 1 && old == ob.NativeEdge {
				use.Reversed = use.Reversed != reverseB
			}
		}
		out.faces = append(out.faces, face)
	}
	shared := edgeMaps[0][oa.NativeEdge]
	// Re-prove the actual exported shared edge against B's original pcurve,
	// after indexed direction remapping; no stored association flag is trusted.
	if used >= o.MaxWork {
		return nil, geom.ErrConvergence
	}
	same, work, e := compareWrappedQuadraticUse(ctx, out.edges[shared], out.faces[1].Surface, out.faces[1].Outer[edgeB[1]], edgecheck.Options{MaxWork: o.MaxWork - used, MaxBits: o.MaxBits, MaxIntervals: o.MaxIntervals})
	used += work
	if e != nil {
		return nil, e
	}
	if !same {
		return nil, geom.ErrConvergence
	}
	boundary, e := validateWrappedAlgebraicPairIncidence(ctx, len(out.vertices), out.edges, out.faces, shared)
	if e != nil {
		return nil, e
	}
	out.boundary = boundary
	out.operations = used
	return out, nil
}

// Two disk faces joined along one entire edge have interval vertex links at
// the two merged endpoints; all other links belong to one source disk. This
// validator checks that exact combinatorial condition instead of inferring it
// from Euler characteristic alone. Face-local UVs need not share a chart.
func validateWrappedAlgebraicPairIncidence(ctx context.Context, nv int, edges []parametric.Edge, faces []Face, shared int) ([]WrappedOpenBoundaryUse, error) {
	if ctx == nil || nv < 4 || nv > 62 || len(edges) < 5 || len(edges) > 63 || len(faces) != 2 || shared < 0 || shared >= len(edges) {
		return nil, geom.ErrInvalid
	}
	incidence := make([][]WrappedOpenBoundaryUse, len(edges))
	vertexFaces := make([]uint8, nv)
	for fi, face := range faces {
		if len(face.Holes) != 0 || len(face.Outer) < 3 || len(face.Outer) > 32 {
			return nil, geom.ErrConvergence
		}
		seenEdges := map[int]bool{}
		seenVertices := map[int]bool{}
		first, previous := -1, -1
		for local, use := range face.Outer {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			if use.Edge < 0 || use.Edge >= len(edges) || seenEdges[use.Edge] {
				return nil, geom.ErrConvergence
			}
			seenEdges[use.Edge] = true
			edge := edges[use.Edge]
			start, end := edge.Start, edge.End
			if use.Reversed {
				start, end = end, start
			}
			if start < 0 || end < 0 || start >= nv || end >= nv || start == end || seenVertices[start] {
				return nil, geom.ErrConvergence
			}
			seenVertices[start] = true
			vertexFaces[start] |= 1 << uint(fi)
			if first < 0 {
				first = start
			} else if previous != start {
				return nil, geom.ErrConvergence
			}
			previous = end
			if face.Reversed {
				start, end = end, start
			}
			incidence[use.Edge] = append(incidence[use.Edge], WrappedOpenBoundaryUse{Face: fi, LocalEdge: local, Use: use, ReversedTraversal: face.Reversed, Start: start, End: end})
		}
		if previous != first {
			return nil, geom.ErrConvergence
		}
	}
	boundary := map[int]WrappedOpenBoundaryUse{}
	incoming := make([]int, nv)
	for i, uses := range incidence {
		if i == shared {
			if len(uses) != 2 || uses[0].Face == uses[1].Face || uses[0].Start != uses[1].End || uses[0].End != uses[1].Start {
				return nil, geom.ErrConvergence
			}
			continue
		}
		if len(uses) != 1 {
			return nil, geom.ErrConvergence
		}
		use := uses[0]
		if _, exists := boundary[use.Start]; exists {
			return nil, geom.ErrConvergence
		}
		boundary[use.Start] = use
		incoming[use.End]++
	}
	joint := edges[shared]
	for v, owners := range vertexFaces {
		if incoming[v] != 1 {
			return nil, geom.ErrConvergence
		}
		if v == joint.Start || v == joint.End {
			if owners != 3 {
				return nil, geom.ErrConvergence
			}
		} else if owners != 1 && owners != 2 {
			return nil, geom.ErrConvergence
		}
	}
	if nv-len(edges)+len(faces) != 1 || len(boundary) != nv {
		return nil, geom.ErrConvergence
	}
	// All vertices are on this one disk boundary; choose vertex0 deterministically.
	at := 0
	var cycle []WrappedOpenBoundaryUse
	for len(cycle) < nv {
		use, ok := boundary[at]
		if !ok {
			return nil, geom.ErrConvergence
		}
		cycle = append(cycle, use)
		delete(boundary, at)
		at = use.End
	}
	if at != 0 || len(boundary) != 0 {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return cycle, nil
}
