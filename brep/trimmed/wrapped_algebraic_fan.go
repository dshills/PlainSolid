package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAlgebraicFanEmbedding certifies three or four original disks around
// one exact represented interior vertex. Every radial mate is declared; source
// charts are equal up to whole-period lifts with zero cyclic holonomy. A simple
// exterior and global projected injectivity prove an embedded open disk.
type WrappedAlgebraicFanEmbedding struct {
	proof   *WrappedAlgebraicFaceChainEmbedding
	centers []int
}

func (p *WrappedAlgebraicFanEmbedding) FaceCount() int {
	if p == nil {
		return 0
	}
	return len(p.centers)
}
func (p *WrappedAlgebraicFanEmbedding) PairShell(i int) (*WrappedAlgebraicOpenShell, error) {
	if p == nil {
		return nil, geom.ErrInvalid
	}
	return p.proof.PairShell(i)
}
func (p *WrappedAlgebraicFanEmbedding) PeriodShift(i int) ([2]*big.Int, error) {
	if p == nil {
		return [2]*big.Int{}, geom.ErrInvalid
	}
	return p.proof.PeriodShift(i)
}
func (p *WrappedAlgebraicFanEmbedding) CenterVertex(i int) (int, error) {
	if p == nil || i < 0 || i >= len(p.centers) {
		return 0, geom.ErrInvalid
	}
	return p.centers[i], nil
}
func (p *WrappedAlgebraicFanEmbedding) Boundary() []WrappedChainBoundaryEdge {
	if p == nil {
		return nil
	}
	return p.proof.Boundary()
}
func (p *WrappedAlgebraicFanEmbedding) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.proof.ExactOperations()
}

func (p *WrappedAlgebraicFanEmbedding) LiftedBounds() geom.ParameterRectangle {
	if p == nil {
		return geom.ParameterRectangle{}
	}
	return p.proof.LiftedBounds()
}
func (p *WrappedAlgebraicFanEmbedding) ProjectionAxes() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.proof.ProjectionAxes()
}

// CertifyWrappedAlgebraicFanEmbedding accepts cyclic pairs (F0,F1), ...,
// (Fn-1,F0). Original face objects must match at every adjacency. Each face's
// two distinct mates meet only at its center vertex. All centers must be the
// same exact model point. Each pair's other endpoint is a distinct rim vertex.
// Limits are cumulative over pair, lift, exterior and injectivity proofs.
func CertifyWrappedAlgebraicFanEmbedding(ctx context.Context, pairs []*WrappedAlgebraicOpenShell, o geom.SurfaceInjectivityOptions) (*WrappedAlgebraicFanEmbedding, error) {
	n := len(pairs)
	if ctx == nil || (n != 3 && n != 4) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, geom.ErrInvalid
	}
	source := make([]*WrappedAlgebraicQuadraticFace, n)
	seen := map[*WrappedAlgebraicQuadraticFace]bool{}
	for i, pair := range pairs {
		if pair == nil || pair.sources[0] == nil || pair.sources[1] == nil {
			return nil, geom.ErrInvalid
		}
		source[i] = pair.sources[0]
		if seen[source[i]] {
			return nil, geom.ErrDomain
		}
		seen[source[i]] = true
	}
	for i, pair := range pairs {
		if pair.sources[1] != source[(i+1)%n] {
			return nil, geom.ErrInvalid
		}
	}
	centers := make([]int, n)
	skip := make([]map[int]bool, n)
	for i := range source {
		incoming, _ := pairs[(i+n-1)%n].pair.Owner(1)
		outgoing, _ := pairs[i].pair.Owner(0)
		if incoming.Edge == outgoing.Edge {
			return nil, geom.ErrDomain
		}
		common := -1
		for _, a := range incoming.Vertices {
			for _, b := range outgoing.Vertices {
				if a == b {
					if common >= 0 {
						return nil, geom.ErrDomain
					}
					common = a
				}
			}
		}
		if common < 0 {
			return nil, geom.ErrDomain
		}
		centers[i] = common
		skip[i] = map[int]bool{incoming.Edge: true, outgoing.Edge: true}
	}
	center := source[0].NativeFace().Vertices[centers[0]]
	rim := map[geom.Vec3]bool{}
	for i, pair := range pairs {
		if source[i].NativeFace().Vertices[centers[i]] != center {
			return nil, geom.ErrDomain
		}
		found := false
		for side := 0; side < 2; side++ {
			_, v, e := pair.pair.Endpoint(side)
			if e != nil {
				return nil, e
			}
			isCenter := v[0] == centers[i]
			if isCenter != (v[1] == centers[(i+1)%n]) {
				return nil, geom.ErrDomain
			}
			if !isCenter {
				x := source[i].NativeFace().Vertices[v[0]]
				if x == center || rim[x] {
					return nil, geom.ErrDomain
				}
				rim[x] = true
				found = true
			}
		}
		if !found {
			return nil, geom.ErrDomain
		}
	}
	proof := &WrappedAlgebraicFaceChainEmbedding{pairs: append([]*WrappedAlgebraicOpenShell(nil), pairs...), shifts: make([][2]*big.Int, n), bounds: geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}}
	total := [2]*big.Int{new(big.Int), new(big.Int)}
	used := 0
	for i, pair := range pairs {
		for a := 0; a < 2; a++ {
			proof.shifts[i][a] = new(big.Int).Set(total[a])
		}
		if used >= o.MaxOperations {
			return nil, geom.ErrConvergence
		}
		opts := o
		opts.MaxOperations -= used
		p, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, pair, opts)
		if e != nil {
			return nil, e
		}
		used += p.ExactOperations()
		for a := 0; a < 2; a++ {
			total[a].Add(total[a], p.shifts[a])
			if total[a].BitLen() > o.MaxCoefficientBits {
				return nil, geom.ErrConvergence
			}
		}
	}
	if total[0].Sign() != 0 || total[1].Sign() != 0 {
		return nil, geom.ErrDomain
	}
	proof, e := certifyWrappedFaceExterior(ctx, source, skip, o, used, proof)
	if e != nil {
		return nil, e
	}
	return &WrappedAlgebraicFanEmbedding{proof: proof, centers: centers}, nil
}

