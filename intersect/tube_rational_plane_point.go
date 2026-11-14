package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// EvaluateRationalPlane preserves original free-parameter units and certifies
// rational paired coordinates against a planar, potentially nonlinear opposite
// support. It does not substitute rounded point proposals for tube identity.
func (s TubeSegment) EvaluateRationalPlane(ctx context.Context, q *big.Rat, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error) {
	if s.a == nil || s.b == nil {
		return geom.TubeAlgebraicGraphPointCertificate{}, geom.ErrInvalid
	}
	return s.proof.EvaluateRationalPlane(ctx, q, o)
}
