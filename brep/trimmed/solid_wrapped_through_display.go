package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// WrappedDisplayUse retains one original face/loop/coedge occurrence. Samples
// follow the stored model edge's increasing Range; Reversed and FaceReversed
// retain original loop/material orientation without changing that parameter.
type WrappedDisplayUse struct {
	Face, Loop, Use, Edge  int
	Reversed, FaceReversed bool
	Vertices               []int
	ModelParameters        []*big.Rat
}

// PreparedWrappedThroughDisplay binds one canonical eccentric circular-frustum
// through bore to its original periodic walls and two exact annular cap trims.
// Only a fully re-proved native tube is accepted, including reindexed/reloaded
// sources. Its private report cannot be replaced by caller-asserted deviation.
type PreparedWrappedThroughDisplay struct {
	prepared *PreparedStoredMixedDisplay
	uses     []WrappedDisplayUse
}

func (p *PreparedWrappedThroughDisplay) Source() Solid {
	if p == nil {
		return Solid{}
	}
	return p.prepared.Source()
}
func (p *PreparedWrappedThroughDisplay) Ancestry() StoredDisplayAncestry {
	if p == nil {
		return StoredDisplayAncestry{}
	}
	return p.prepared.Ancestry()
}
func (p *PreparedWrappedThroughDisplay) SquaredDistance() *big.Rat {
	if p == nil {
		return nil
	}
	return p.prepared.SourceSquaredDistance()
}
func (p *PreparedWrappedThroughDisplay) Uses() []WrappedDisplayUse {
	if p == nil {
		return nil
	}
	out := append([]WrappedDisplayUse(nil), p.uses...)
	for i := range out {
		out[i].Vertices = append([]int(nil), out[i].Vertices...)
		row := out[i].ModelParameters
		out[i].ModelParameters = make([]*big.Rat, len(row))
		for j, q := range row {
			out[i].ModelParameters[j] = new(big.Rat).Set(q)
		}
	}
	return out
}
func (p *PreparedWrappedThroughDisplay) Place(ctx context.Context, pose placement.RationalRigid, o placement.ClosedDisplayOptions) (*PlacedStoredMixedDisplay, error) {
	if p == nil || p.prepared == nil {
		return nil, geom.ErrInvalid
	}
	return p.prepared.Place(ctx, pose, o)
}

// PrepareWrappedFrustumThroughDisplay uses the existing complete source/rim/
// annular-fill/lift certificates, then fresh exact display nesting. Meshing and
// display validation have separate existing quotas. Source validation and the
// existing mesher each retain their own bounded surface proof budget; the
// per-edge mapping is linear in the admitted mesh. Native admission is capped
// at 4 faces, 18 edges, 16 corners, and each display at 100000 vertices/triangles.
// Model ranges must be the proved original canonical U (rim) or [0,1] (seam).
// Source and display embeddings are separate; no volume-error claim is made.
func (s Solid) PrepareWrappedFrustumThroughDisplay(ctx context.Context, o CertifiedWrappedFrustumMeshOptions, validation placement.ClosedDisplayOptions) (*PreparedWrappedThroughDisplay, error) {
	return s.prepareWrappedOpenDisplay(ctx, o, validation, "tube")
}