// WrappedAlgebraicFanShell is a native indexed open disk with one circular
// interior vertex link and interval boundary links, retaining original charts.
// It does not assert any closed material volume.
type WrappedAlgebraicFanShell struct {
	embedding *WrappedAlgebraicFanEmbedding
	graph     *WrappedAlgebraicFaceChainShell
	center    int
}

func (s *WrappedAlgebraicFanShell) Embedding() *WrappedAlgebraicFanEmbedding {
	if s == nil {
		return nil
	}
	return s.embedding
}
func (s *WrappedAlgebraicFanShell) CenterVertex() int {
	if s == nil {
		return -1
	}
	return s.center
}
func (s *WrappedAlgebraicFanShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.graph.Vertices()
}
func (s *WrappedAlgebraicFanShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.graph.Edges()
}
func (s *WrappedAlgebraicFanShell) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.graph.Faces()
}
func (s *WrappedAlgebraicFanShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.graph.Boundary()
}
func (s *WrappedAlgebraicFanShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.VertexSources(i)
}
func (s *WrappedAlgebraicFanShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.EdgeSources(i)
}
func (s *WrappedAlgebraicFanShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.graph.ExactOperations()
}

// NewWrappedAlgebraicFanShell merges only the cyclic mate identities from the
// immutable embedding, then re-proves all actual exported edge uses. MaxWork
// covers this construction, independently of the earlier embedding budget.
func NewWrappedAlgebraicFanShell(ctx context.Context, p *WrappedAlgebraicFanEmbedding, o edgecheck.Options) (*WrappedAlgebraicFanShell, error) {
	if ctx == nil || p == nil || p.proof == nil || (p.FaceCount() != 3 && p.FaceCount() != 4) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxWork == 0 {
		o.MaxWork = 64000000
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
	n := p.FaceCount()
	source := make([]SewFace, n)
	offset := make([]int, n)
	total := 0
	for i, pair := range p.proof.pairs {
		if e := wrappedShellSourceBinding(ctx, pair); e != nil {
			return nil, e
		}
		source[i] = pair.sources[0].NativeFace()
		offset[i] = total
		total += len(source[i].Vertices)
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
	for i, pair := range p.proof.pairs {
		for side := 0; side < 2; side++ {
			_, v, e := pair.pair.Endpoint(side)
			if e != nil {
				return nil, e
			}
			a, b := root(offset[i]+v[0]), root(offset[(i+1)%n]+v[1])
			if a != b {
				parent[b] = a
			}
		}
	}
	graph := &WrappedAlgebraicFaceChainShell{}
	vm := make([][]int, n)
	ids := map[int]int{}
	for fi, f := range source {
		vm[fi] = make([]int, len(f.Vertices))
		for vi, v := range f.Vertices {
			r := root(offset[fi] + vi)
			id, ok := ids[r]
			if !ok {
				id = len(graph.vertices)
				ids[r] = id
				graph.vertices = append(graph.vertices, v)
				graph.vertexSources = append(graph.vertexSources, nil)
			} else if graph.vertices[id] != v {
				return nil, geom.ErrDomain
			}
			vm[fi][vi] = id
			graph.vertexSources[id] = append(graph.vertexSources[id], WrappedOpenVertexSource{fi, vi})
		}
	}
	// Every radial edge has exactly two declarations. Pick the lower face index
	// as its model owner, including the closing pair (last,first).
	type ref struct{ face, edge int }
	mates := map[ref]ref{}
	for i, pair := range p.proof.pairs {
		a, _ := pair.pair.Owner(0)
		b, _ := pair.pair.Owner(1)
		x, y := ref{i, a.NativeEdge}, ref{(i + 1) % n, b.NativeEdge}
		if _, ok := mates[x]; ok {
			return nil, geom.ErrDomain
		}
		if _, ok := mates[y]; ok {
			return nil, geom.ErrDomain
		}
		mates[x] = y
		mates[y] = x
	}
	em := make([][]int, n)
	reversed := make([][]bool, n)
	for fi, f := range source {
		em[fi] = make([]int, len(f.Edges))
		reversed[fi] = make([]bool, len(f.Edges))
		for ei, edge := range f.Edges {
			id := -1
			other, shared := mates[ref{fi, ei}]
			if shared && other.face < fi {
				id = em[other.face][other.edge]
				reversed[fi][ei] = true
			}
			if id < 0 {
				id = len(graph.edges)
				edge.Start = vm[fi][edge.Start]
				edge.End = vm[fi][edge.End]
				graph.edges = append(graph.edges, edge)
				graph.edgeSources = append(graph.edgeSources, nil)
			}
			em[fi][ei] = id
			graph.edgeSources[id] = append(graph.edgeSources[id], WrappedOpenEdgeSource{Face: fi, Edge: ei, Loop: 0, LocalEdge: ei, Reversed: reversed[fi][ei]})
		}
		face := cloneFace(f.Face)
		for i := range face.Outer {
			u := &face.Outer[i]
			old := u.Edge
			u.Edge = em[fi][old]
			u.Reversed = u.Reversed != reversed[fi][old]
		}
		graph.faces = append(graph.faces, face)
	}
	for _, f := range graph.faces {
		for _, u := range f.Outer {
			if graph.operations >= o.MaxWork {
				return nil, geom.ErrConvergence
			}
			opts := o
			opts.MaxWork -= graph.operations
			same, work, e := compareWrappedQuadraticUse(ctx, graph.edges[u.Edge], f.Surface, u, opts)
			graph.operations += work
			if e != nil {
				return nil, e
			}
			if !same {
				return nil, geom.ErrConvergence
			}
		}
	}
	center := vm[0][p.centers[0]]
	boundary, e := validateWrappedFanIncidence(ctx, graph, center)
	if e != nil {
		return nil, e
	}
	if len(boundary) != len(p.proof.boundary) {
		return nil, geom.ErrDomain
	}
	for i, u := range boundary {
		at := i
		if graph.faces[0].Reversed {
			at = len(boundary) - 1 - i
		}
		original := p.proof.boundary[at]
		if u.Face != original.Face || u.LocalEdge != original.LocalEdge {
			return nil, geom.ErrDomain
		}
	}
	graph.boundary = boundary
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return &WrappedAlgebraicFanShell{embedding: p, graph: graph, center: center}, nil
}

func validateWrappedFanIncidence(ctx context.Context, g *WrappedAlgebraicFaceChainShell, center int) ([]WrappedOpenBoundaryUse, error) {
	if ctx == nil || g == nil {
		return nil, geom.ErrInvalid
	}
	n := len(g.faces)
	nv := len(g.vertices)
	if (n < 3 || n > 8) || center < 0 || center >= nv || nv-len(g.edges)+n != 1 || len(g.vertexSources) != nv || len(g.edgeSources) != len(g.edges) || nv > 128 || len(g.edges) > 128 {
		return nil, geom.ErrDomain
	}
	incidence := make([][]WrappedOpenBoundaryUse, len(g.edges))
	masks := make([]int, nv)
	for fi, f := range g.faces {
		if len(f.Holes) != 0 || len(f.Outer) < 3 || len(f.Outer) > 32 {
			return nil, geom.ErrDomain
		}
		seen := map[int]bool{}
		first, previous := -1, -1
		for local, u := range f.Outer {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			if u.Edge < 0 || u.Edge >= len(g.edges) {
				return nil, geom.ErrInvalid
			}
			edge := g.edges[u.Edge]
			a, b := edge.Start, edge.End
			if u.Reversed {
				a, b = b, a
			}
			if a < 0 || b < 0 || a >= nv || b >= nv || a == b || seen[a] {
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
			incidence[u.Edge] = append(incidence[u.Edge], WrappedOpenBoundaryUse{Face: fi, LocalEdge: local, Use: u, ReversedTraversal: f.Reversed, Start: a, End: b})
		}
		if previous != first || !seen[center] {
			return nil, geom.ErrDomain
		}
	}
	links := make([][]int, n)
	radial := make([]int, nv)
	outgoing := map[int]WrappedOpenBoundaryUse{}
	incoming := make([]int, nv)
	shared := 0
	for ei, uses := range incidence {
		if len(uses) != len(g.edgeSources[ei]) {
			return nil, geom.ErrDomain
		}
		switch len(uses) {
		case 1:
			u := uses[0]
			if u.Start == center || u.End == center {
				return nil, geom.ErrDomain
			}
			if _, ok := outgoing[u.Start]; ok {
				return nil, geom.ErrDomain
			}
			outgoing[u.Start] = u
			incoming[u.End]++
		case 2:
			a, b := uses[0], uses[1]
			if a.Face == b.Face || a.Start != b.End || a.End != b.Start || (a.Start != center && a.End != center) {
				return nil, geom.ErrDomain
			}
			shared++
			links[a.Face] = append(links[a.Face], b.Face)
			links[b.Face] = append(links[b.Face], a.Face)
			rim := a.Start
			if rim == center {
				rim = a.End
			}
			radial[rim]++
		default:
			return nil, geom.ErrDomain
		}
	}
	if shared != n || len(outgoing) != nv-1 || masks[center] != (1<<n)-1 || len(g.vertexSources[center]) != n {
		return nil, geom.ErrDomain
	}
	// The actual shared-edge incidence, not Euler alone, must form one circular
	// face link around the unique interior vertex.
	reached := map[int]bool{0: true}
	queue := []int{0}
	for len(queue) > 0 {
		fi := queue[0]
		queue = queue[1:]
		if len(links[fi]) != 2 || links[fi][0] == links[fi][1] {
			return nil, geom.ErrDomain
		}
		for _, j := range links[fi] {
			if !reached[j] {
				reached[j] = true
				queue = append(queue, j)
			}
		}
	}
	if len(reached) != n {
		return nil, geom.ErrDomain
	}
	for vi, owners := range g.vertexSources {
		mask := 0
		for _, v := range owners {
			if v.Face < 0 || v.Face >= n || v.Vertex < 0 || v.Vertex >= 32 || mask&(1<<v.Face) != 0 {
				return nil, geom.ErrDomain
			}
			mask |= 1 << v.Face
		}
		if mask != masks[vi] {
			return nil, geom.ErrDomain
		}
		if vi == center {
			if incoming[vi] != 0 {
				return nil, geom.ErrDomain
			}
			continue
		}
		if incoming[vi] != 1 || radial[vi] > 1 || len(owners) != 1+radial[vi] {
			return nil, geom.ErrDomain
		}
	}
	start := -1
	for _, u := range outgoing {
		if u.Face == 0 && (start < 0 || u.LocalEdge < outgoing[start].LocalEdge) {
			start = u.Start
		}
	}
	if start < 0 {
		return nil, geom.ErrDomain
	}
	if g.faces[0].Reversed {
		start = outgoing[start].End
	}
	at := start
	var boundary []WrappedOpenBoundaryUse
	for len(boundary) < nv-1 {
		u, ok := outgoing[at]
		if !ok {
			return nil, geom.ErrDomain
		}
		delete(outgoing, at)
		boundary = append(boundary, u)
		at = u.End
	}
	if at != start || len(outgoing) != 0 {
		return nil, geom.ErrDomain
	}
	return boundary, ctx.Err()
}
