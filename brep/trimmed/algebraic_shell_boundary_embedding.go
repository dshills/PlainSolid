package trimmed

import (
	"context"
	"plainsolid/geom"
)

// AlgebraicShellBoundaryEmbeddingOptions adds a conservative all-segment-pairs
// quota to the independent boundary chord proof. MaxPairs defaults to 2,000,000
// and cannot exceed 16,000,000; sweep-excluded pairs still consume this quota.
type AlgebraicShellBoundaryEmbeddingOptions struct {
	Boundary AlgebraicShellBoundaryOptions
	MaxPairs int
}

// EmbeddedAlgebraicShellBoundary certifies the represented one-dimensional
// display only. It does not prove original curved arcs are separated, nor face
// coverage, a manifold face fill, or an embedded solid. PairChecks counts every
// potential pair of unique edge segments, including strict sweep exclusions.
type EmbeddedAlgebraicShellBoundary struct {
	Boundary   CertifiedAlgebraicShellBoundary
	PairChecks int
}

// TessellateEmbeddedBoundaryCertified retains the exact same certified chords
// as TessellateBoundaryCertified, then proves absence of nonincident display
// contacts with exact rational predicates. Shared indexed endpoints are legal;
// collinear overlap, crossings and equal positions with different indices are
// not. Opposite coedges reference one checked edge, avoiding artificial duplicate
// contacts. Failures return no partial boundary or embedding certificate.
func (s AlgebraicShellBoolean) TessellateEmbeddedBoundaryCertified(ctx context.Context, o AlgebraicShellBoundaryEmbeddingOptions) (EmbeddedAlgebraicShellBoundary, error) {
	var empty EmbeddedAlgebraicShellBoundary
	if ctx == nil || o.MaxPairs < 0 || o.MaxPairs > 16000000 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 2000000
	}
	network, e := s.TessellateBoundaryCertified(ctx, o.Boundary)
	if e != nil {
		return empty, e
	}
	checks, e := checkBoundaryPolylineEmbedding(ctx, network.Vertices, network.Edges, o.MaxPairs)
	if e != nil {
		return empty, e
	}
	return EmbeddedAlgebraicShellBoundary{Boundary: network, PairChecks: checks}, nil
}
