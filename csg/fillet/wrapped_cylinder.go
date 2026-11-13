package fillet

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedCylinderRims rounds selected end rims of a canonical circular
// wrapped cylinder with exact toroidal blends. Use zero radius to leave
// an end sharp. Both end radii and the retained wall must satisfy the
// bounds documented by trimmed.FilletWrappedCylinderRims.
func WrappedCylinderRims(ctx context.Context, solid trimmed.Solid, lower, upper float64, o geom.SurfacePatchOptions) (
	trimmed.Solid, trimmed.WrappedCylinderRimFilletHistory, error) {
	return trimmed.FilletWrappedCylinderRims(ctx, solid, lower, upper, o)
}
