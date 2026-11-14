package trimmed

import (
	"context"
	"plainsolid/geom"
)

const BlindTripleCrossedBoreFloorFace = 6

// BoxBlindTripleCrossedBores retains X/Y through bores and one finite Z bore
// of the same radius. Its floor must lie beyond the entire triple overlap.
// Original stock faces remain0..5; floor6 precedes Z7..22,X23..38,Y39..54.
func BoxBlindTripleCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, depth float64, oppositeCap bool, tolerance float64) (BoxCrossedBoresResult, error) {
	if depth == 0 {
		return BoxCrossedBoresResult{}, geom.ErrDomain
	}
	return BoxFiniteTripleCrossedBores(ctx, frame, dimensions, crossing, radius, [3]float64{2: depth}, [3]bool{2: oppositeCap}, tolerance)
}
