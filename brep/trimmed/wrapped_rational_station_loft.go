package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// WrappedRationalStationProof describes a canonical star-shaped rational
// quadrant profile loft. Scales are positive XY control-polygon scales, not
// circular radii. Source bands are homothetic between strictly ordered planes.
// History always indexes the original native material.
type WrappedRationalStationProof struct {
	Center          [2]*big.Rat
	Heights, Scales []*big.Rat
	History         WrappedLoftHistory
	Complete        bool
}

// NewWrappedRationalStationLoft admits 1..16 certified rational quadrant bands.
// Positive weights and the canonical ordered square control polygon establish
// a strictly polar-monotone simple profile at every height. Adjacent whole rim
// evaluators must agree exactly: equal endpoint corners/scales are insufficient.
// The existing native join re-proves zero model/pcurve error on all uses before
// admitting a result. Ordered disjoint open height slabs establish global source
// embedding, including reentrant slope creases. Surface budgets apply per band
// and to the bounded shared-edge proof; unsupported profiles fail atomically.
func NewWrappedRationalStationLoft(ctx context.Context, bands []*geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions) (Solid, WrappedLoftHistory, error) {
	if ctx == nil || len(bands) < 1 || len(bands) > 16 || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, WrappedLoftHistory{}, geom.ErrInvalid
	}
	var previous geom.WrappedFrustumEmbedding
	for i, w := range bands {
		if e := ctx.Err(); e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		p, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
		if e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		if !p.Complete || i > 0 && (p.CenterX.Cmp(previous.CenterX) != 0 || p.CenterY.Cmp(previous.CenterY) != 0 || p.LowerZ.Cmp(previous.UpperZ) != 0 || p.LowerRadius.Cmp(previous.UpperRadius) != 0) {
			return Solid{}, WrappedLoftHistory{}, geom.ErrConvergence
		}
		previous = p
	}
	return joinWrappedStationBands(ctx, bands, tolerance, o)
}

// ValidateWrappedRationalStationLoft authenticates original source topology and
// complete rational rim identities, including after native reload/reindexing.
// It does not widen the circular point/line/material query APIs.
func (s Solid) ValidateWrappedRationalStationLoft(ctx context.Context, o geom.SurfacePatchOptions) (WrappedRationalStationProof, error) {
	p, _, e := s.validateWrappedProfileStationLoft(ctx, o, false)
	if e != nil {
		return WrappedRationalStationProof{}, e
	}
	return WrappedRationalStationProof{p.Center, p.Heights, p.Radii, p.History, p.Complete}, nil
}

// PrepareWrappedRationalStationDisplay certifies the complete native rational
// profile loft, including exact shared rim samples, both full-profile caps,
// two-sided source-image bounds and independent closed display contact checks.
// It uses the same bounded mesh algorithm as circular preparation; 16 bands and
// 100000 display vertices/triangles are maximum. No circular surrogate or
// tolerance-based stitching is used and no volume-error bound is asserted.
func (s Solid) PrepareWrappedRationalStationDisplay(ctx context.Context, o CertifiedWrappedStationMeshOptions, validation placement.ClosedDisplayOptions) (*PreparedWrappedStationDisplay, error) {
	return s.prepareWrappedProfileStationDisplay(ctx, o, validation, false)
}
