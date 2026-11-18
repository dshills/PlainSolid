//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// PreparedWrappedStockDisplay authenticates a canonical wrapped frustum with
// its two full planar disk caps, original periodic seam and rim edge uses.
// Its inherited source-image distance is distinct from material volume error.
type PreparedWrappedStockDisplay struct {
	display *PreparedWrappedThroughDisplay
}

func (p *PreparedWrappedStockDisplay) Source() Solid {
	if p == nil {
		return Solid{}
	}
	return p.display.Source()
}
func (p *PreparedWrappedStockDisplay) Ancestry() StoredDisplayAncestry {
	if p == nil {
		return StoredDisplayAncestry{}
	}
	return p.display.Ancestry()
}
func (p *PreparedWrappedStockDisplay) SquaredDistance() *big.Rat {
	if p == nil {
		return nil
	}
	return p.display.SquaredDistance()
}
func (p *PreparedWrappedStockDisplay) Uses() []WrappedDisplayUse {
	if p == nil {
		return nil
	}
	return p.display.Uses()
}
func (p *PreparedWrappedStockDisplay) Place(ctx context.Context, pose placement.RationalRigid, o placement.ClosedDisplayOptions) (*PlacedStoredMixedDisplay, error) {
	if p == nil || p.display == nil {
		return nil, geom.ErrInvalid
	}
	return p.display.Place(ctx, pose, o)
}

// PrepareWrappedFrustumStockDisplay re-proves the original 3-face stock and
// consumes the complete side/cap correspondence and represented embedding
// certificate. Reindexed native topology is supported; independently edited
// trims are rejected. Admission is bounded to 9 edges, 8 corners and 18 uses;
// display size is capped at 100000 vertices/triangles. Existing source, mesh
// and fresh display proof quotas remain independent. Failure returns no result.
func (s Solid) PrepareWrappedFrustumStockDisplay(ctx context.Context, o CertifiedWrappedFrustumMeshOptions, validation placement.ClosedDisplayOptions) (*PreparedWrappedStockDisplay, error) {
	if ctx == nil || len(s.faces) != 3 || len(s.edges) != 9 || len(s.vertices) != 8 {
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
	if uses != 18 {
		return nil, geom.ErrConvergence
	}
	w, _, e := s.ValidateCanonicalWrappedFrustum(ctx, o.Embedding)
	if e != nil {
		return nil, e
	}
	reference, _, e := NewWrappedFrustum(ctx, w, s.tolerance, o.Embedding)
	if e != nil {
		return nil, e
	}
	ids, e := matchCanonicalWrappedMaterial(ctx, s, reference, o.Embedding)
	if e != nil {
		return nil, e
	}
	if o.MaxTriangles == 0 || o.MaxTriangles > 100000 {
		o.MaxTriangles = 100000
	}
	mesh, e := TessellateWrappedFrustumClosedCertified(ctx, w, o)
	if e != nil {
		return nil, e
	}
	report := CertifiedWrappedFrustumOpenBoreMesh{Mesh: mesh.Mesh, OriginalFace: append([]int(nil), mesh.OriginalFace...), SourceImageSquaredDistance: mesh.SourceImageSquaredDistance}
	report.Walls[0] = mesh.Lateral
	report.RimVertices[0] = mesh.RimVertices
	for i, owner := range report.OriginalFace {
		if owner < 0 || owner >= len(ids) {
			return nil, geom.ErrDomain
		}
		report.OriginalFace[i] = ids[owner]
	}
	p, e := s.bindPreparedWrappedDisplay(ctx, report, []int{ids[0]}, validation)
	if e != nil {
		return nil, e
	}
	return &PreparedWrappedStockDisplay{display: p}, nil
}
