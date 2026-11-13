package draft

import (
	"context"
	"plainsolid/brep"
)

// RemoveSupport removes one complete unique supporting face of an exact convex
// polyhedron and reconstructs the remaining halfspaces. This can recover a
// planar chamfer/corner cut. It does not recognize or remove arbitrary nonconvex
// protrusions, pockets, fillets or curved features. Unbounded remaining supports
// return ErrUnbounded; split coplanar selections return ErrUnsupported.
//
// Before conversion the result provably contains the original convex solid:
// its inequalities are a strict subset of the original inequalities. The added
// region is the intersection of a convex result with the removed halfspace's
// exterior, hence convex and connected when nonempty. Output undergoes the same
// bounded conversion and embedding checks as Apply. OldToNew[face] is -1;
// remaining face provenance identifies the supports extended by the removal.
func RemoveSupport(ctx context.Context, s brep.Solid, face int, o Options) (Result, error) {
	return RemoveSupports(ctx, s, []int{face}, o)
}

// RemoveSupports simultaneously removes selected original supporting facets.
// All facets of a coplanar support must be selected together. Duplicate indices
// are invalid; an empty selection performs validated identity reconstruction.
// Every remaining support is unchanged and the exact result contains the input.
// Unlike single-face removal, the aggregate added region can be disconnected.
// Boundedness and output embedding checks are mandatory; no partial result is
// returned if any selected support or resource limit is invalid.
func RemoveSupports(ctx context.Context, s brep.Solid, faces []int, o Options) (Result, error) {
	if ctx == nil {
		return Result{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Result{}, e
	}
	normalized, e := options(o)
	if e != nil {
		return Result{}, e
	}
	if len(faces) > normalized.MaxFaces {
		return Result{}, ErrLimit
	}
	count := len(s.Faces())
	removed := map[int]bool{}
	for _, i := range faces {
		if i < 0 || i >= count || removed[i] {
			return Result{}, ErrInvalid
		}
		removed[i] = true
	}
	base, e := Apply(ctx, s, nil, normalized)
	if e != nil {
		return Result{}, e
	}
	for _, group := range base.FaceSources {
		selected := 0
		for _, i := range group {
			if removed[i] {
				selected++
			}
		}
		if selected != 0 && selected != len(group) {
			return Result{}, ErrUnsupported
		}
	}
	return applyFiltered(ctx, s, nil, nil, removed, normalized)
}
