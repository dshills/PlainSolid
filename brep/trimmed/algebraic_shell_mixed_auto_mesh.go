package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedIntersectionMeshOptions sets independent limits for the original
// boundary, exact support discovery, planar and roof fill, and final
// assembly. Planar discovery recognizes represented exact planes; other
// faces must be parts of one qualified graph roof per operand.
type MixedIntersectionMeshOptions struct {
	Boundary          AlgebraicShellBoundaryEmbeddingOptions
	Planar            MixedAffineFragmentFillOptions
	Roof              MixedRoofCollarOptions
	Assembly          MixedCompleteMeshOptions
	SupportOperations int
}

// MixedDifferenceMeshOptions uses the same stage limits for the exact
// complement and reversed-tool proofs.
type MixedDifferenceMeshOptions = MixedIntersectionMeshOptions

// TessellateIntersectionMeshCertified builds the full certified display of
// a qualified mixed original intersection, with no caller-supplied face
// roles or plane guesses. Exact support identity supplies candidate roles;
// all existing continuous trim, source approximation and final collision
// proofs still run. Unsupported inputs produce no partial mesh.
func (s *MixedAlgebraicShellBoolean) TessellateIntersectionMeshCertified(ctx context.Context,
	opts MixedIntersectionMeshOptions) (*CertifiedMixedAlgebraicShellMesh, error) {
	return s.tessellateCompleteMixedMesh(ctx, opts, parametric.Intersection)
}

// TessellateDifferenceMeshCertified automatically recognizes the supports
// of a qualified mixed difference. Material complement and tool-face
// orientation are independently proved before assembling all components.
func (s *MixedAlgebraicShellBoolean) TessellateDifferenceMeshCertified(ctx context.Context,
	opts MixedDifferenceMeshOptions) (*CertifiedMixedAlgebraicShellMesh, error) {
	return s.tessellateCompleteMixedMesh(ctx, opts, parametric.Difference)
}

func (s *MixedAlgebraicShellBoolean) tessellateCompleteMixedMesh(ctx context.Context,
	opts MixedIntersectionMeshOptions, operation parametric.BooleanOp) (*CertifiedMixedAlgebraicShellMesh, error) {
	if ctx == nil || s == nil || s.section == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if s.boundary.op != operation {
		return nil, geom.ErrUnrepresentable
	}
	if opts.SupportOperations == 0 {
		opts.SupportOperations = 2000000
	}
	if opts.SupportOperations < 1 || opts.SupportOperations > 16000000 {
		return nil, geom.ErrInvalid
	}
	if opts.Boundary.Boundary.Deflection <= 0 || opts.Roof.Width <= 0 {
		return nil, geom.ErrInvalid
	}
	type owner struct{ operand, original int }
	byOwner := map[owner]*MixedPlanarFragmentProposal{}
	roofOwners := [2]int{-1, -1}
	var planes []MixedPlanarFragmentProposal
	var roofs []int
	budget := opts.SupportOperations
	for id, f := range s.boundary.faces {
		if f.Operand < 0 || f.Operand > 1 {
			return nil, geom.ErrInvalid
		}
		key := owner{f.Operand, f.Original}
		proposal, ok := byOwner[key]
		if !ok {
			source, e := s.OriginalFace(id)
			if e != nil {
				return nil, e
			}
			if budget < 1 {
				return nil, geom.ErrConvergence
			}
			patch, e := exactFaceSupportPatch(source, geom.SurfacePatchOptions{MaxOperations: budget, MaxCoefficientBits: 4096})
			if e != nil {
				return nil, e
			}
			if e = ctx.Err(); e != nil {
				return nil, e
			}
			budget -= patch.ExactOperations()
			if budget < 1 {
				return nil, geom.ErrConvergence
			}
			pp, yes, e := mixedExactPlaneProposal(ctx, patch, id, &budget)
			if e != nil {
				return nil, e
			}
			if yes {
				proposal = &pp
			}
			byOwner[key] = proposal
		}
		if proposal != nil {
			p := *proposal
			p.Face = id
			planes = append(planes, p)
			continue
		}
		if roofOwners[f.Operand] != -1 && roofOwners[f.Operand] != f.Original {
			return nil, geom.ErrUnrepresentable
		}
		roofOwners[f.Operand] = f.Original
		roofs = append(roofs, id)
	}
	if roofOwners[0] < 0 || roofOwners[1] < 0 || len(planes) == 0 {
		return nil, geom.ErrUnrepresentable
	}
	n, e := s.TessellateEmbeddedBoundaryCertified(ctx, opts.Boundary)
	if e != nil {
		return nil, e
	}
	planar, e := n.FillPlanarFragmentsCertified(ctx, planes, opts.Planar)
	if e != nil {
		return nil, e
	}
	var fills []*CertifiedMixedRoofFragment
	for _, id := range roofs {
		other := roofOwners[s.boundary.faces[id].Operand^1]
		var r *CertifiedMixedRoofFragment
		if operation == parametric.Difference {
			r, e = n.TessellateDifferenceRoofFragmentCertified(ctx, id, other, opts.Roof)
		} else {
			r, e = n.TessellateRoofFragmentCertified(ctx, id, other, opts.Roof)
		}
		if e != nil {
			return nil, e
		}
		fills = append(fills, r)
	}
	return n.AssembleCompleteMeshCertified(ctx, planar, fills, opts.Assembly)
}
