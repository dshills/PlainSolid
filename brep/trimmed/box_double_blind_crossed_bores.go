package trimmed

import (
	"context"
	"plainsolid/geom"
)

// DoubleBlindCrossedBoreZFloorFace and DoubleBlindCrossedBoreXFloorFace are
// stable floor indices; original rectangular stock faces remain0..5.
const (
	DoubleBlindCrossedBoreZFloorFace = 6
	DoubleBlindCrossedBoreXFloorFace = 15
)

// BoxDoubleBlindCrossedBores removes two equal-radius perpendicular finite
// cylinders. Each depth is measured inward from its selected stock cap and
// must extend strictly beyond the complete mutual intersection.
func BoxDoubleBlindCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, xDepth, zDepth float64, xOppositeCap, zOppositeCap bool, tolerance float64) (BoxCrossedBoresResult, error) {
	return boxFiniteCrossedPair(ctx, frame, dimensions, crossing, radius, zDepth, zOppositeCap, true, xDepth, xOppositeCap, true, tolerance)
}
