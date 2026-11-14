package fillet

import (
	"context"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// BlindBoreWrappedRoundedCylinder cuts one axial circular blind bore from the
// selected original cap. Depth is strictly positive and its exact floor must
// lie strictly inside the straight outer cylindrical band's axial interval.
// This excludes through cuts, floors at blend shoulders, and cuts whose floor
// lies in a rounded end region. The bore disk must also lie strictly inside
// BOTH original flat cap disks, including the narrower opposite cap.
//
// Original entities/supports keep their IDs. The donor's actual inward wall
// and floor roles supply the generated topology; the floor's physical normal
// points into the removed cavity (+Z for upper entry, -Z for lower entry).
// FloorFace/FloorZ identify its new support, with no original face ancestor.
// BoreRims are in lower/upper axial order, including the floor rim. All exact
// chart, trim, model-edge and per-call budget gates of the through API apply.
func BlindBoreWrappedRoundedCylinder(ctx context.Context, source trimmed.Solid, center geom.Vec2, radius, depth float64, fromUpper bool, o geom.SurfacePatchOptions) (WrappedRoundedCylinderBore, error) {
	return boreWrappedRoundedCylinder(ctx, source, center, radius, depth, fromUpper, true, o)
}
