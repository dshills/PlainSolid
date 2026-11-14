package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedAlgebraicFaceChainEmbedding proves two to eight original disks form an embedded
// open disk, joined only along the declared entire edges. It is a global
// separation certificate, not a closed solid or an indexed multi-face shell.
// The existing pair shells retain all native geometry and mate ownership.
type WrappedAlgebraicFaceChainEmbedding struct {
	pairs      []*WrappedAlgebraicOpenShell
	shifts     [][2]*big.Int
	boundary   []WrappedChainBoundaryEdge
	bounds     geom.ParameterRectangle
	axes       [2]int
	operations int
}

func (p *WrappedAlgebraicFaceChainEmbedding) PairShell(i int) (*WrappedAlgebraicOpenShell, error) {
	if p == nil || i < 0 || i >= len(p.pairs) {
		return nil, geom.ErrInvalid
	}
	return p.pairs[i], nil
}
func (p *WrappedAlgebraicFaceChainEmbedding) PeriodShift(face int) ([2]*big.Int, error) {
	var out [2]*big.Int
	if p == nil || face < 0 || face >= len(p.shifts) {
		return out, geom.ErrInvalid
	}
	for i := range out {
		out[i] = new(big.Int).Set(p.shifts[face][i])
	}
	return out, nil
}
func (p *WrappedAlgebraicFaceChainEmbedding) Boundary() []WrappedChainBoundaryEdge {
	if p == nil {
		return nil
	}
	return append([]WrappedChainBoundaryEdge(nil), p.boundary...)
}
func (p *WrappedAlgebraicFaceChainEmbedding) LiftedBounds() geom.ParameterRectangle {
	if p == nil {
		return geom.ParameterRectangle{}
	}
	return p.bounds
}
func (p *WrappedAlgebraicFaceChainEmbedding) ProjectionAxes() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.axes
}
func (p *WrappedAlgebraicFaceChainEmbedding) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}

// FaceCount is the number of original immutable source disks.
func (p *WrappedAlgebraicFaceChainEmbedding) FaceCount() int {
	if p == nil {
		return 0
	}
	return len(p.shifts)
}

