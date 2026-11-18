//go:build research

package trimmed

import (
	"context"

	"plainsolid/geom"
	"plainsolid/section"
)

// SectionAffineSplineShell returns the complete planar section of a solid
// with affine spline supports and straight UV trims. Source face and edge
// IDs refer to the original spline boundary. The ancestry's rational corner
// coordinates are exact; its simplified display loops use float coordinates.
// Contacts at source vertices follow section.PlanarWithAncestry's strict gate.
func SectionAffineSplineShell(ctx context.Context, s Solid, plane geom.Plane,
	o AffineSplineShellBooleanOptions) (section.Ancestry, error) {
	p, e := PlanarBoundaryFromAffineSplines(ctx, s, o)
	if e != nil {
		return section.Ancestry{}, e
	}
	return section.PlanarWithAncestry(ctx, p, plane, o.Planar.MaxInputTriangles)
}
