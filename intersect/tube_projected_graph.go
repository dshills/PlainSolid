package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// EvaluateRationalProjectedGraph uses original free-coordinate units and model
// axes (0=X,1=Y,2=Z), preserving original XYZ event data after exact permutation.
func (s TubeSegment) EvaluateRationalProjectedGraph(ctx context.Context, free *big.Rat, axes [2]int, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error) {
	if s.pa == nil {
		return geom.TubeAlgebraicGraphPointCertificate{}, geom.ErrInvalid
	}
	return s.proof.EvaluateRationalProjectedGraph(ctx, free, axes, o)
}
func (s TubeSegment) BindProjectedGraphEvent(ctx context.Context, event *geom.ExactComposedCurveEvent, axes [2]int, o geom.TubeAlgebraicGraphEndpointOptions) (*geom.TubeGraphEventBinding, error) {
	if s.pa == nil {
		return nil, geom.ErrInvalid
	}
	return s.proof.BindProjectedGraphEvent(ctx, event, axes, o)
}

// DiscoverAlgebraicProjectedGraphEndpoint uses normalized endpoint 0 or 1,
// respecting traversal reversal while preserving exact original chart units.
func (s TubeSegment) DiscoverAlgebraicProjectedGraphEndpoint(ctx context.Context, end int, axes [2]int, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphEndpointCertificate, error) {
	if s.pa == nil || end < 0 || end > 1 {
		return geom.TubeAlgebraicGraphEndpointCertificate{}, geom.ErrInvalid
	}
	if s.reversed {
		end ^= 1
	}
	return s.proof.DiscoverAlgebraicProjectedGraphEndpoint(ctx, end == 1, axes, o)
}
