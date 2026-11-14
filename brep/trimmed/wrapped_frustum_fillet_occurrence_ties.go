package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumOccurrenceNearestTies preserves the original proof in its
// source frame and provides world witnesses for every proved tied face.
// Excluded/Unresolved retain original native IDs. Complete classifies all
// faces, not all minimizer points. Exact proper rigid placement preserves
// squared distances and cannot turn an unresolved face into a proved tie.
type FilletedFrustumOccurrenceNearestTies struct {
	Original             FilletedFrustumNearestTies
	Tied                 []FilletedFrustumTiedFace
	Excluded, Unresolved []int
	Lower, Upper         *big.Rat
	Exact, Complete      bool
}

// ClassifyNearestFacesRational performs one inverse transform and ONE source
// tie query, then transforms its attained witnesses. It never combines
// candidate ledgers from different sources or placements. Pose arithmetic
// is limited to4096 bits and every failure returns an empty result.
func (q FilletedFrustumOccurrence) ClassifyNearestFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumTieOptions) (out FilletedFrustumOccurrenceNearestTies, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceNearestTies{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.ClassifyNearestFacesRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	out.Lower = new(big.Rat).Set(original.Baseline.Lower)
	out.Upper = new(big.Rat).Set(original.Baseline.Upper)
	out.Exact = original.Exact
	out.Complete = original.Complete
	out.Excluded = append([]int(nil), original.Excluded...)
	out.Unresolved = append([]int(nil), original.Unresolved...)
	for _, tie := range original.Tied {
		world, e := q.pose.TransformPoint(ctx, tie.Witness, false, 4096)
		if e != nil {
			return out, e
		}
		out.Tied = append(out.Tied, FilletedFrustumTiedFace{Face: tie.Face, Witness: world})
	}
	return out, nil
}