// CertifyWrappedAlgebraicFaceChainEmbedding proves a connected simple chain of
// two to eight original disks. Consecutive pairs must share the identical
// immutable middle source. Each interior face has two disjoint mate endpoint
// sets, so every vertex link is an interval with at most two face sectors.
// Exact period lifts propagate along the declared chain; no other adjacency
// or contact is inferred. Reused original faces reject.
//
// All cancelled mates have complete reversed UV identity. Their removal must
// leave one simple positive exterior. Winding additivity proves all interiors
// disjoint, and one globally injective lifted hull transfers the proof to model
// space. Up to 256 source edges and cumulative arithmetic work are bounded.
func CertifyWrappedAlgebraicFaceChainEmbedding(ctx context.Context, pairs []*WrappedAlgebraicOpenShell, o geom.SurfaceInjectivityOptions) (*WrappedAlgebraicFaceChainEmbedding, error) {
	if ctx == nil || len(pairs) < 1 || len(pairs) > 7 {
		return nil, geom.ErrInvalid
	}
	source := make([]*WrappedAlgebraicQuadraticFace, len(pairs)+1)
	seen := map[*WrappedAlgebraicQuadraticFace]bool{}
	for i, pair := range pairs {
		if pair == nil || pair.sources[0] == nil || pair.sources[1] == nil {
			return nil, geom.ErrInvalid
		}
		if i == 0 {
			source[0] = pair.sources[0]
			seen[source[0]] = true
		} else if pair.sources[0] != source[i] {
			return nil, geom.ErrInvalid
		}
		source[i+1] = pair.sources[1]
		if seen[source[i+1]] {
			return nil, geom.ErrDomain
		}
		seen[source[i+1]] = true
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
	used := 0
	remaining := func() int {
		n := o.MaxOperations - used
		if n <= 0 {
			return -1
		}
		return n
	}
	check := func(q *big.Rat) error {
		used++
		if q == nil || used > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	pairProof := make([]*WrappedAlgebraicShellEmbedding, len(pairs))
	for i, s := range pairs {
		opts := o
		opts.MaxOperations = remaining()
		p, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, s, opts)
		if e != nil {
			return nil, e
		}
		pairProof[i] = p
		used += p.ExactOperations()
	}
	for i := 1; i < len(pairs); i++ {
		left, _ := pairs[i-1].pair.Owner(1)
		right, _ := pairs[i].pair.Owner(0)
		for _, a := range left.Vertices {
			for _, b := range right.Vertices {
				if a == b {
					return nil, geom.ErrConvergence
				}
			}
		}
	}
	out := &WrappedAlgebraicFaceChainEmbedding{pairs: append([]*WrappedAlgebraicOpenShell(nil), pairs...), shifts: make([][2]*big.Int, len(source)), bounds: geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}}
	for axis := 0; axis < 2; axis++ {
		out.shifts[0][axis] = new(big.Int)
		for i, p := range pairProof {
			out.shifts[i+1][axis] = new(big.Int).Add(out.shifts[i][axis], p.shifts[axis])
		}
	}
	w := source[0].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch)
	periods := w.Periods()
	skip := make([]map[int]bool, len(source))
	for i := range skip {
		skip[i] = map[int]bool{}
	}
	for i, pair := range pairs {
		a, _ := pair.pair.Owner(0)
		b, _ := pair.pair.Owner(1)
		skip[i][a.Edge] = true
		skip[i+1][b.Edge] = true
	}
	var edges []WrappedChainBoundaryEdge
	for fi, f := range source {
		var delta [2]*big.Rat
		for axis := range delta {
			delta[axis] = new(big.Rat)
			if periods[axis] != nil {
				delta[axis].Mul(new(big.Rat).SetInt(out.shifts[fi][axis]), periods[axis])
			}
			if e := check(delta[axis]); e != nil {
				return nil, e
			}
		}
		for ei, q := range f.face.loops[0] {
			h := q.HomogeneousControls()
			for i := range h {
				for _, v := range h[i] {
					if e := check(v); e != nil {
						return nil, e
					}
				}
				for axis := 0; axis < 2; axis++ {
					h[i][axis].Sub(h[i][axis], new(big.Rat).Mul(delta[axis], h[i][2]))
					if e := check(h[i][axis]); e != nil {
						return nil, e
					}
					v := new(big.Rat).Quo(h[i][axis], h[i][2])
					if e := check(v); e != nil {
						return nil, e
					}
					x, e := geom.RepresentedExactScalar(v)
					if e != nil {
						return nil, e
					}
					if axis == 0 {
						out.bounds.UMin = math.Min(out.bounds.UMin, x)
						out.bounds.UMax = math.Max(out.bounds.UMax, x)
					} else {
						out.bounds.VMin = math.Min(out.bounds.VMin, x)
						out.bounds.VMax = math.Max(out.bounds.VMax, x)
					}
				}
			}
			aligned, e := NewExactQuadraticUV(h)
			if e != nil {
				return nil, e
			}
			if !skip[fi][ei] {
				edges = append(edges, WrappedChainBoundaryEdge{fi, ei, aligned})
			}
		}
	}
	// Unique exact endpoints exclude pinched fans and undeclared vertex contacts.
	// No model-coordinate proximity or rational enclosure is used for incidence.
	key := func(p [2]*big.Rat) string { return p[0].RatString() + "," + p[1].RatString() }
	starts := map[string]int{}
	ends := map[string]bool{}
	for i, edge := range edges {
		uv, _ := edge.UV.Endpoints()
		a, b := key(uv[0]), key(uv[1])
		if _, ok := starts[a]; ok {
			return nil, geom.ErrDomain
		}
		if ends[b] || a == b {
			return nil, geom.ErrDomain
		}
		starts[a] = i
		ends[b] = true
	}
	if len(edges) < 3 {
		return nil, geom.ErrConvergence
	}
	firstUV, _ := edges[0].UV.Endpoints()
	start := key(firstUV[0])
	at := start
	var loop trimvalidate.Loop
	for range edges {
		i, ok := starts[at]
		if !ok {
			return nil, geom.ErrDomain
		}
		delete(starts, at)
		edge := edges[i]
		out.boundary = append(out.boundary, edge)
		q, e := trimvalidate.ExactQuadratic(edge.UV.HomogeneousControls())
		if e != nil {
			return nil, e
		}
		loop = append(loop, q)
		uv, _ := edge.UV.Endpoints()
		at = key(uv[1])
	}
	if at != start || len(starts) != 0 {
		return nil, geom.ErrDomain
	}
	cert, report, e := trimvalidate.CertifyLoops(ctx, loop, nil, trimvalidate.Options{MaxEdges: 256, MaxWork: remaining(), MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	used += report.Work
	if report.Status == trimvalidate.Invalid {
		return nil, geom.ErrDomain
	}
	if cert == nil || report.Status != trimvalidate.Certified {
		return nil, geom.ErrConvergence
	}
	opts := o
	opts.MaxOperations = remaining()
	embedding, e := w.CertifyLiftedRectangleInjectivity(ctx, out.bounds, opts)
	if e != nil {
		return nil, e
	}
	used += embedding.ExactOperations
	if !embedding.ProvenInjective {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.axes = embedding.ProjectionAxes
	out.operations = used
	return out, nil
}
