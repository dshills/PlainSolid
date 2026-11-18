//go:build research

package trimmed

import (
	"context"
	"math/big"

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
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	p, e := CertifyWrappedAlgebraicFaceChainEmbedding(ctx, []*WrappedAlgebraicOpenShell{ab, bc}, o)
	if e != nil {
		return nil, e
	}
	out := &WrappedAlgebraicChainEmbedding{pairs: [2]*WrappedAlgebraicOpenShell{ab, bc}, boundary: p.boundary, bounds: p.bounds, axes: p.axes, operations: p.operations}
	copy(out.shifts[:], p.shifts)
	return out, nil
}
