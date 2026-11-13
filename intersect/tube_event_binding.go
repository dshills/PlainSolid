package intersect

import (
	"context"
	"plainsolid/geom"
)

// BindAffineGraphEvent independently binds a model event to this segment's
// immutable branch. The free root uses original chart units, regardless of
// segment traversal reversal. It does not treat a rounded display point as proof.
func (s TubeSegment) BindAffineGraphEvent(ctx context.Context, event *geom.ExactComposedCurveEvent, o geom.TubeAlgebraicGraphEndpointOptions) (*geom.TubeGraphEventBinding, error) {
	if s.pa == nil {
		return nil, geom.ErrInvalid
	}
	return s.proof.BindAffineGraphEvent(ctx, event, o)
}
