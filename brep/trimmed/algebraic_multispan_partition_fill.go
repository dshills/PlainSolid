package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// MultiSpanAlgebraicPartitionMeshOptions bounds cumulative exact knot clipping
// over all triangles and refinement passes, including the chart Lipschitz proof.
// Limits default to100000 cells/pieces and are capped at1M.
type MultiSpanAlgebraicPartitionMeshOptions struct {
	Partition                   CurvedAlgebraicPartitionMeshOptions
	Span                        MultiSpanTriangleBoundOptions
	MaxSpanCells, MaxSpanPieces int
}

// TessellateMultiSpanCurvedPartitionCertified accepts C0 positive rational
// degree<=2 knot spans. Exact UV triangles are clipped against each crossed knot
// cell for residual certification; the actual shared mesh stays conforming.
// It provides source-image distance, not chart injectivity or display embedding.
func (s AlgebraicSubdivision) TessellateMultiSpanCurvedPartitionCertified(ctx context.Context, face int, line [3]*big.Rat, o MultiSpanAlgebraicPartitionMeshOptions) (CertifiedCurvedAlgebraicPartitionMesh, error) {
	if o.MaxSpanCells == 0 {
		o.MaxSpanCells = 100000
	}
	if o.MaxSpanPieces == 0 {
		o.MaxSpanPieces = 100000
	}
	if o.Span.MaxCells == 0 {
		o.Span.MaxCells = 1024
	}
	if o.Span.MaxPieces == 0 {
		o.Span.MaxPieces = 8192
	}
	if o.Span.MaxInputBits == 0 {
		o.Span.MaxInputBits = 16384
	}
	if o.MaxSpanCells < 1 || o.MaxSpanCells > 1000000 || o.MaxSpanPieces < 1 || o.MaxSpanPieces > 1000000 || o.Span.MaxCells < 1 || o.Span.MaxCells > 65536 || o.Span.MaxPieces < 1 || o.Span.MaxPieces > 1000000 || o.Span.MaxInputBits < 1 || o.Span.MaxInputBits > 65536 {
		return CertifiedCurvedAlgebraicPartitionMesh{}, geom.ErrInvalid
	}
	work := multiSpanMeshWork{options: MultiSpanCurvedTrimMeshOptions{Span: o.Span, MaxSpanCells: o.MaxSpanCells, MaxSpanPieces: o.MaxSpanPieces}}
	return s.tessellateCurvedPartition(ctx, face, line, o.Partition, &work)
}
