package trimmed

import (
	"context"
	"plainsolid/geom"
)

// MultiSpanTubeLensMeshOptions adds cumulative knot-cell/residual-piece limits
// over both graph-face refinement passes.
type MultiSpanTubeLensMeshOptions struct {
	MaxResidualOperations       int
	AffineCharts                bool // Exact invertible affine XY projections; false retains identity-XY gate.
	Closed                      AlgebraicTubeLensMeshOptions
	Span                        MultiSpanTriangleBoundOptions
	MaxSpanCells, MaxSpanPieces int
}

// TessellateMultiSpanClosedCertified retains the same boundary samples
// and exact closed-display acceptance checks while using C0 piecewise rational
// degree<=2 Lipschitz and residual bounds. It never rounds knot intersections.
// The input must already be a sealed lens. Section crossings and boundary
// anchors must be independently proved by its source constructor.
func (s AlgebraicTubeLens) TessellateMultiSpanClosedCertified(ctx context.Context, o MultiSpanTubeLensMeshOptions) (CertifiedAlgebraicTubeLensMesh, error) {
	work, e := newTubeLensSpanWork(o)
	if e != nil {
		return CertifiedAlgebraicTubeLensMesh{}, e
	}
	return s.tessellateClosedTubeLens(ctx, o.Closed, &work, o.AffineCharts, nil, nil)
}
func newTubeLensSpanWork(o MultiSpanTubeLensMeshOptions) (multiSpanMeshWork, error) {
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
		return multiSpanMeshWork{}, geom.ErrInvalid
	}
	if o.Span.MaxDegree < 0 || o.Span.MaxDegree > 8 || o.Span.MaxResidualOperations < 0 || o.Span.MaxResidualOperations > 100000000 || o.MaxResidualOperations < 0 || o.MaxResidualOperations > 1000000000 {
		return multiSpanMeshWork{}, geom.ErrInvalid
	}
	return multiSpanMeshWork{options: MultiSpanCurvedTrimMeshOptions{Span: o.Span, MaxSpanCells: o.MaxSpanCells, MaxSpanPieces: o.MaxSpanPieces, MaxResidualOperations: o.MaxResidualOperations}}, nil
}
