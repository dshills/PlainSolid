package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Tessellate returns a conforming display whose exact represented volume
// differs from the original positive cap integral by at most two percent.
func (r *AcnodalCubicCut) Tessellate(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	return r.TessellateWithVolumeTolerance(ctx, o, .02)
}

// TessellateWithVolumeTolerance refines the original trimmed boundary until
// the exact volume and collision checks pass. Deflection and angle keep the
// semantics of Solid.Tessellate. The caller's triangle and subdivision caps
// apply to every candidate, with at most eight attempts.
func (r *AcnodalCubicCut) TessellateWithVolumeTolerance(ctx context.Context, o parametric.TessellationOptions,
	relative float64) (mesh.Mesh, error) {
	var zero mesh.Mesh
	if r == nil || r.volume == nil || r.volume.Sign() <= 0 || !finite(relative) || relative <= 0 {
		return zero, geom.ErrInvalid
	}
	o, err := normalizeTrimmedTessellationOptions(ctx, o)
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	o.MaxTriangles = min(o.MaxTriangles, collision.MaxTriangles)
	// The exact mesh triple product sums to six times the signed volume.
	want := new(big.Rat).Mul(r.volume, big.NewRat(6, 1))
	tolerance := new(big.Rat).Mul(want, ratInset(relative))
	for attempt := 0; attempt < 8 && o.Deflection > 0; attempt++ {
		candidate, err := r.solid.Tessellate(ctx, o)
		if err != nil {
			return zero, err
		}
		if len(candidate.Triangles) == 0 || len(candidate.Vertices) == 0 {
			return zero, geom.ErrConvergence
		}
		if clippedCutDisplayMatchesVolume(ctx, candidate, want, tolerance) {
			locator, err := collision.NewMeshPointLocator(ctx, candidate, nil, collision.LocationOptions{MaxTriangles: o.MaxTriangles})
			if errors.Is(err, collision.ErrLimit) {
				return zero, parametric.ErrBudget
			}
			if err != nil {
				return zero, err
			}
			if len(locator.Shells()) != 1 {
				return zero, geom.ErrConvergence
			}
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			return candidate, nil
		}
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		o.Deflection *= .5
		o.Angle *= .5
	}
	return zero, parametric.ErrBudget
}
