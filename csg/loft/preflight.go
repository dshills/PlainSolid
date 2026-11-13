package loft

import (
	"plainsolid/brep/support"
	"plainsolid/collision"
)

// preflightSizes enforces explicitly configured proof-size gates before native
// triangulation. Unset limits stay with their existing proof passes, preserving
// default geometry-error precedence. Counts are exact for accepted two-cap
// n-gon/quad-band topology: V=n*stations, F=2+n*(stations-1), T=2*V-4.
func preflightSizes(vertices, faces int, o Options) error {
	if o.Support.MaxVertices < 0 || o.Support.MaxFaces < 0 {
		return support.ErrInvalid
	}
	if (o.Support.MaxVertices > 0 && vertices > o.Support.MaxVertices) || (o.Support.MaxFaces > 0 && faces > o.Support.MaxFaces) {
		return support.ErrLimit
	}
	if o.Validation.MaxTriangles < 0 || o.Validation.MaxTriangles > collision.MaxTriangles {
		return collision.ErrInvalid
	}
	if o.Validation.MaxTriangles > 0 && 2*vertices-4 > o.Validation.MaxTriangles {
		return collision.ErrLimit
	}
	return nil
}
