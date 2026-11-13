package intersect

import (
	"context"
	"plainsolid/geom"
)

// ParameterTraversalSigns returns signs along this segment's traversal. Ranges
// are derivatives with respect to signed original free distance, not normalized
// display t; their magnitudes are unchanged by traversal reversal.
func (s TubeSegment) ParameterTraversalSigns(ctx context.Context, o geom.SurfacePatchOptions) (geom.SurfaceTubeDerivativeSigns, error) {
	if s.pa == nil {
		return geom.SurfaceTubeDerivativeSigns{}, geom.ErrInvalid
	}
	r, e := s.proof.ParameterDerivativeSigns(ctx, o)
	if s.reversed {
		for i := range r.Sign {
			r.Sign[i] = -r.Sign[i]
			if r.HasRange[i] {
				q := r.Ranges[i]
				r.Ranges[i] = geom.CertifiedScalarRange{Min: -q.Max, Max: -q.Min}
			}
		}
	}
	return r, e
}
