package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedChainBoundaryEdge identifies an original authored edge. UV is its
// exact positive exterior occurrence in face zero's lifted chart. The source
// face and its original UV remain available through PairShell.
type WrappedChainBoundaryEdge struct {
	Face, LocalEdge int
	UV              ExactQuadraticUV
}

// WrappedAlgebraicChainEmbedding proves three original disks form an embedded
// open disk, joined only along the two declared entire edges. It is a global
// separation certificate, not a closed solid or an indexed three-face shell.
// The two existing pair shells retain all native geometry and mate ownership.
type WrappedAlgebraicChainEmbedding struct {
	pairs      [2]*WrappedAlgebraicOpenShell
	shifts     [3][2]*big.Int
	boundary   []WrappedChainBoundaryEdge
	bounds     geom.ParameterRectangle
	axes       [2]int
	operations int
}

func (p *WrappedAlgebraicChainEmbedding) PairShell(i int) (*WrappedAlgebraicOpenShell, error) {
	if p == nil || i < 0 || i > 1 {
		return nil, geom.ErrInvalid
	}
	return p.pairs[i], nil
}
func (p *WrappedAlgebraicChainEmbedding) PeriodShift(face int) ([2]*big.Int, error) {
	var out [2]*big.Int
	if p == nil || face < 0 || face > 2 {
		return out, geom.ErrInvalid
	}
	for i := range out {
		out[i] = new(big.Int).Set(p.shifts[face][i])
	}
	return out, nil
}
func (p *WrappedAlgebraicChainEmbedding) Boundary() []WrappedChainBoundaryEdge {
	if p == nil {
		return nil
	}
	return append([]WrappedChainBoundaryEdge(nil), p.boundary...)
}
func (p *WrappedAlgebraicChainEmbedding) LiftedBounds() geom.ParameterRectangle {
	if p == nil {
		return geom.ParameterRectangle{}
	}
	return p.bounds
}
func (p *WrappedAlgebraicChainEmbedding) ProjectionAxes() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.axes
}
func (p *WrappedAlgebraicChainEmbedding) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}

// CertifyWrappedAlgebraicChainEmbedding accepts ordered pair shells (A,B) and
// (B,C), with the same immutable original B object. It requires two disjoint
// middle edges (no triple vertex), the same exact support chart map, whole
// period lift correspondence, and one common physical orientation. At most 96
// source edges are considered. MaxOperations is cumulative over both pair
// proofs, exact lift transport, exterior validation and global injectivity.
//
// Cancelling the two complete reversed UV mates must leave one simple positive
// loop. Additivity of winding numbers then proves all three disk interiors
// disjoint. The original simple loops and disjoint middle mates exclude any
// contact hidden on a cancelled edge. A single projected-injective hull proves
// the same statements in model space. Other adjacency graphs remain unsupported.
func CertifyWrappedAlgebraicChainEmbedding(ctx context.Context, ab, bc *WrappedAlgebraicOpenShell, o geom.SurfaceInjectivityOptions) (*WrappedAlgebraicChainEmbedding, error) {
	if ctx == nil || ab == nil || bc == nil || ab.sources[1] == nil || ab.sources[1] != bc.sources[0] {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
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
	var pairProof [2]*WrappedAlgebraicShellEmbedding
	for i, s := range []*WrappedAlgebraicOpenShell{ab, bc} {
		opts := o
		opts.MaxOperations = remaining()
		p, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, s, opts)
		if e != nil {
			return nil, e
		}
		pairProof[i] = p
		used += p.ExactOperations()
	}
	middleLeft, _ := ab.pair.Owner(1)
	middleRight, _ := bc.pair.Owner(0)
	for _, a := range middleLeft.Vertices {
		for _, b := range middleRight.Vertices {
			if a == b {
				return nil, geom.ErrConvergence
			}
		}
	}
	out := &WrappedAlgebraicChainEmbedding{pairs: [2]*WrappedAlgebraicOpenShell{ab, bc}, bounds: geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}}
	for axis := 0; axis < 2; axis++ {
		out.shifts[0][axis] = new(big.Int)
		out.shifts[1][axis] = new(big.Int).Set(pairProof[0].shifts[axis])
		out.shifts[2][axis] = new(big.Int).Add(out.shifts[1][axis], pairProof[1].shifts[axis])
	}
	source := [3]*WrappedAlgebraicQuadraticFace{ab.sources[0], ab.sources[1], bc.sources[1]}
	w := source[0].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch)
	periods := w.Periods()
	first, _ := ab.pair.Owner(0)
	last, _ := bc.pair.Owner(1)
	skip := [3]map[int]bool{{first.Edge: true}, {middleLeft.Edge: true, middleRight.Edge: true}, {last.Edge: true}}
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
	cert, report, e := trimvalidate.CertifyLoops(ctx, loop, nil, trimvalidate.Options{MaxEdges: 96, MaxWork: remaining(), MaxBits: o.MaxCoefficientBits})
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
