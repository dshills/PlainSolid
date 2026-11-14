package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumWorldToroidWitness is the exact world boundary point
// Base+Numerator/sqrt(Radicand), with the unique positive root enclosed in
// Root. Base is the transformed meridian-circle center for this azimuth,
// not the torus-axis center. Face is an unchanged original native owner.
type FilletedFrustumWorldToroidWitness struct {
	Base, Numerator [3]*big.Rat
	Radicand        *big.Rat
	Root            [2]*big.Rat
	Face            int
}

// FilletedFrustumOccurrenceToroidDistance preserves source squared-distance
// bounds and nearest status under an exact proper rigid pose. WorldToroid is
// populated for a newly selected stationary torus witness; otherwise Fallback
// contains the world rational/rim witness from the same source calculation.
// Original retains source-frame coordinates. No rounded world Solid is made.
type FilletedFrustumOccurrenceToroidDistance struct {
	Original                 FilletedFrustumToroidDistance
	Lower, Upper             *big.Rat
	WorldToroid              *FilletedFrustumWorldToroidWitness
	Fallback                 FilletedFrustumOccurrenceRimDistance
	NearestProven, Converged bool
}

// BoundBoundaryToroidDistanceRational supports the original proved rational
// local azimuth subset under arbitrary proper rational world rotation and
// translation. The source query runs once. Exact vector/point transforms have
// a4096-bit intermediate limit; any failure discards the complete result.
func (q FilletedFrustumOccurrence) BoundBoundaryToroidDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumToroidDistanceOptions) (out FilletedFrustumOccurrenceToroidDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceToroidDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundBoundaryToroidDistanceRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	out.Lower = new(big.Rat).Set(original.Lower)
	out.Upper = new(big.Rat).Set(original.Upper)
	out.NearestProven = original.NearestProven
	out.Converged = original.Converged
	if original.Toroid == nil {
		out.Fallback, e = q.transformFilletedRimDistance(ctx, original.Baseline)
		if e != nil {
			return out, e
		}
		return out, nil
	}
	w := original.Toroid
	base, e := q.pose.TransformPoint(ctx, w.Center, false, 4096)
	if e != nil {
		return out, e
	}
	numerator, e := q.pose.TransformVector(ctx, w.Numerator, false, 4096)
	if e != nil {
		return out, e
	}
	out.WorldToroid = &FilletedFrustumWorldToroidWitness{Base: base, Numerator: numerator, Radicand: new(big.Rat).Set(w.Radicand), Root: [2]*big.Rat{new(big.Rat).Set(w.Root[0]), new(big.Rat).Set(w.Root[1])}, Face: w.Face}
	return out, nil
}
