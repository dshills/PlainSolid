package intersect

import (
	"context"
	"plainsolid/geom"
)

// EncloseFreeInterval encloses the entire proved model branch over represented
// ORIGINAL free-coordinate limits. Segment traversal orientation is irrelevant.
// HasBounds retains the geometry proof even if a refinement limit is reported.
func (s TubeSegment) EncloseFreeInterval(ctx context.Context, start, end float64, o geom.SurfaceTubeIntervalOptions) (geom.SurfaceTubeIntervalEnclosure, error) {
	if s.pa == nil {
		return geom.SurfaceTubeIntervalEnclosure{}, geom.ErrInvalid
	}
	return s.proof.EncloseInterval(ctx, start, end, o)
}
