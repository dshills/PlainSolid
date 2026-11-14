package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAlgebraicSeparatedChain is an embedded indexed open disk with three
// different original wrapped supports. Independent adjacent mate/separator
// proofs and a strictly positive gap between their parallel world planes
// separate the nonadjacent faces. No common parameter chart is asserted.
type WrappedAlgebraicSeparatedChain struct {
	adjacent   [2]*WrappedAlgebraicSeparatedPair
	graph      *WrappedAlgebraicFaceChainShell
	gap        *big.Rat
	operations int
}

func (s *WrappedAlgebraicSeparatedChain) Adjacent(i int) (*WrappedAlgebraicSeparatedPair, error) {
	if s == nil || i < 0 || i > 1 {
		return nil, geom.ErrInvalid
	}
	return s.adjacent[i], nil
}

// SeparationGap is the exact positive world-coordinate gap separating faces0,2.
func (s *WrappedAlgebraicSeparatedChain) SeparationGap() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.gap)
}
func (s *WrappedAlgebraicSeparatedChain) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.graph.Vertices()
}
func (s *WrappedAlgebraicSeparatedChain) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.graph.Edges()
}
func (s *WrappedAlgebraicSeparatedChain) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.graph.Faces()
}
func (s *WrappedAlgebraicSeparatedChain) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.graph.Boundary()
}
func (s *WrappedAlgebraicSeparatedChain) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.VertexSources(i)
}
func (s *WrappedAlgebraicSeparatedChain) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.EdgeSources(i)
}
func (s *WrappedAlgebraicSeparatedChain) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// NewWrappedAlgebraicSeparatedChain accepts ordered pairs(A,B),(B,C) sharing
// the identical immutable B source. Each plane contains exactly its declared
// mate; face interiors lie strictly on opposite sides. Both planes must select
// the same world axis and their ordered levels must leave a strict positive
// gap between A and C. Coincident planes, unproved nonadjacent contact, and
// disjoint/unbound source declarations reject atomically. The original chart
// scopes, signed scales, physical orientations and exact root witnesses remain
// on the pair objects; only declared indexed mates are merged.
// MaxOperations covers both full separator proofs and every exported edge use.
func NewWrappedAlgebraicSeparatedChain(ctx context.Context, ab, bc *WrappedAlgebraicOpenShell, separators [2]WrappedCoordinateSeparator, o geom.SurfaceAffineCoordinateOptions) (*WrappedAlgebraicSeparatedChain, error) {
	result, e := NewWrappedAlgebraicSeparatedSeries(ctx, []*WrappedAlgebraicOpenShell{ab, bc}, separators[:], o)
	if e != nil {
		return nil, e
	}
	gap, e := result.NonadjacentGap(0, 2)
	if e != nil {
		return nil, e
	}
	return &WrappedAlgebraicSeparatedChain{adjacent: [2]*WrappedAlgebraicSeparatedPair{result.adjacent[0], result.adjacent[1]}, graph: result.graph, gap: gap, operations: result.operations}, nil
}
