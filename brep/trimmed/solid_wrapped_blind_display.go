package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// PreparedWrappedBlindDisplay authenticates a single native eccentric circular
// frustum blind bore: one annular opening, its opposite full stock cap, and the
// retained inward bore floor. Original wall, cap, floor and coedge identities
// survive placement. It does not claim a bound on material volume error.
type PreparedWrappedBlindDisplay struct {
	display *PreparedWrappedThroughDisplay
}

func (p *PreparedWrappedBlindDisplay) Source() Solid {
	if p == nil {
		return Solid{}
	}
	return p.display.Source()
}
func (p *PreparedWrappedBlindDisplay) Ancestry() StoredDisplayAncestry {
	if p == nil {
		return StoredDisplayAncestry{}
	}
	return p.display.Ancestry()
}
func (p *PreparedWrappedBlindDisplay) SquaredDistance() *big.Rat {
	if p == nil {
		return nil
	}
	return p.display.SquaredDistance()
}
func (p *PreparedWrappedBlindDisplay) Uses() []WrappedDisplayUse {
	if p == nil {
		return nil
	}
	return p.display.Uses()
}
func (p *PreparedWrappedBlindDisplay) Place(ctx context.Context, pose placement.RationalRigid, o placement.ClosedDisplayOptions) (*PlacedStoredMixedDisplay, error) {
	if p == nil || p.display == nil {
		return nil, geom.ErrInvalid
	}
	return p.display.Place(ctx, pose, o)
}

// PrepareWrappedFrustumBlindDisplay consumes the existing complete blind-bore
// meshing certificate and independently checks represented embedding/orientation.
// Admission requires the original canonical 5-face, 18-edge, 16-corner, 36-use
// native topology; either entry end is supported. Returned samples retain each
// original edge's increasing parameter and both original orientation flags.
// Mesh size is capped at 100000 vertices/triangles; surface, meshing and fresh
// global display validation retain separate bounded work quotas. Any failure
// returns no preparation. Unsupported wrapped sources are never approximated
// into this domain.
func (s Solid) PrepareWrappedFrustumBlindDisplay(ctx context.Context, o CertifiedWrappedFrustumMeshOptions, validation placement.ClosedDisplayOptions) (*PreparedWrappedBlindDisplay, error) {
	p, e := s.prepareWrappedOpenDisplay(ctx, o, validation, "blind-bore")
	if e != nil {
		return nil, e
	}
	return &PreparedWrappedBlindDisplay{display: p}, nil
}
