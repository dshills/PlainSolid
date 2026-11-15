package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Tessellate builds a separate display mesh for each native Cut component.
// It retains native vertex indices and checks sampled deflection and normal
// angles. MaxTriangles bounds the complete display and the contact proof is
// subject to the collision package's exact validation limits. The display
// components meet only at the source-declared pairs in Contacts.
func (r *ClippedPolynomialCut) Tessellate(ctx context.Context, o parametric.TessellationOptions) ([]mesh.Mesh, error) {
	if r == nil || r.native == nil {
		return nil, geom.ErrInvalid
	}
	o, e := normalizeTrimmedTessellationOptions(ctx, o)
	if e != nil {
		return nil, e
	}
	if o.AnalyticBound {
		return nil, parametric.ErrDeflectionUnsupported
	}
	parts := r.native.material.components
	displays := make([]mesh.Mesh, len(parts))
	maximum := min(o.MaxTriangles, collision.MaxTriangles)
	remaining := maximum
	for i, part := range parts {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if remaining == 0 {
			return nil, parametric.ErrBudget
		}
		options := o
		options.MaxTriangles = remaining
		part = part.withTessellationTrimMemo(&tessellationTrimMemo{limit: tessellationTrimMemoLimit})
		display, e := part.tessellateClippedPolynomial(ctx, r.native.axis, options)
		if e != nil {
			return nil, fmt.Errorf("trimmed: clipped Cut display component %d: %w", i, e)
		}
		displays[i] = display
		remaining -= len(display.Triangles)
	}

	declarations := make([]collision.AssemblyVertexContact, 0, len(r.native.material.contacts))
	for _, contact := range r.native.material.contacts {
		declarations = append(declarations, collision.AssemblyVertexContact{
			A: collision.AssemblyVertex{Solid: contact.Components[0], Vertex: contact.Vertices[0]},
			B: collision.AssemblyVertex{Solid: contact.Components[1], Vertex: contact.Vertices[1]},
		})
	}
	_, e = collision.ValidateMeshVertexContactAssembly(ctx, displays, declarations, collision.VertexContactAssemblyOptions{
		Validation: collision.LocationOptions{MaxTriangles: maximum},
	})
	if e != nil {
		return nil, fmt.Errorf("trimmed: clipped Cut display contacts: %w", e)
	}
	return displays, nil
}
