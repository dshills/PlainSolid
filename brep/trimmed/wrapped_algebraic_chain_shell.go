package trimmed

import (
	"context"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAlgebraicChainShell is an immutable indexed native open disk with
// three original faces. Its embedding certificate proves global separation;
// its indexed incidence and every exported model-edge/pcurve association are
// independently checked. It is not a closed Solid or a volume claim.
type WrappedAlgebraicChainShell struct {
	embedding     *WrappedAlgebraicChainEmbedding
	vertices      []geom.Vec3
	edges         []parametric.Edge
	faces         []Face
	vertexSources [][]WrappedOpenVertexSource
	edgeSources   [][]WrappedOpenEdgeSource
	boundary      []WrappedOpenBoundaryUse
	operations    int
}

func (s *WrappedAlgebraicChainShell) Embedding() *WrappedAlgebraicChainEmbedding {
	if s == nil {
		return nil
	}
	return s.embedding
}
func (s *WrappedAlgebraicChainShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return append([]geom.Vec3(nil), s.vertices...)
}
func (s *WrappedAlgebraicChainShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return append([]parametric.Edge(nil), s.edges...)
}
func (s *WrappedAlgebraicChainShell) Faces() []Face {
	if s == nil {
		return nil
	}
	out := make([]Face, len(s.faces))
	for i, f := range s.faces {
		out[i] = cloneFace(f)
	}
	return out
}
func (s *WrappedAlgebraicChainShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return append([]WrappedOpenBoundaryUse(nil), s.boundary...)
}
func (s *WrappedAlgebraicChainShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil || i < 0 || i >= len(s.vertexSources) {
		return nil, geom.ErrInvalid
	}
	return append([]WrappedOpenVertexSource(nil), s.vertexSources[i]...), nil
}
func (s *WrappedAlgebraicChainShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil || i < 0 || i >= len(s.edgeSources) {
		return nil, geom.ErrInvalid
	}
	return append([]WrappedOpenEdgeSource(nil), s.edgeSources[i]...), nil
}
func (s *WrappedAlgebraicChainShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// NewWrappedAlgebraicChainShell materializes a previously verified immutable
// chain. MaxWork covers all exported continuous edge/use rechecks, excluding
// the already completed embedding proof. Incidence is bounded by 96 source
// uses. Only certified original mate endpoints and whole edges are identified;
// coordinates are checked for equality after identification, never used to weld.
func NewWrappedAlgebraicChainShell(ctx context.Context, p *WrappedAlgebraicChainEmbedding, o edgecheck.Options) (*WrappedAlgebraicChainShell, error) {
	if ctx == nil || p == nil || p.pairs[0] == nil || p.pairs[1] == nil || len(p.boundary) < 3 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxWork == 0 {
		o.MaxWork = 16000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 4096
	}
	if o.MaxWork < 1 || o.MaxBits < 64 || o.MaxIntervals < 1 {
		return nil, geom.ErrInvalid
	}
	for _, pair := range p.pairs {
		if e := wrappedShellSourceBinding(ctx, pair); e != nil {
			return nil, e
		}
	}
	source := [3]SewFace{p.pairs[0].sources[0].NativeFace(), p.pairs[0].sources[1].NativeFace(), p.pairs[1].sources[1].NativeFace()}
	var offsets [3]int
	total := 0
	for fi, f := range source {
		offsets[fi] = total
		if len(f.Vertices) != len(f.Edges) || len(f.Edges) < 3 || len(f.Edges) > 32 {
			return nil, geom.ErrInvalid
		}
		total += len(f.Vertices)
	}
	parent := make([]int, total)
	for i := range parent {
		parent[i] = i
	}
	root := func(i int) int {
		for parent[i] != i {
			i = parent[i]
		}
		return i
	}
	// Mate identities are expressed in each original source, not pair-local
	// reindexed vertices. The shared middle face is inserted exactly once.
	for pi, pair := range p.pairs {
		for side := 0; side < 2; side++ {
			_, v, e := pair.pair.Endpoint(side)
			if e != nil {
				return nil, e
			}
			a, b := root(offsets[pi]+v[0]), root(offsets[pi+1]+v[1])
			if a == b {
				return nil, geom.ErrDomain
			}
			parent[b] = a
		}
	}
	out := &WrappedAlgebraicChainShell{embedding: p}
	ids := map[int]int{}
	var vertexMap [3][]int
	for fi, f := range source {
		vertexMap[fi] = make([]int, len(f.Vertices))
		for vi, v := range f.Vertices {
			r := root(offsets[fi] + vi)
			id, ok := ids[r]
			if !ok {
				id = len(out.vertices)
				ids[r] = id
				out.vertices = append(out.vertices, v)
				out.vertexSources = append(out.vertexSources, nil)
			} else if out.vertices[id] != v {
				return nil, geom.ErrDomain
			}
			vertexMap[fi][vi] = id
			out.vertexSources[id] = append(out.vertexSources[id], WrappedOpenVertexSource{fi, vi})
		}
	}
	var edgeMap [3][]int
	var reverse [3][]bool
	for fi, f := range source {
		edgeMap[fi] = make([]int, len(f.Edges))
		reverse[fi] = make([]bool, len(f.Edges))
		for ei, edge := range f.Edges {
			id := -1
			rev := false
			if fi > 0 {
				left, _ := p.pairs[fi-1].pair.Owner(0)
				right, _ := p.pairs[fi-1].pair.Owner(1)
				if ei == right.NativeEdge {
					id = edgeMap[fi-1][left.NativeEdge]
					rev = true
				}
			}
			if id < 0 {
				id = len(out.edges)
				edge.Start = vertexMap[fi][edge.Start]
				edge.End = vertexMap[fi][edge.End]
				out.edges = append(out.edges, edge)
				out.edgeSources = append(out.edgeSources, nil)
			}
			edgeMap[fi][ei] = id
			reverse[fi][ei] = rev
			out.edgeSources[id] = append(out.edgeSources[id], WrappedOpenEdgeSource{Face: fi, Edge: ei, Loop: 0, LocalEdge: ei, Reversed: rev})
		}
		face := cloneFace(f.Face)
		for i := range face.Outer {
			use := &face.Outer[i]
			old := use.Edge
			use.Edge = edgeMap[fi][old]
			use.Reversed = use.Reversed != reverse[fi][old]
		}
		out.faces = append(out.faces, face)
	}
	// Reprove every actual exported use, including both sides of each shared
	// model edge, against that face's original lifted pcurve and support.
	for _, face := range out.faces {
		for _, use := range face.Outer {
			if out.operations >= o.MaxWork {
				return nil, geom.ErrConvergence
			}
			opts := o
			opts.MaxWork -= out.operations
			same, work, e := compareWrappedQuadraticUse(ctx, out.edges[use.Edge], face.Surface, use, opts)
			out.operations += work
			if e != nil {
				return nil, e
			}
			if !same {
				return nil, geom.ErrConvergence
			}
		}
	}
	boundary, e := validateWrappedChainIncidence(ctx, out.vertices, out.edges, out.faces, out.vertexSources, out.edgeSources)
	if e != nil {
		return nil, e
	}
	// Match the certified global exterior to the native graph, including source
	// owner labels; reversed face normals reverse the whole physical traversal.
	for i := range p.boundary {
		at := i
		if out.faces[0].Reversed {
			at = len(p.boundary) - 1 - i
		}
		expected := p.boundary[at]
		if i >= len(boundary) || boundary[i].Face != expected.Face || boundary[i].LocalEdge != expected.LocalEdge {
			return nil, geom.ErrDomain
		}
	}
	if len(boundary) != len(p.boundary) {
		return nil, geom.ErrDomain
	}
	out.boundary = boundary
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}

func validateWrappedChainIncidence(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, vs [][]WrappedOpenVertexSource, es [][]WrappedOpenEdgeSource) ([]WrappedOpenBoundaryUse, error) {
	if len(faces) != 3 || len(edges) > 94 || len(vertices) > 92 || len(vertices)-len(edges)+3 != 1 {
		return nil, geom.ErrDomain
	}
	incidence := make([][]WrappedOpenBoundaryUse, len(edges))
	masks := make([]int, len(vertices))
	for fi, f := range faces {
		seen := map[int]bool{}
		first, previous := -1, -1
		for local, use := range f.Outer {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			if use.Edge < 0 || use.Edge >= len(edges) {
				return nil, geom.ErrInvalid
			}
			edge := edges[use.Edge]
			a, b := edge.Start, edge.End
			if use.Reversed {
				a, b = b, a
			}
			if a < 0 || b < 0 || a >= len(vertices) || b >= len(vertices) || a == b || seen[a] {
				return nil, geom.ErrDomain
			}
			seen[a] = true
			masks[a] |= 1 << fi
			if first < 0 {
				first = a
			} else if previous != a {
				return nil, geom.ErrDomain
			}
			previous = b
			if f.Reversed {
				a, b = b, a
			}
			incidence[use.Edge] = append(incidence[use.Edge], WrappedOpenBoundaryUse{Face: fi, LocalEdge: local, Use: use, ReversedTraversal: f.Reversed, Start: a, End: b})
		}
		if previous != first {
			return nil, geom.ErrDomain
		}
	}
	outgoing := map[int]WrappedOpenBoundaryUse{}
	incoming := make([]int, len(vertices))
	shared := 0
	for i, uses := range incidence {
		if len(uses) != len(es[i]) {
			return nil, geom.ErrDomain
		}
		switch len(uses) {
		case 1:
			u := uses[0]
			if _, ok := outgoing[u.Start]; ok {
				return nil, geom.ErrDomain
			}
			outgoing[u.Start] = u
			incoming[u.End]++
		case 2:
			shared++
			a, b := uses[0], uses[1]
			if a.Face+1 != b.Face || a.Start != b.End || a.End != b.Start {
				return nil, geom.ErrDomain
			}
		default:
			return nil, geom.ErrDomain
		}
	}
	if shared != 2 || len(outgoing) != len(vertices) {
		return nil, geom.ErrDomain
	}
	for i, mask := range masks {
		if incoming[i] != 1 || len(vs[i]) < 1 || len(vs[i]) > 2 || !(mask == 1 || mask == 2 || mask == 4 || mask == 3 || mask == 6) {
			return nil, geom.ErrDomain
		}
		ownerMask := 0
		for _, v := range vs[i] {
			ownerMask |= 1 << v.Face
		}
		if ownerMask != mask {
			return nil, geom.ErrDomain
		}
	}
	// Source face zero edge zero is the embedding certificate's deterministic
	// starting edge unless it was cancelled. Choose its first surviving edge.
	start := -1
	for _, u := range outgoing {
		if u.Face == 0 && (start < 0 || u.LocalEdge < outgoing[start].LocalEdge) {
			start = u.Start
		}
	}
	if start < 0 {
		return nil, geom.ErrDomain
	}
	if faces[0].Reversed { // reverse of the positive cycle starts at its last edge
		// The positive first edge's start is the physical first edge's end. Start
		// reversed traversal there, making it agree with reversal of the full list.
		start = outgoing[start].End
	}
	at := start
	var cycle []WrappedOpenBoundaryUse
	for len(cycle) < len(vertices) {
		u, ok := outgoing[at]
		if !ok {
			return nil, geom.ErrDomain
		}
		delete(outgoing, at)
		cycle = append(cycle, u)
		at = u.End
	}
	if at != start || len(outgoing) != 0 {
		return nil, geom.ErrDomain
	}
	return cycle, nil
}
