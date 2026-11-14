package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// NearestBoundaryRational resolves the distance in the original vertical
// stock and preserves the exact distance and curve parameter under the
// rational rigid pose. A direct point witness is returned in world space;
// curve witnesses still identify the original cap curve and face owner.
func (o AlgebraicGraphStockOccurrence) NearestBoundaryRational(ctx context.Context, p [3]*big.Rat,
	limits geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalDistance, error) {
	var zero AlgebraicGraphVerticalDistance
	q, e := o.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return zero, e
	}
	d, e := o.model.NearestBoundaryRational(ctx, q, limits)
	if e != nil {
		return zero, e
	}
	if d.Point[0] != nil {
		d.Point, e = o.pose.TransformPoint(ctx, d.Point, false, 4096)
		if e != nil {
			return zero, e
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return d, nil
}
