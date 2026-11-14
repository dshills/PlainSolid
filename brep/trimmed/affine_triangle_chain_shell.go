package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AffineTriangleChainShell is an embedded three-face open disk with a connected
// boundary vertex fan. Only the two whole mates and their common original
// endpoint are identified. It is neither a closed shell nor a solid.
type AffineTriangleChainShell struct {
	pairs      [2]*AffineTriangleMateShell
	outer      *geom.SurfaceAffineTriangleVertexContact
	graph      *WrappedAlgebraicOpenShell
	operations int
}

func (s *AffineTriangleChainShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.graph.Vertices()
}
func (s *AffineTriangleChainShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.graph.Edges()
}
func (s *AffineTriangleChainShell) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.graph.Faces()
}
func (s *AffineTriangleChainShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.graph.Boundary()
}
func (s *AffineTriangleChainShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.VertexSources(i)
}
func (s *AffineTriangleChainShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.EdgeSources(i)
}
func (s *AffineTriangleChainShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}
func (s *AffineTriangleChainShell) OuterContact() *geom.SurfaceAffineTriangleVertexContact {
	if s == nil {
		return nil
	}
	return s.outer
}

// NewAffineTriangleChainShell independently rebuilds both native pair shells,
// matches the original middle triangle by exact UV vertices, and binds the
// outer vertex-only proof to those same original triangles. MaxWork covers
// both pair reconstructions and nine further actual native-use reproofs.
func NewAffineTriangleChainShell(ctx context.Context, ab, bc *geom.SurfaceAffineTriangleMate, outer *geom.SurfaceAffineTriangleVertexContact, o AffineTriangleShellOptions) (*AffineTriangleChainShell, error) {
	if ctx == nil || ab == nil || bc == nil || outer == nil {
		return nil, geom.ErrInvalid
	}
	if o.MaxWork == 0 {
		o.MaxWork = 64000000
	}
	if o.Proof.MaxWork == 0 {
		o.Proof.MaxWork = 1000000
	}
	if o.Proof.MaxBits == 0 {
		o.Proof.MaxBits = 16384
	}
	if o.MaxWork < 1 || o.Proof.MaxWork < 1 || o.Proof.MaxBits < 64 {
		return nil, geom.ErrInvalid
	}
	a, ta, _ := ab.Source(0)
	b, tb, _ := ab.Source(1)
	bb, tbb, _ := bc.Source(0)
	c, tc, _ := bc.Source(1)
	oa, toa, _ := outer.Source(0)
	oc, toc, _ := outer.Source(1)
	if b != bb || a != oa || c != oc || a == b || a == c || b == c {
		return nil, geom.ErrDomain
	}
	match := func(x, y [3][2]*big.Rat) ([3]int, bool) {
		var m [3]int
		seen := [3]bool{}
		for i := range x {
			found := -1
			for j := range y {
				if x[i][0].Cmp(y[j][0]) == 0 && x[i][1].Cmp(y[j][1]) == 0 {
					found = j
					break
				}
			}
			if found < 0 || seen[found] {
				return m, false
			}
			m[i] = found
			seen[found] = true
		}
		return m, true
	}
	middle, ok := match(tbb, tb)
	if !ok {
		return nil, geom.ErrDomain
	}
	am, ok := match(toa, ta)
	if !ok {
		return nil, geom.ErrDomain
	}
	cm, ok := match(toc, tc)
	if !ok {
		return nil, geom.ErrDomain
	}
	// The two distinct middle edges share exactly one vertex. Verify that this
	// is precisely the original endpoint certified by the outer point proof.
	shared := -1
	for _, v := range middle[:2] {
		if v < 2 {
			if shared >= 0 {
				return nil, geom.ErrDomain
			}
			shared = v
		}
	}
	if shared < 0 {
		return nil, geom.ErrDomain
	}
	cshared := 0
	if middle[1] == shared {
		cshared = 1
	}
	ov := outer.Vertices()
	if am[ov[0]] != shared || cm[ov[1]] != cshared {
		return nil, geom.ErrDomain
	}
	out := &AffineTriangleChainShell{outer: outer, graph: &WrappedAlgebraicOpenShell{}}
	for i, contact := range []*geom.SurfaceAffineTriangleMate{ab, bc} {
		if out.operations >= o.MaxWork {
			return nil, geom.ErrConvergence
		}
		opts := o
		opts.MaxWork -= out.operations
		p, e := NewAffineTriangleMateShell(ctx, contact, opts)
		if e != nil {
			return nil, e
		}
		out.operations += p.operations
		out.pairs[i] = p
	}
	// Original owner-local vertex IDs are A=AB.A, B=AB.B, C=BC.B.
	maps := [3][3]int{{0, 1, 2}, {0, 1, 3}, {-1, -1, 4}}
	maps[2][0] = maps[1][middle[0]]
	maps[2][1] = maps[1][middle[1]]
	g := out.graph
	g.vertices = append(g.vertices, out.pairs[0].graph.vertices...)
	g.vertices = append(g.vertices, out.pairs[1].graph.vertices[3])
	g.vertexSources = make([][]WrappedOpenVertexSource, 5)
	for owner, m := range maps {
		for v, index := range m {
			g.vertexSources[index] = append(g.vertexSources[index], WrappedOpenVertexSource{Face: owner, Vertex: v})
		}
	}
	patches := [3]*geom.SurfacePatch{a, b, c}
	triangles := [3][3][2]*big.Rat{ta, tb, tc}
	for owner := 0; owner < 3; owner++ {
		// Reuse independently accepted source adapters, never rebuild float poles.
		surface := out.pairs[0].graph.faces[0].Surface
		if owner == 1 {
			surface = out.pairs[0].graph.faces[1].Surface
		}
		if owner == 2 {
			surface = out.pairs[1].graph.faces[1].Surface
		}
		if surface.(*geom.ExactTensorSurface).ExactPatch() != patches[owner] {
			return nil, geom.ErrDomain
		}
		var uv [3]geom.Vec2
		for i, v := range triangles[owner] {
			x, xok := v[0].Float64()
			y, yok := v[1].Float64()
			if !xok || !yok {
				return nil, geom.ErrUnrepresentable
			}
			uv[i] = geom.Vec2{X: x, Y: y}
		}
		var indices [3]int
		var reversed [3]bool
		for local := 0; local < 3; local++ {
			start, end := maps[owner][local], maps[owner][(local+1)%3]
			found := -1
			for ei, e := range g.edges {
				if e.Start == start && e.End == end || e.Start == end && e.End == start {
					found = ei
					reversed[local] = e.Start != start
					break
				}
			}
			if found >= 0 {
				// A repeated edge must be one of the two explicitly proved whole mates.
				allowed := owner == 1 && local == 0 || owner == 2 && local == 0
				if !allowed {
					return nil, geom.ErrDomain
				}
				indices[local] = found
				g.edgeSources[found] = append(g.edgeSources[found], WrappedOpenEdgeSource{Face: owner, Edge: local, Loop: 0, LocalEdge: local, Reversed: reversed[local]})
			} else {
				indices[local] = len(g.edges)
				g.edges = append(g.edges, parametric.Edge{Start: start, End: end, Curve: surfaceTrimCurve{surface: surface, a: uv[local], b: uv[(local+1)%3]}, Range: parametric.Interval{Max: 1}})
				g.edgeSources = append(g.edgeSources, []WrappedOpenEdgeSource{{Face: owner, Edge: local, Loop: 0, LocalEdge: local}})
			}
		}
		// Orientation is inherited from the already proved pair's CCW source loop.
		// Derive authored order from which original edge 0 traversal its pair uses.
		sourceFace := out.pairs[0].graph.faces[owner%2]
		if owner == 2 {
			sourceFace = out.pairs[1].graph.faces[1]
		}
		positive := false
		for _, u := range sourceFace.Outer {
			if u.Edge == 0 {
				positive = !u.Reversed
			}
		}
		order := []int{0, 1, 2}
		if !positive {
			order = []int{2, 1, 0}
		}
		face := Face{Surface: surface}
		for _, i := range order {
			x, y := uv[i], uv[(i+1)%3]
			rev := reversed[i]
			if !positive {
				x, y = y, x
				rev = !rev
			}
			face.Outer = append(face.Outer, Use{Edge: indices[i], Reversed: rev, UVStart: x, UVEnd: y})
		}
		g.faces = append(g.faces, face)
	}
	if len(g.edges) != 7 {
		return nil, geom.ErrDomain
	}
	for _, f := range g.faces {
		for _, u := range f.Outer {
			if o.Proof.MaxWork > o.MaxWork-out.operations {
				return nil, geom.ErrConvergence
			}
			same, e := compareTriangleNativeUse(ctx, g.edges[u.Edge], f.Surface, u, o.Proof)
			if e != nil {
				return nil, e
			}
			if !same {
				return nil, geom.ErrDomain
			}
			out.operations += o.Proof.MaxWork
		}
	}
	boundary, e := validateTriangleChainIncidence(ctx, g)
	if e != nil {
		return nil, e
	}
	g.boundary = boundary
	return out, nil
}

