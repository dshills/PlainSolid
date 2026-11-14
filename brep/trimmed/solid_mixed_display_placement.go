package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// StoredDisplayAncestry retains original source indices and exact model-edge
// parameters. FaceVertices includes private interior IDs. All IDs refer to the
// same mesh indexing before and after placement; no rounded welding occurs.
type StoredDisplayAncestry struct {
	FaceVertices, EdgeVertices [][]int
	EdgeParameters             [][]*big.Rat
	CornerVertices             map[int]int
}

func (a StoredDisplayAncestry) clone() StoredDisplayAncestry {
	out := StoredDisplayAncestry{FaceVertices: make([][]int, len(a.FaceVertices)), EdgeVertices: make([][]int, len(a.EdgeVertices)), EdgeParameters: make([][]*big.Rat, len(a.EdgeParameters)), CornerVertices: make(map[int]int, len(a.CornerVertices))}
	for i, v := range a.FaceVertices {
		out.FaceVertices[i] = append([]int(nil), v...)
	}
	for i, v := range a.EdgeVertices {
		out.EdgeVertices[i] = append([]int(nil), v...)
	}
	for i, v := range a.EdgeParameters {
		out.EdgeParameters[i] = make([]*big.Rat, len(v))
		for j, q := range v {
			out.EdgeParameters[i][j] = new(big.Rat).Set(q)
		}
	}
	for k, v := range a.CornerVertices {
		out.CornerVertices[k] = v
	}
	return out
}

// PreparedStoredMixedDisplay is an immutable certificate bound to the original
// Solid. Only the proved single-shell or nested-material constructors create
// it: arbitrary mesh reports
// and caller-asserted deviations are not accepted. Solid's immutable surface/
// curve implementation contract continues to apply.
type PreparedStoredMixedDisplay struct {
	sourceSeparationSquared *big.Rat // optional strict original multi-shell clearance
	source                  Solid
	display                 *placement.PreparedClosedDisplay
	squared                 *big.Rat
	ancestry                StoredDisplayAncestry
}

// PrepareMixedClosedDisplay generates and proves the supported native trim mesh
// first, then independently proves represented component nesting. Meshing quotas
// are unchanged; display validation has separate bounded work. The existing
// mesher's one connected shell and original supported surface/trim gates remain.
// This proves source-image/display proximity, not source material embedding or
// ideal volume. All failure paths return nil.
func (s Solid) PrepareMixedClosedDisplay(ctx context.Context, meshing StoredMixedClosedMeshOptions, display placement.ClosedDisplayOptions) (*PreparedStoredMixedDisplay, error) {
	report, e := s.TessellateMixedClosedCertified(ctx, meshing)
	if e != nil {
		return nil, e
	}
	if report.SquaredDistance == nil || report.SquaredDistance.Sign() < 0 || report.SquaredDistance.Num().BitLen() > 65536 || report.SquaredDistance.Denom().BitLen() > 65536 {
		return nil, geom.ErrConvergence
	}
	prepared, e := placement.PrepareClosedDisplay(ctx, report.Mesh, report.OriginalFace, display)
	if e != nil {
		return nil, e
	}
	ancestry := StoredDisplayAncestry{report.FaceVertices, report.Network.EdgeVertices, report.Network.EdgeParameters, report.Network.CornerVertices}
	return &PreparedStoredMixedDisplay{source: s, display: prepared, squared: new(big.Rat).Set(report.SquaredDistance), ancestry: ancestry}, nil
}
func (p *PreparedStoredMixedDisplay) Source() Solid {
	if p == nil {
		return Solid{}
	}
	return p.source
}
func (p *PreparedStoredMixedDisplay) Ancestry() StoredDisplayAncestry {
	if p == nil {
		return StoredDisplayAncestry{}
	}
	return p.ancestry.clone()
}
func (p *PreparedStoredMixedDisplay) SourceSquaredDistance() *big.Rat {
	if p == nil || p.squared == nil {
		return nil
	}
	return new(big.Rat).Set(p.squared)
}

// PlacedStoredMixedDisplay is a source-authenticated bound, unlike the generic
// placement helper's conditional inherited error. Its squared distance includes
// the exact measured additional rounding and bounds both directed distances to
// the exact rigid image of this source's boundary. Original face/edge/corner
// ancestry is unchanged. No transformed native BRep or volume claim is made.
type PlacedStoredMixedDisplay struct {
	source  *PreparedStoredMixedDisplay
	display *placement.PlacedClosedDisplay
	squared *big.Rat
}

func (p *PlacedStoredMixedDisplay) Display() *placement.PlacedClosedDisplay {
	if p == nil {
		return nil
	}
	return p.display
}
func (p *PlacedStoredMixedDisplay) Source() Solid {
	if p == nil {
		return Solid{}
	}
	return p.source.Source()
}
func (p *PlacedStoredMixedDisplay) Ancestry() StoredDisplayAncestry {
	if p == nil {
		return StoredDisplayAncestry{}
	}
	return p.source.Ancestry()
}
func (p *PlacedStoredMixedDisplay) SquaredDistance() *big.Rat {
	if p == nil || p.squared == nil {
		return nil
	}
	return new(big.Rat).Set(p.squared)
}

// Place reuses the immutable original proof; it never remeshes or reinterprets
// a caller's bound. World topology/contact/nesting is freshly proved. For source
// squared bound s and rounding r, sqrt(s)<=max(1,s) supplies a rational upper
// bound s+2*r*max(1,s)+r² without numerical square roots. For s=0 it uses r².
// Fixed arithmetic accepts at most 65536 bits per input and <=200000-bit output.
func (p *PreparedStoredMixedDisplay) Place(ctx context.Context, pose placement.RationalRigid, o placement.ClosedDisplayOptions) (*PlacedStoredMixedDisplay, error) {
	if p == nil || p.display == nil || p.squared == nil {
		return nil, geom.ErrInvalid
	}
	world, e := p.display.Place(ctx, pose, o)
	if e != nil {
		return nil, e
	}
	return p.bindPlacedDisplay(ctx, world)
}
func (p *PreparedStoredMixedDisplay) bindPlacedDisplay(ctx context.Context, world *placement.PlacedClosedDisplay) (*PlacedStoredMixedDisplay, error) {
	r := world.RoundingUpper()
	if r.Num().BitLen() > 65536 || r.Denom().BitLen() > 65536 {
		return nil, geom.ErrConvergence
	}
	squared := new(big.Rat).Mul(r, r)
	if p.squared.Sign() != 0 {
		upper := new(big.Rat).Set(p.squared)
		if upper.Cmp(big.NewRat(1, 1)) < 0 {
			upper.SetInt64(1)
		}
		cross := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(r, upper))
		squared.Add(squared, cross)
		squared.Add(squared, p.squared)
	}
	if squared.Num().BitLen() > 200000 || squared.Denom().BitLen() > 200000 {
		return nil, geom.ErrConvergence
	}
	if p.sourceSeparationSquared != nil && new(big.Rat).Mul(big.NewRat(4, 1), squared).Cmp(p.sourceSeparationSquared) >= 0 {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return &PlacedStoredMixedDisplay{p, world, squared}, nil
}