// prepareWrappedOpenDisplay shares only original-use binding; source-specific
// admission and complete meshing remain separate existing proofs.
func (s Solid) prepareWrappedOpenDisplay(ctx context.Context, o CertifiedWrappedFrustumMeshOptions, validation placement.ClosedDisplayOptions, kind string) (*PreparedWrappedThroughDisplay, error) {
	faces := 4
	if kind == "blind-bore" {
		faces = 5
	} else if kind != "tube" {
		return nil, geom.ErrInvalid
	}
	if ctx == nil || len(s.faces) != faces || len(s.edges) > 18 || len(s.vertices) > 16 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	uses := 0
	for _, f := range s.faces {
		if len(f.Holes) > 1 || len(f.Outer) > 10 {
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
	if uses != 36 {
		return nil, geom.ErrConvergence
	}
	source, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o.Embedding)
	if e != nil {
		return nil, fmt.Errorf("source: %w", e)
	}
	if source.Kind != kind || !source.Complete {
		return nil, geom.ErrConvergence
	}
	if o.MaxTriangles == 0 || o.MaxTriangles > 100000 {
		o.MaxTriangles = 100000
	}
	var report CertifiedWrappedFrustumOpenBoreMesh
	if kind == "tube" {
		report, e = s.TessellateCanonicalWrappedFrustumThroughBoreCertified(ctx, o)
	} else {
		report, e = s.TessellateCanonicalWrappedFrustumOpenBoreCertified(ctx, o)
	}
	if e != nil {
		return nil, fmt.Errorf("mesh: %w", e)
	}
	return s.bindPreparedWrappedDisplay(ctx, report, []int{source.OuterFaces[0], source.InnerFaces[0]}, validation)
}

// bindPreparedWrappedDisplay is internal: only complete authenticated source
// constructors may supply a report. It is not a caller-asserted certificate.
func (s Solid) bindPreparedWrappedDisplay(ctx context.Context, report CertifiedWrappedFrustumOpenBoreMesh, wallFaces []int, validation placement.ClosedDisplayOptions) (*PreparedWrappedThroughDisplay, error) {
	if len(report.Mesh.Vertices) > 100000 || report.SourceImageSquaredDistance == nil || report.SourceImageSquaredDistance.Sign() < 0 || report.SourceImageSquaredDistance.Num().BitLen() > 65536 || report.SourceImageSquaredDistance.Denom().BitLen() > 65536 {
		return nil, geom.ErrConvergence
	}
	ancestry := StoredDisplayAncestry{FaceVertices: make([][]int, len(s.faces)), EdgeVertices: make([][]int, len(s.edges)), EdgeParameters: make([][]*big.Rat, len(s.edges)), CornerVertices: map[int]int{}}
	record := func(edge int, ids []int, params []*big.Rat) error {
		if edge < 0 || edge >= len(s.edges) || len(ids) < 2 || len(ids) != len(params) {
			return geom.ErrDomain
		}
		native := s.edges[edge]
		if params[0].Cmp(ratInset(native.Range.Min)) != 0 || params[len(params)-1].Cmp(ratInset(native.Range.Max)) != 0 {
			return geom.ErrConvergence
		}
		if old := ancestry.EdgeVertices[edge]; old != nil {
			if len(old) != len(ids) {
				return geom.ErrDomain
			}
			for i := range old {
				if old[i] != ids[i] || ancestry.EdgeParameters[edge][i].Cmp(params[i]) != 0 {
					return geom.ErrDomain
				}
			}
			return nil
		}
		ancestry.EdgeVertices[edge] = ids
		ancestry.EdgeParameters[edge] = params
		for i, corner := range []int{native.Start, native.End} {
			id := ids[0]
			if i == 1 {
				id = ids[len(ids)-1]
			}
			if old, ok := ancestry.CornerVertices[corner]; ok && old != id {
				return fmt.Errorf("corner %d ids %d/%d: %w", corner, old, id, geom.ErrDomain)
			}
			ancestry.CornerVertices[corner] = id
		}
		return nil
	}
	for wall, face := range wallFaces {
		grid := report.Walls[wall]
		u, v := grid.UParameters, grid.VParameters
		nu := len(u) - 1
		if nu < 4 || len(v) < 2 || len(report.RimVertices[wall][0]) != nu || len(report.RimVertices[wall][1]) != nu {
			return nil, geom.ErrDomain
		}
		for _, use := range s.faces[face].Outer {
			if use.Curve != nil {
				return nil, geom.ErrConvergence
			}
			var ids []int
			var params []*big.Rat
			if use.UVStart.Y == use.UVEnd.Y {
				end := -1
				if use.UVStart.Y == v[0] {
					end = 0
				}
				if use.UVStart.Y == v[len(v)-1] {
					end = 1
				}
				if end < 0 {
					return nil, geom.ErrConvergence
				}
				lo, hi := use.UVStart.X, use.UVEnd.X
				if lo > hi {
					lo, hi = hi, lo
				}
				for i, x := range u {
					if x >= lo && x <= hi {
						ids = append(ids, report.RimVertices[wall][end][i%nu])
						params = append(params, ratInset(x))
					}
				}
				if len(params) < 2 || params[0].Cmp(ratInset(lo)) != 0 || params[len(params)-1].Cmp(ratInset(hi)) != 0 {
					return nil, geom.ErrConvergence
				}
			} else if use.UVStart.X == use.UVEnd.X && (use.UVStart.X == u[0] || use.UVStart.X == u[nu]) {
				base := report.RimVertices[wall][0][0]
				for j, y := range v {
					id := base + j
					if id < 0 || id >= len(report.Mesh.Vertices) || j >= len(grid.Mesh.Vertices) || report.Mesh.Vertices[id] != grid.Mesh.Vertices[j] {
						return nil, geom.ErrDomain
					}
					ids = append(ids, id)
					params = append(params, new(big.Rat).Quo(new(big.Rat).Sub(ratInset(y), ratInset(v[0])), new(big.Rat).Sub(ratInset(v[len(v)-1]), ratInset(v[0]))))
				}
			} else {
				return nil, geom.ErrConvergence
			}
			if e := record(use.Edge, ids, params); e != nil {
				return nil, fmt.Errorf("edge %d: %w", use.Edge, e)
			}
		}
	}
	for _, row := range ancestry.EdgeVertices {
		if len(row) < 2 {
			return nil, geom.ErrConvergence
		}
	}
	out := &PreparedWrappedThroughDisplay{}
	for fi, f := range s.faces {
		for li, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for ui, use := range loop {
				out.uses = append(out.uses, WrappedDisplayUse{fi, li, ui, use.Edge, use.Reversed, f.Reversed, ancestry.EdgeVertices[use.Edge], ancestry.EdgeParameters[use.Edge]})
			}
		}
	}
	sets := make([]map[int]bool, len(s.faces))
	for i := range sets {
		sets[i] = map[int]bool{}
	}
	for i, tr := range report.Mesh.Triangles {
		face := report.OriginalFace[i]
		if face < 0 || face >= len(sets) {
			return nil, geom.ErrDomain
		}
		for _, id := range tr {
			sets[face][id] = true
		}
	}
	for face, set := range sets {
		for id := range set {
			ancestry.FaceVertices[face] = append(ancestry.FaceVertices[face], id)
		}
		sort.Ints(ancestry.FaceVertices[face])
	}
	display, e := placement.PrepareClosedDisplay(ctx, report.Mesh, report.OriginalFace, validation)
	if e != nil {
		return nil, fmt.Errorf("display: %w", e)
	}
	shells := display.Shells()
	if len(shells) != 1 || shells[0].Depth != 0 {
		return nil, geom.ErrDomain
	}
	out.prepared = &PreparedStoredMixedDisplay{source: s, display: display, squared: new(big.Rat).Set(report.SourceImageSquaredDistance), ancestry: ancestry}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
