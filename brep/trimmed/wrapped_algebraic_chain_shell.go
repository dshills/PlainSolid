package trimmed

import (
	"context"
	"math/big"

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
	if p == nil {
		return nil, geom.ErrInvalid
	}
	if o.MaxWork == 0 {
		o.MaxWork = 16000000
	}
	general := &WrappedAlgebraicFaceChainEmbedding{pairs: append([]*WrappedAlgebraicOpenShell(nil), p.pairs[:]...), shifts: append([][2]*big.Int(nil), p.shifts[:]...), boundary: p.boundary, bounds: p.bounds, axes: p.axes, operations: p.operations}
	s, e := NewWrappedAlgebraicFaceChainShell(ctx, general, o)
	if e != nil {
		return nil, e
	}
	return &WrappedAlgebraicChainShell{embedding: p, vertices: s.vertices, edges: s.edges, faces: s.faces, vertexSources: s.vertexSources, edgeSources: s.edgeSources, boundary: s.boundary, operations: s.operations}, nil
}
func validateWrappedChainIncidence(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, vs [][]WrappedOpenVertexSource, es [][]WrappedOpenEdgeSource) ([]WrappedOpenBoundaryUse, error) {
	if len(faces) != 3 {
		return nil, geom.ErrDomain
	}
	return validateWrappedFaceChainIncidence(ctx, vertices, edges, faces, vs, es)
}
