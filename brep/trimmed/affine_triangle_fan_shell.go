package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AffineTriangleFanShell is an embedded three-face open disk with a circular
// central vertex link. Every pair of original supports has its own complete
// mate/separation certificate. No closed material volume is claimed.
type AffineTriangleFanShell struct {
	pairs              []*AffineTriangleMateShell
	graph              *WrappedAlgebraicFaceChainShell
	center, operations int
}

func (s *AffineTriangleFanShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.graph.Vertices()
}
func (s *AffineTriangleFanShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.graph.Edges()
}
func (s *AffineTriangleFanShell) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.graph.Faces()
}
func (s *AffineTriangleFanShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.graph.Boundary()
}
func (s *AffineTriangleFanShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.VertexSources(i)
}
func (s *AffineTriangleFanShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.EdgeSources(i)
}
func (s *AffineTriangleFanShell) CenterVertex() int {
	if s == nil {
		return -1
	}
	return s.center
}
func (s *AffineTriangleFanShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// NewAffineTriangleFanShell takes cyclic mates F0/F1, F1/F2 and F2/F0.
// Each owner's entire original triangle must match between declarations, up to
// exact vertex permutation. Only declared mate endpoints/edges are identified.
// All original charts are independently projected-injective; all actual native
// model uses and the connected circular vertex link are proved again.
func NewAffineTriangleFanShell(ctx context.Context, mates [3]*geom.SurfaceAffineTriangleMate, o AffineTriangleShellOptions) (*AffineTriangleFanShell, error) {
	return buildAffineTriangleFan(ctx, mates[:], nil, o)
}
func buildAffineTriangleFan(ctx context.Context, mates []*geom.SurfaceAffineTriangleMate, opposite []*geom.SurfaceAffineTriangleVertexContact, o AffineTriangleShellOptions) (*AffineTriangleFanShell, error) {
	n := len(mates)
	if n != 3 && n != 4 || n == 3 && len(opposite) != 0 || n == 4 && len(opposite) != 2 {
		return nil, geom.ErrInvalid
	}

	checks := make([]AffineTriangleVertexPair, len(opposite))
	for i, p := range opposite {
		checks[i] = AffineTriangleVertexPair{Faces: [2]int{i, i + 2}, Proof: p}
	}
	return buildAffineTriangleFanChecked(ctx, mates, checks, o)
}
func buildAffineTriangleFanChecked(ctx context.Context, mates []*geom.SurfaceAffineTriangleMate, checks []AffineTriangleVertexPair, o AffineTriangleShellOptions) (*AffineTriangleFanShell, error) {
	n := len(mates)
	if n < 3 || n > 8 || len(checks) != n*(n-3)/2 {
		return nil, geom.ErrInvalid
	}
	seenPairs := map[[2]int]bool{}
	for _, check := range checks {
		a, b := check.Faces[0], check.Faces[1]
		if a < 0 || b >= n || a >= b || b == a+1 || a == 0 && b == n-1 || check.Proof == nil || seenPairs[check.Faces] {
			return nil, geom.ErrInvalid
		}
		seenPairs[check.Faces] = true
	}

	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
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
	patches := make([]*geom.SurfacePatch, n)
	triangles := make([][3][2]*big.Rat, n)
	for i, p := range mates {
		if p == nil {
			return nil, geom.ErrInvalid
		}
		patches[i], triangles[i], _ = p.Source(0)
	}
	for i, p := range patches {
		for j := 0; j < i; j++ {
			if p == patches[j] {
				return nil, geom.ErrDomain
			}
		}
	}
	maps := make([][3]int, n)
	for i, p := range mates {
		source, t, _ := p.Source(1)
		next := (i + 1) % n
		if source != patches[next] {
			return nil, geom.ErrDomain
		}
		seen := [3]bool{}
		for a, v := range t {
			found := -1
			for b, w := range triangles[next] {
				if v[0].Cmp(w[0]) == 0 && v[1].Cmp(w[1]) == 0 {
					found = b
					break
				}
			}
			if found < 0 || seen[found] {
				return nil, geom.ErrDomain
			}
			maps[i][a] = found
			seen[found] = true
		}
	}
	// Bind every nonadjacent certificate to the exact full original triangles,
	// not merely to coincident numerical central points.
	oppositeVertices := make([][2]int, len(checks))
	for i, check := range checks {
		p := check.Proof
		if p == nil {
			return nil, geom.ErrInvalid
		}
		for owner := 0; owner < 2; owner++ {
			face := check.Faces[owner]
			source, t, e := p.Source(owner)
			if e != nil || source != patches[face] {
				return nil, geom.ErrDomain
			}
			seen := [3]bool{}
			mapping := [3]int{}
			for a, v := range t {
				found := -1
				for b, w := range triangles[face] {
					if v[0].Cmp(w[0]) == 0 && v[1].Cmp(w[1]) == 0 {
						found = b
						break
					}
				}
				if found < 0 || seen[found] {
					return nil, geom.ErrDomain
				}
				mapping[a] = found
				seen[found] = true
			}
			oppositeVertices[i][owner] = mapping[p.Vertices()[owner]]
		}
	}
	type ref struct{ face, edge int }
	declared := map[ref]ref{}
	for i := range mates {
		next := (i + 1) % n
		a, b := maps[i][0], maps[i][1]
		edge := -1
		for j := 0; j < 3; j++ {
			if j == a && (j+1)%3 == b || j == b && (j+1)%3 == a {
				edge = j
			}
		}
		x, y := ref{i, 0}, ref{next, edge}
		if edge < 0 {
			return nil, geom.ErrDomain
		}
		if _, ok := declared[x]; ok {
			return nil, geom.ErrDomain
		}
		if _, ok := declared[y]; ok {
			return nil, geom.ErrDomain
		}
		declared[x] = y
		declared[y] = x
	}
	out := &AffineTriangleFanShell{pairs: make([]*AffineTriangleMateShell, n), graph: &WrappedAlgebraicFaceChainShell{}, center: -1}
	for i, p := range mates {
		if out.operations >= o.MaxWork {
			return nil, geom.ErrConvergence
		}
		opts := o
		opts.MaxWork -= out.operations
		pair, e := NewAffineTriangleMateShell(ctx, p, opts)
		if e != nil {
			return nil, e
		}
		out.operations += pair.operations
		out.pairs[i] = pair
	}
	parent := make([]int, 3*n)
	for i := range parent {
		parent[i] = i
	}
	root := func(i int) int {
		for parent[i] != i {
			i = parent[i]
		}
		return i
	}
	for i := range mates {
		for side := 0; side < 2; side++ {
			a, b := root(3*i+side), root(3*((i+1)%n)+maps[i][side])
			parent[b] = a
		}
	}
	g := out.graph
	ids := map[int]int{}
	vm := make([][3]int, n)
	for fi, pair := range out.pairs {
		for vi := 0; vi < 3; vi++ {
			r := root(3*fi + vi)
			id, ok := ids[r]
			v := pair.graph.vertices[vi]
			if !ok {
				id = len(g.vertices)
				ids[r] = id
				g.vertices = append(g.vertices, v)
				g.vertexSources = append(g.vertexSources, nil)
			} else if g.vertices[id] != v {
				return nil, geom.ErrDomain
			}
			vm[fi][vi] = id
			g.vertexSources[id] = append(g.vertexSources[id], WrappedOpenVertexSource{Face: fi, Vertex: vi})
		}
	}
	if len(g.vertices) != n+1 {
		return nil, geom.ErrDomain
	}
	for i, v := range g.vertexSources {
		if len(v) == n {
			if out.center >= 0 {
				return nil, geom.ErrDomain
			}
			out.center = i
		}
	}
	if out.center < 0 {
		return nil, geom.ErrDomain
	}
	for i, check := range checks {
		for owner := 0; owner < 2; owner++ {
			if vm[check.Faces[owner]][oppositeVertices[i][owner]] != out.center {
				return nil, geom.ErrDomain
			}
		}
	}
	edgeIDs := map[ref]int{}
	for fi, pair := range out.pairs {
		var em [3]int
		var reverse [3]bool
		for local := 0; local < 3; local++ {
			r := ref{fi, local}
			mate, matched := declared[r]
			id, seen := edgeIDs[mate]
			if matched && seen {
				e := g.edges[id]
				a, b := vm[fi][local], vm[fi][(local+1)%3]
				if e.Start == a && e.End == b {
				} else if e.Start == b && e.End == a {
					reverse[local] = true
				} else {
					return nil, geom.ErrDomain
				}
				em[local] = id
				g.edgeSources[id] = append(g.edgeSources[id], WrappedOpenEdgeSource{Face: fi, Edge: local, Loop: 0, LocalEdge: local, Reversed: reverse[local]})
			} else {
				id = len(g.edges)
				e := pair.graph.edges[local]
				e.Start = vm[fi][local]
				e.End = vm[fi][(local+1)%3]
				g.edges = append(g.edges, e)
				em[local] = id
				g.edgeSources = append(g.edgeSources, []WrappedOpenEdgeSource{{Face: fi, Edge: local, Loop: 0, LocalEdge: local}})
			}
			edgeIDs[r] = em[local]
		}
		f := cloneFace(pair.graph.faces[0])
		for i, u := range f.Outer {
			local := u.Edge
			if local < 0 || local > 2 {
				return nil, geom.ErrDomain
			}
			u.Edge = em[local]
			u.Reversed = u.Reversed != reverse[local]
			f.Outer[i] = u
		}
		g.faces = append(g.faces, f)
	}
	if len(g.edges) != 2*n {
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
	boundary, e := validateWrappedFanIncidence(ctx, g, out.center)
	if e != nil {
		return nil, e
	}
	g.boundary = boundary
	return out, nil
}
