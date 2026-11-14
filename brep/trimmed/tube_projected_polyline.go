package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
)

func validTubePolylineProjection(axes [2]int) bool {
	return axes[0] >= 0 && axes[0] < 3 && axes[1] >= 0 && axes[1] < 3 && axes[0] != axes[1]
}

// Only exact point isolation is permuted. Original free-coordinate intervals,
// outward XYZ branch boxes and the independent box-to-chord proof are unchanged.
type projectedVerifiedPolylineGraph struct {
	geom.VerifiedSurfaceTube
	axes [2]int
}

func (p projectedVerifiedPolylineGraph) EvaluateRationalGraph(ctx context.Context, q *big.Rat, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error) {
	return p.VerifiedSurfaceTube.EvaluateRationalProjectedGraph(ctx, q, p.axes, o)
}

type projectedSegmentPolylineGraph struct {
	segmentGraphAdapter
	axes [2]int
}

func (p projectedSegmentPolylineGraph) EvaluateRationalGraph(ctx context.Context, q *big.Rat, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error) {
	return p.segment.EvaluateRationalProjectedGraph(ctx, q, p.axes, o)
}

// A refinement limit does not invalidate an already established whole-branch
// enclosure. No other errors are absorbed, and the caller still charges all
// exact work and proves the requested chord bound from this returned box.
func projectedPolylineEnclosure(r geom.SurfaceTubeIntervalEnclosure, e error, limit int) (geom.SurfaceTubeIntervalEnclosure, error) {
	if errors.Is(e, geom.ErrConvergence) && r.HasBounds && r.Bounds.Valid() && r.ExactOperations < limit {
		return r, nil
	}
	return r, e
}
func (p projectedVerifiedPolylineGraph) EncloseInterval(ctx context.Context, a, b float64, o geom.SurfaceTubeIntervalOptions) (geom.SurfaceTubeIntervalEnclosure, error) {
	r, e := p.VerifiedSurfaceTube.EncloseInterval(ctx, a, b, o)
	return projectedPolylineEnclosure(r, e, o.MaxOperations)
}
func (p projectedSegmentPolylineGraph) EncloseInterval(ctx context.Context, a, b float64, o geom.SurfaceTubeIntervalOptions) (geom.SurfaceTubeIntervalEnclosure, error) {
	r, e := p.segment.EncloseFreeInterval(ctx, a, b, o)
	return projectedPolylineEnclosure(r, e, o.MaxOperations)
}
