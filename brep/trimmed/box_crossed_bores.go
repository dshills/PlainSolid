package trimmed

import (
	"context"
	"plainsolid/geom"
)

// BoxCrossedBoresResult retains two equal-radius orthogonal through-bores.
type BoxCrossedBoresResult struct {
	Solid                           Solid
	RemovedVolume, SurfaceAreaDelta float64
}

// BoxCrossedBores removes cylinders along local X and Z through crossing from
// [0,dimensions]. Signed cardinal frames and strictly interior crossings are
// supported. Both complete Steinmetz intersection lobes must clear every cap.
func BoxCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, tolerance float64) (BoxCrossedBoresResult, error) {
	return boxFiniteCrossedBores(ctx, frame, dimensions, crossing, radius, dimensions.Z, false, false, tolerance)
}
