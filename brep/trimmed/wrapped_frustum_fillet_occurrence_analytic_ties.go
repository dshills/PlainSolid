package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumOccurrenceAnalyticNearestFaces retains original-frame proof
// and a world-space symbolic witness in WorldProof's selected fallback chain.
// Kind selects its representation. All owner IDs and separation statuses are
// unchanged by the proper rigid pose. Complete concerns original faces only,
// never uniqueness of the point or resolution of unsupported equal radicals.
type FilletedFrustumOccurrenceAnalyticNearestFaces struct {
	Original                   FilletedFrustumAnalyticNearestFaces
	WorldProof                 FilletedFrustumOccurrenceBoreDistance
	Kind                       FilletedFrustumClosestKind
	Tied, Excluded, Unresolved []int
	Complete                   bool
}

// ClassifyAnalyticNearestFacesRational performs ONE original classification
// and transforms its existing witness, without repeating geometry queries or
// comparison work. Rational proper orthonormality preserves squared intervals
// and strict separation. Pose arithmetic has4096-bit limits; errors are atomic.
func (q FilletedFrustumOccurrence) ClassifyAnalyticNearestFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedDistanceOptions) (out FilletedFrustumOccurrenceAnalyticNearestFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceAnalyticNearestFaces{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.ClassifyAnalyticNearestFacesRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	if original.Proof.BoreProof == nil {
		return out, geom.ErrConvergence
	}
	world, e := q.transformFilletedBoreDistance(ctx, *original.Proof.BoreProof)
	if e != nil {
		return out, e
	}
	out.Original = original
	out.WorldProof = world
	out.Kind = original.Proof.Kind
	out.Complete = original.Complete
	out.Tied = append([]int(nil), original.Tied...)
	out.Excluded = append([]int(nil), original.Excluded...)
	out.Unresolved = append([]int(nil), original.Unresolved...)
	return out, nil
}
