package trimmed

import (
	"context"
	"plainsolid/geom"
)

// MultiSpanClosedLensMeshOptions adds cumulative knot-cell/residual-piece limits
// over both halves and all four curved-face refinement passes.
type MultiSpanClosedLensMeshOptions struct {
	Closed                      ClosedAlgebraicLensMeshOptions
	Span                        MultiSpanTriangleBoundOptions
	MaxSpanCells, MaxSpanPieces int
}

// TessellateMultiSpanClosedHalvesCertified retains the same boundary samples
// and exact closed-display acceptance checks while using C0 piecewise rational
// degree<=2 Lipschitz and residual bounds. It never rounds knot intersections.
func (s AlgebraicLensPlaneSplit) TessellateMultiSpanClosedHalvesCertified(ctx context.Context, o MultiSpanClosedLensMeshOptions) (CertifiedClosedAlgebraicLensMesh, error) {
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
		return CertifiedClosedAlgebraicLensMesh{}, geom.ErrInvalid
	}
	work := multiSpanMeshWork{options: MultiSpanCurvedTrimMeshOptions{Span: o.Span, MaxSpanCells: o.MaxSpanCells, MaxSpanPieces: o.MaxSpanPieces}}
	return s.tessellateClosedHalves(ctx, o.Closed, &work)
}
