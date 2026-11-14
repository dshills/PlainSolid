package trimmed

import (
	"context"
	"plainsolid/geom"
)

// BoxFarTangentCrossedBores stops a lower-entry equal-radius Z bore exactly
// at the far tangent plane of its X through bore. The closed manifold retains
// a full planar floor at face6; its rim meets the crossing at two vertices.
func BoxFarTangentCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, tolerance float64) (BoxCrossedBoresResult, error) {
	return boxFiniteCrossedPairLimit(ctx, frame, dimensions, crossing, radius, crossing.Z+radius, false, true, dimensions.X, false, false, true, tolerance)
}
