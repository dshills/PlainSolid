package fillet

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedCylinderBoreRims rolls the inner rims of an existing circular through
// bore between parallel cylinders, including eccentric axes. Zero leaves an
// end sharp. See trimmed.FilletWrappedCylinderBoreRims for exact clearance
// and support requirements.
func WrappedCylinderBoreRims(ctx context.Context, solid trimmed.Solid, lower, upper float64, o geom.SurfacePatchOptions) (
	trimmed.Solid, trimmed.WrappedCylinderBoreFilletHistory, error) {
	return trimmed.FilletWrappedCylinderBoreRims(ctx, solid, lower, upper, o)
}