func validateTriangleChainIncidence(ctx context.Context, g *WrappedAlgebraicOpenShell) ([]WrappedOpenBoundaryUse, error) {
	if len(g.vertices) != 5 || len(g.edges) != 7 || len(g.faces) != 3 {
		return nil, geom.ErrInvalid
	}
	uses := make([][]WrappedOpenBoundaryUse, 7)
	incident := make([]map[int]bool, 5)
	for i := range incident {
		incident[i] = map[int]bool{}
	}
	for fi, f := range g.faces {
		if len(f.Outer) != 3 || len(f.Holes) != 0 || f.Reversed {
			return nil, geom.ErrDomain
		}
		first, previous := -1, -1
		seen := map[int]bool{}
		for local, u := range f.Outer {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			if u.Edge < 0 || u.Edge >= 7 {
				return nil, geom.ErrDomain
			}
			edge := g.edges[u.Edge]
			a, b := edge.Start, edge.End
			if u.Reversed {
				a, b = b, a
			}
			if a < 0 || a >= 5 || b < 0 || b >= 5 || a == b || seen[a] {
				return nil, geom.ErrDomain
			}
			seen[a] = true
			if previous >= 0 && previous != a {
				return nil, geom.ErrDomain
			}
			if first < 0 {
				first = a
			}
			previous = b
			incident[a][fi] = true
			incident[b][fi] = true
			uses[u.Edge] = append(uses[u.Edge], WrappedOpenBoundaryUse{Face: fi, LocalEdge: local, Use: u, Start: a, End: b})
		}
		if previous != first {
			return nil, geom.ErrDomain
		}
	}
	next := map[int]WrappedOpenBoundaryUse{}
	incoming := map[int]int{}
	shared := 0
	for _, u := range uses {
		if len(u) == 1 {
			if _, ok := next[u[0].Start]; ok {
				return nil, geom.ErrDomain
			}
			next[u[0].Start] = u[0]
			incoming[u[0].End]++
		} else if len(u) == 2 {
			shared++
			if u[0].Start != u[1].End || u[0].End != u[1].Start || u[0].Face == u[1].Face {
				return nil, geom.ErrDomain
			}
		} else {
			return nil, geom.ErrDomain
		}
	}
	if shared != 2 || len(next) != 5 {
		return nil, geom.ErrDomain
	}
	for vertex, owners := range incident {
		if incoming[vertex] != 1 {
			return nil, geom.ErrDomain
		}
		links := map[int][]int{}
		count := 0
		for ei, u := range uses {
			e := g.edges[ei]
			if len(u) == 2 && (e.Start == vertex || e.End == vertex) {
				a, b := u[0].Face, u[1].Face
				links[a] = append(links[a], b)
				links[b] = append(links[b], a)
				count++
			}
		}
		if count != len(owners)-1 {
			return nil, geom.ErrDomain
		}
		start := -1
		for f := range owners {
			start = f
			if len(links[f]) > 2 {
				return nil, geom.ErrDomain
			}
		}
		visited := map[int]bool{}
		stack := []int{start}
		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[f] {
				continue
			}
			visited[f] = true
			stack = append(stack, links[f]...)
		}
		if len(visited) != len(owners) {
			return nil, geom.ErrDomain
		}
	}
	var boundary []WrappedOpenBoundaryUse
	v := 0
	seen := map[int]bool{}
	for i := 0; i < 5; i++ {
		if seen[v] {
			return nil, geom.ErrDomain
		}
		seen[v] = true
		u, ok := next[v]
		if !ok {
			return nil, geom.ErrDomain
		}
		boundary = append(boundary, u)
		v = u.End
	}
	if v != 0 {
		return nil, geom.ErrDomain
	}
	return boundary, nil
}
