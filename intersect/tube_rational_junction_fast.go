package intersect

import (
	"context"
	"plainsolid/geom"
)

// verifiedSingletonTubeJunction is a bounded optional proof stage. It does not
// trust public evaluation enclosures: singleton model bounds only enable a common
// original parameter tuple, independently checked on BOTH immutable tubes.
// Nonsingleton model bounds do no work and retain the existing general junction path.
// Its arithmetic limit is per proof, as for the existing junction stages.
func verifiedSingletonTubeJunction(ctx context.Context, left, right geom.VerifiedSurfaceTube, leftEnd, rightEnd int, x, y geom.SurfaceTubeEvaluation, o geom.SurfaceTubeJunctionOptions) (bool, int, error) {
	if ctx == nil || leftEnd < 0 || leftEnd > 1 || rightEnd < 0 || rightEnd > 1 {
		return false, 0, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return false, 0, e
	}
	if !x.Enclosed || !y.Enclosed || x.Bounds.Min != x.Bounds.Max || y.Bounds.Min != y.Bounds.Max || x.Bounds.Min != y.Bounds.Min {
		return false, 0, nil
	}
	hint := tubeJunctionHint(x, y)
	for _, q := range hint {
		if q == nil {
			return false, 0, nil
		}
	}
	limit := o.MaxOperations
	if limit == 0 {
		limit = 1000000
	}
	proof, e := geom.CompareVerifiedTubeEndpoints(ctx, left, right, leftEnd == 1, rightEnd == 1, hint, hint, geom.TubeEndpointOptions{MaxOperations: limit, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return false, proof.ExactOperations, e
	}
	// Both certificates proved this exact original tuple, not just coincident XYZ
	// on possibly distinct occurrences of a periodic or self-overlapping support.
	for i := range hint {
		if proof.Left.Parameters[i] == nil || proof.Right.Parameters[i] == nil || proof.Left.Parameters[i].Cmp(hint[i]) != 0 || proof.Right.Parameters[i].Cmp(hint[i]) != 0 {
			return false, proof.ExactOperations, geom.ErrInvalid
		}
	}
	return proof.Complete && proof.Equal, proof.ExactOperations, nil
}
