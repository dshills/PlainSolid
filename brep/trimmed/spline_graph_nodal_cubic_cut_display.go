package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Tessellate displays the independently indexed components and proves that
// their only contact is the declared original node. The triangle budget
// covers the complete assembly.
func (r *NodalCubicCut) Tessellate(ctx context.Context, o parametric.TessellationOptions) ([]mesh.Mesh, error) {
	if r == nil || r.sources[0] == nil {
		return nil, geom.ErrInvalid
	}
	o, err := normalizeTrimmedTessellationOptions(ctx, o)
	if err != nil {
		return nil, err
	}
	limit := min(o.MaxTriangles, collision.MaxTriangles)
	o.MaxTriangles = limit
	parts := make([]mesh.Mesh, len(r.components))
	for i, component := range r.components {
		if o.MaxTriangles <= 0 {
			return nil, parametric.ErrBudget
		}
		part, err := component.Tessellate(ctx, o)
		if err != nil {
			return nil, fmt.Errorf("trimmed: nodal cubic Cut display component %d: %w", i, err)
		}
		parts[i] = part
		o.MaxTriangles -= len(part.Triangles)
	}
	ids := r.contact.Vertices
	contacts := []collision.AssemblyVertexContact{{
		A: collision.AssemblyVertex{Solid: 0, Vertex: ids[0]},
		B: collision.AssemblyVertex{Solid: 1, Vertex: ids[1]},
	}}
	validation := collision.VertexContactAssemblyOptions{Validation: collision.LocationOptions{MaxTriangles: limit}}
	if _, err := collision.ValidateMeshVertexContactAssembly(ctx, parts, contacts, validation); err != nil {
		return nil, fmt.Errorf("trimmed: nodal cubic Cut display contact: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return parts, nil
}
