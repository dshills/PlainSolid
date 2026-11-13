package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// EmbeddedDisplayOptions keeps sampling and exact-contact budgets explicit.
// Tessellation.Deflection is the existing sampled analytic-mesh target, not a
// new continuous Hausdorff guarantee. Collision limits never yield a partial
// accepted display. Meshes exceeding collision.MaxTriangles are rejected.
type EmbeddedDisplayOptions struct {
	Tessellation parametric.TessellationOptions
	Collision    collision.Options
}

// EmbeddedDisplay proves a closed manifold represented triangle boundary,
// exact positive total signed volume and absence of non-incidence contacts.
// It does not prove the source curved B-rep's continuous embedding or distance
// to the triangles; these are deliberately separate from display validation.
type EmbeddedDisplay struct {
	Mesh         mesh.Mesh
	Collision    collision.Report
	SignedVolume *big.Rat
}

func (s Solid) TessellateEmbeddedDisplay(ctx context.Context, o EmbeddedDisplayOptions) (EmbeddedDisplay, error) {
	if ctx == nil {
		return EmbeddedDisplay{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return EmbeddedDisplay{}, e
	}
	if o.Tessellation.MaxTriangles == 0 {
		o.Tessellation.MaxTriangles = collision.MaxTriangles
	}
	if o.Tessellation.MaxTriangles < 0 || o.Tessellation.MaxTriangles > collision.MaxTriangles {
		return EmbeddedDisplay{}, parametric.ErrBudget
	}
	m, e := s.Tessellate(ctx, o.Tessellation)
	if e != nil {
		return EmbeddedDisplay{}, e
	}
	volume, report, e := checkClosedPeriodicMesh(ctx, m, o.Collision)
	if e != nil {
		return EmbeddedDisplay{}, e
	}
	return EmbeddedDisplay{m, report, volume}, nil
}
