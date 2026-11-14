package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// PreparedWrappedStationDisplay retains one proved station loft with
// original lateral crease/rim identities, two caps and exact model parameters.
// Each internal station uses one shared indexed row certified on both bands.
type PreparedWrappedStationDisplay struct {
	display *PreparedWrappedThroughDisplay
	history WrappedLoftHistory
}

func (p *PreparedWrappedStationDisplay) Source() Solid {
	if p == nil {
		return Solid{}
	}
	return p.display.Source()
}
func (p *PreparedWrappedStationDisplay) Ancestry() StoredDisplayAncestry {
	if p == nil {
		return StoredDisplayAncestry{}
	}
	return p.display.Ancestry()
}
func (p *PreparedWrappedStationDisplay) SquaredDistance() *big.Rat {
	if p == nil {
		return nil
	}
	return p.display.SquaredDistance()
}
func (p *PreparedWrappedStationDisplay) Uses() []WrappedDisplayUse {
	if p == nil {
		return nil
	}
	return p.display.Uses()
}
func (p *PreparedWrappedStationDisplay) Place(ctx context.Context, pose placement.RationalRigid, o placement.ClosedDisplayOptions) (*PlacedStoredMixedDisplay, error) {
	if p == nil || p.display == nil {
		return nil, geom.ErrInvalid
	}
	return p.display.Place(ctx, pose, o)
}

// Stations returns owned original face and four-quarter rim identities.
func (p *PreparedWrappedStationDisplay) Stations() WrappedLoftHistory {
	if p == nil {
		return WrappedLoftHistory{}
	}
	h := p.history
	h.WallFaces = append([]int(nil), h.WallFaces...)
	h.StationRims = append([][4]int(nil), h.StationRims...)
	return h
}

// PrepareWrappedStationDisplay authenticates 1..16 original circular bands and
// their complete caps. The existing mesher proves exact original sample equality
// at every adjacent station, shares indices before construction, and checks full
// represented embedding. No rounded welding or caller-provided error is used.
// Band triangle checks and station samples are cumulative as in the existing
// mesher. Source/per-band/global contact proofs keep separate quotas. Display
// size is additionally capped at 100000 vertices/triangles, with atomic failure.
func (s Solid) PrepareWrappedStationDisplay(ctx context.Context, o CertifiedWrappedStationMeshOptions, validation placement.ClosedDisplayOptions) (*PreparedWrappedStationDisplay, error) {
	return s.prepareWrappedProfileStationDisplay(ctx, o, validation, true)
}

func (s Solid) prepareWrappedProfileStationDisplay(ctx context.Context, o CertifiedWrappedStationMeshOptions, validation placement.ClosedDisplayOptions, circular bool) (*PreparedWrappedStationDisplay, error) {
	n := len(s.faces) - 2
	if ctx == nil || n < 1 || n > 16 || len(s.edges) != 5*n+4 || len(s.vertices) != 4*(n+1) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	uses := 0
	for _, f := range s.faces {
		if len(f.Holes) != 0 || len(f.Outer) > 10 {
			return nil, geom.ErrConvergence
		}
		uses += len(f.Outer)
	}
	if uses != 10*n+8 {
		return nil, geom.ErrConvergence
	}
	if o.Band.MaxTriangles == 0 || o.Band.MaxTriangles > 100000 {
		o.Band.MaxTriangles = 100000
	}
	var boundary []wrappedDisplayBand
	report, e := s.tessellateWrappedProfileStationLoft(ctx, o, 1, &boundary, circular)
	if e != nil {
		return nil, e
	}
	if !report.Source.Complete || len(boundary) != n {
		return nil, geom.ErrConvergence
	}
	normalized := CertifiedWrappedFrustumOpenBoreMesh{Mesh: report.Mesh, OriginalFace: report.OriginalFace, SourceImageSquaredDistance: report.SourceImageSquaredDistance}
	p, e := s.bindPreparedWrappedBands(ctx, normalized, boundary, validation)
	if e != nil {
		return nil, e
	}
	return &PreparedWrappedStationDisplay{display: p, history: report.Source.History}, nil
}
