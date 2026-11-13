package trimmed

import (
	"context"
	"plainsolid/geom"
)

// MixedExactMultiSpanMeshOptions bounds source/UV/lift work as before, plus
// cumulative exact knot-cell and clipped triangle-piece work across all lift
// refinement passes. The span defaults/limits match MultiSpanCurvedTrimMeshOptions.
// MaxDegree must be0(default2),1,or2; higher-degree supports are not accepted here.
type MixedExactMultiSpanMeshOptions struct {
	Face                        MixedExactCurvedMeshOptions
	Span                        MultiSpanTriangleBoundOptions
	MaxSpanCells, MaxSpanPieces int
}

// TessellateMixedExactMultiSpanRegionCertified extends the mixed native trim
// face proof to C0 positive rational multi-span supports of degree<=2 per axis.
// It integrates exact cellwise derivative bounds to obtain a whole-chart
// Lipschitz bound. Each display triangle residual is independently partitioned
// at all represented knots using exact rational UV intersections, then bounded
// on each source cell. The displayed triangle itself need not be split at the
// knot: its same-parameter residual covers both one-sided source continuations.
// Original trim samples and their indices remain fixed. Neither smoothing a
// crease nor approximate knot placement is assumed. This is an image-distance
// certificate, not a topology or embedding proof of the source surface.
func TessellateMixedExactMultiSpanRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, holes [][]PCurve, o MixedExactMultiSpanMeshOptions) (CertifiedMixedExactCurvedMesh, error) {
	if o.Span.MaxDegree < 0 || o.Span.MaxDegree > 2 {
		return CertifiedMixedExactCurvedMesh{}, geom.ErrInvalid
	}
	work, e := newTubeLensSpanWork(MultiSpanTubeLensMeshOptions{Span: o.Span, MaxSpanCells: o.MaxSpanCells, MaxSpanPieces: o.MaxSpanPieces})
	if e != nil {
		return CertifiedMixedExactCurvedMesh{}, e
	}
	return tessellateMixedExactCurvedRegion(ctx, p, outer, holes, o.Face, &work)
}
