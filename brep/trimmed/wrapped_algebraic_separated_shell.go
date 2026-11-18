//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAlgebraicSeparatedShell is an embedded indexed native open disk on
// two independently parameterized, generally different wrapped support maps.
// The common model edge and original owner chart/root identities are retained
// by its immutable PairShell. This is not a closed solid or a volume claim.
type WrappedAlgebraicSeparatedShell struct {
	proof      *WrappedAlgebraicSeparatedPair
	operations int
}

func (s *WrappedAlgebraicSeparatedShell) PairShell() *WrappedAlgebraicOpenShell {
	if s == nil {
		return nil
	}
	return s.proof.Shell()
}
func (s *WrappedAlgebraicSeparatedShell) Separation() *WrappedAlgebraicSeparatedPair {
	if s == nil {
		return nil
	}
	return s.proof
}
func (s *WrappedAlgebraicSeparatedShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.PairShell().Vertices()
}
func (s *WrappedAlgebraicSeparatedShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.PairShell().Edges()
}
func (s *WrappedAlgebraicSeparatedShell) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.PairShell().Faces()
}
func (s *WrappedAlgebraicSeparatedShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.PairShell().Boundary()
}
func (s *WrappedAlgebraicSeparatedShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.PairShell().VertexSources(i)
}
func (s *WrappedAlgebraicSeparatedShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.PairShell().EdgeSources(i)
}
func (s *WrappedAlgebraicSeparatedShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// NewWrappedAlgebraicSeparatedShell re-proves global separation and every
// actual indexed model-edge association to both original owner pcurves. Work
// is cumulative across these proofs; the already constructed pair is immutable.
func NewWrappedAlgebraicSeparatedShell(ctx context.Context, pair *WrappedAlgebraicOpenShell, q WrappedCoordinateSeparator, o geom.SurfaceAffineCoordinateOptions) (*WrappedAlgebraicSeparatedShell, error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	proof, e := CertifyWrappedAlgebraicSeparatedPair(ctx, pair, q, o)
	if e != nil {
		return nil, e
	}
	used := proof.ExactOperations()
	for _, face := range pair.faces {
		for _, use := range face.Outer {
			if used >= o.MaxOperations {
				return nil, geom.ErrConvergence
			}
			same, work, e := compareWrappedQuadraticUse(ctx, pair.edges[use.Edge], face.Surface, use, edgecheck.Options{MaxWork: o.MaxOperations - used, MaxBits: o.MaxCoefficientBits, MaxIntervals: 8192})
			used += work
			if e != nil {
				return nil, e
			}
			if !same {
				return nil, geom.ErrConvergence
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return &WrappedAlgebraicSeparatedShell{proof: proof, operations: used}, nil
}
