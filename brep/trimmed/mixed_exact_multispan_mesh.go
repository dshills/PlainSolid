package trimmed

import (
	"context"
	"plainsolid/geom"
)

// MixedExactMultiSpanMeshOptions bounds source/UV/lift work as before, plus
// cumulative exact knot-cell and clipped triangle-piece work across all lift
// refinement passes. The span defaults/limits match MultiSpanCurvedTrimMeshOptions.
// Span.MaxDegree defaults to2; explicit values through8 enable exact higher-
// degree rational residuals on C0 knot cells. MaxResidualOperations bounds their cumulative work
// across every triangle/refinement pass (default128M, maximum1B).
type MixedExactMultiSpanMeshOptions struct {
	MaxResidualOperations       int
	Face                        MixedExactCurvedMeshOptions
	Span                        MultiSpanTriangleBoundOptions
	MaxSpanCells, MaxSpanPieces int
}

// TessellateMixedExactMultiSpanRegionCertified extends the mixed native trim
// face proof to C0 positive rational multi-span supports. Degree<=2 is the
// default; explicit Span.MaxDegree enables degrees through8 per axis. Every
// SurfacePatch source is clamped with internal knot multiplicity<=degree:
// geom.NewSurfacePatch rejects full-multiplicity internal seams. Exact single-
// span construction and knot refinement preserve that invariant. Consequently
// adjacent original cells share their full homogeneous boundary curve, giving
// C0 across the entire seam, including genuine derivative creases.
// It integrates exact cellwise derivative bounds to obtain a whole-chart
// Lipschitz bound. Each display triangle residual is independently partitioned
// at all represented knots using exact rational UV intersections, then bounded
// on each source cell. The displayed triangle itself need not be split at the
// knot: its same-parameter residual covers both one-sided source continuations.
// Original trim samples and their indices remain fixed. Neither smoothing a
// crease nor approximate knot placement is assumed. This is an image-distance
// certificate, not a topology or embedding proof of the source surface.
func TessellateMixedExactMultiSpanRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, holes [][]PCurve, o MixedExactMultiSpanMeshOptions) (CertifiedMixedExactCurvedMesh, error) {
	return tessellateMixedExactMultiSpanRegionCertified(ctx, p, outer, holes, o, false)
}

func tessellateMixedExactMultiSpanRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, holes [][]PCurve, o MixedExactMultiSpanMeshOptions, alignSupportKnots bool) (CertifiedMixedExactCurvedMesh, error) {
	if o.Span.MaxDegree < 0 || o.Span.MaxDegree > 8 {
		return CertifiedMixedExactCurvedMesh{}, geom.ErrInvalid
	}
	work, e := newTubeLensSpanWork(MultiSpanTubeLensMeshOptions{MaxResidualOperations: o.MaxResidualOperations, Span: o.Span, MaxSpanCells: o.MaxSpanCells, MaxSpanPieces: o.MaxSpanPieces})
	if e != nil {
		return CertifiedMixedExactCurvedMesh{}, e
	}
	return tessellateMixedExactCurvedRegion(ctx, p, outer, holes, o.Face, &work, alignSupportKnots)
}
