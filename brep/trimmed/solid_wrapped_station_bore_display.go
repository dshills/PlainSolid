package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// PreparedWrappedStationBoreDisplay authenticates independently partitioned
// outer and inward inner station walls with two annular caps. All
// returned face/edge/use IDs refer to the original native material, not to
// reconstructed capped shells used privately by its source certificate.
type PreparedWrappedStationBoreDisplay struct {
	display *PreparedWrappedThroughDisplay
	walls   [2][]int
	caps    [2]int
}

func (p *PreparedWrappedStationBoreDisplay) Source() Solid {
	if p == nil {
		return Solid{}
	}
	return p.display.Source()
}
func (p *PreparedWrappedStationBoreDisplay) Ancestry() StoredDisplayAncestry {
	if p == nil {
		return StoredDisplayAncestry{}
	}
	return p.display.Ancestry()
}
func (p *PreparedWrappedStationBoreDisplay) SquaredDistance() *big.Rat {
	if p == nil {
		return nil
	}
	return p.display.SquaredDistance()
}
func (p *PreparedWrappedStationBoreDisplay) Uses() []WrappedDisplayUse {
	if p == nil {
		return nil
	}
	return p.display.Uses()
}
func (p *PreparedWrappedStationBoreDisplay) Place(ctx context.Context, pose placement.RationalRigid, o placement.ClosedDisplayOptions) (*PlacedStoredMixedDisplay, error) {
	if p == nil || p.display == nil {
		return nil, geom.ErrInvalid
	}
	return p.display.Place(ctx, pose, o)
}

// Walls returns owned original outer and inner face IDs in axial order.
func (p *PreparedWrappedStationBoreDisplay) Walls() [2][]int {
	if p == nil {
		return [2][]int{}
	}
	return [2][]int{append([]int(nil), p.walls[0]...), append([]int(nil), p.walls[1]...)}
}

// Caps returns original lower and upper annular cap face IDs.
func (p *PreparedWrappedStationBoreDisplay) Caps() [2]int {
	if p == nil {
		return [2]int{-1, -1}
	}
	return p.caps
}

// PrepareWrappedStationBoreDisplay consumes the complete original hollow-loft
// source and display certificate. It preserves independent station partitions,
// exact shared rim identity and inward ownership; no point welding is used.
// Admission is capped at 16 total wall bands and 100000 display vertices/triangles.
// Existing cumulative triangle/station budgets and separately bounded source,
// local and final contact proofs remain in force. No volume-error bound is claimed.
func (s Solid) PrepareWrappedStationBoreDisplay(ctx context.Context, o CertifiedWrappedStationMeshOptions, validation placement.ClosedDisplayOptions) (*PreparedWrappedStationBoreDisplay, error) {
	return s.prepareWrappedProfileStationBoreDisplay(ctx, o, validation, true)
}

// PrepareWrappedRationalStationBoreDisplay binds a certified noncircular
// annular mesh to the original native faces, uses and shared station rims.
func (s Solid) PrepareWrappedRationalStationBoreDisplay(ctx context.Context, o CertifiedWrappedStationMeshOptions, validation placement.ClosedDisplayOptions) (*PreparedWrappedStationBoreDisplay, error) {
	return s.prepareWrappedProfileStationBoreDisplay(ctx, o, validation, false)
}

func (s Solid) prepareWrappedProfileStationBoreDisplay(ctx context.Context, o CertifiedWrappedStationMeshOptions, validation placement.ClosedDisplayOptions, circular bool) (*PreparedWrappedStationBoreDisplay, error) {
	n := len(s.faces) - 2
	if ctx == nil || n < 2 || n > 16 || len(s.edges) != 5*n+8 || len(s.vertices) != 4*(n+2) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	uses := 0
	for _, f := range s.faces {
		if len(f.Outer) > 10 || len(f.Holes) > 1 {
			return nil, geom.ErrConvergence
		}
		uses += len(f.Outer)
		for _, hole := range f.Holes {
			if len(hole) > 4 {
				return nil, geom.ErrConvergence
			}
			uses += len(hole)
		}
	}
	if uses != 10*n+16 {
		return nil, geom.ErrConvergence
	}
	if o.Band.MaxTriangles == 0 || o.Band.MaxTriangles > 100000 {
		o.Band.MaxTriangles = 100000
	}
	var boundary []wrappedDisplayBand
	report, e := s.tessellateWrappedProfileStationBore(ctx, o, &boundary, circular)
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
	return &PreparedWrappedStationBoreDisplay{display: p, walls: [2][]int{append([]int(nil), report.Source.OuterWalls...), append([]int(nil), report.Source.InnerWalls...)}, caps: report.Source.Caps}, nil
}
