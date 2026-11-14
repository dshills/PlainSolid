package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumOccurrenceBoreConeFaces retains the completed local proof and
// both original candidate witnesses in world coordinates. Only Original.Tied
// identifies nearest owners; the other candidate is not promoted to nearest.
// Proper rational rigid invariance preserves the squared distance enclosure.
// Unresolved results contain neither world candidate.
type FilletedFrustumOccurrenceBoreConeFaces struct {
	Original  FilletedFrustumBoreConeFaces
	WorldCone *FilletedFrustumWorldConeWitness
	WorldBore *FilletedFrustumWorldBoreWitness
}

// ClassifyBoreConeFacesRational uses one exact inverse placement, one source
// support query and algebraic coefficient transforms. It never reruns a source
// distance query to place the second candidate. Pose intermediates are bounded
// to4096 bits; source options retain their original budgets. Errors are atomic.
func (q FilletedFrustumOccurrence) ClassifyBoreConeFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumBoreConeOptions) (out FilletedFrustumOccurrenceBoreConeFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceBoreConeFaces{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	source, e := q.source.ClassifyBoreConeFacesRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = source
	if !source.Complete {
		return out, nil
	}
	cone, e := q.transformFilletedConeDistance(ctx, FilletedFrustumConeDistance{Cone: source.Cone, Lower: source.Lower, Upper: source.Upper})
	if e != nil {
		return out, e
	}
	bore, e := q.transformFilletedBoreDistance(ctx, FilletedFrustumBoreDistance{Bore: source.Bore, Lower: source.Lower, Upper: source.Upper})
	if e != nil {
		return out, e
	}
	out.WorldCone = cone.WorldCone
	out.WorldBore = bore.WorldBore
	return out, nil
}
