//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumWorldBoreWitness is Center+RadialNumerator/sqrt(Radicand).
// Axis is the exact transformed cylinder axis, orthogonal to RadialNumerator.
// The positive root interval is an enclosure, not a rounded substitute.
type FilletedFrustumWorldBoreWitness struct {
	Center, RadialNumerator, Axis [3]*big.Rat
	Radicand                      *big.Rat
	Root                          [2]*big.Rat
	Face                          int
}

type FilletedFrustumOccurrenceBoreDistance struct {
	Original                 FilletedFrustumBoreDistance
	Lower, Upper             *big.Rat
	WorldBore                *FilletedFrustumWorldBoreWitness
	Fallback                 FilletedFrustumOccurrenceConeDistance
	NearestProven, Converged bool
}

// BoundBoundaryBoreDistanceRational uses one inverse query and symbolically
// places its selected witness. Proper rigid invariance preserves all source
// owners, squared bounds and proof status. Errors and 4096-bit pose failures
// return no partial proof. Existing explicit cone/torus methods are unchanged.
func (q FilletedFrustumOccurrence) BoundBoundaryBoreDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumBoreDistanceOptions) (out FilletedFrustumOccurrenceBoreDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceBoreDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundBoundaryBoreDistanceRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	return q.transformFilletedBoreDistance(ctx, original)
}

// transformFilletedBoreDistance reuses the completed source query.
func (q FilletedFrustumOccurrence) transformFilletedBoreDistance(ctx context.Context, original FilletedFrustumBoreDistance) (out FilletedFrustumOccurrenceBoreDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceBoreDistance{}
		}
	}()
	var e error
	out.Original = original
	out.Lower = new(big.Rat).Set(original.Lower)
	out.Upper = new(big.Rat).Set(original.Upper)
	out.NearestProven = original.NearestProven
	out.Converged = original.Converged
	if original.Bore == nil {
		out.Fallback, e = q.transformFilletedConeDistance(ctx, original.Baseline)
		if e != nil {
			return out, e
		}
		return out, nil
	}
	w := original.Bore
	center, e := q.pose.TransformPoint(ctx, w.Center, false, 4096)
	if e != nil {
		return out, e
	}
	radial, e := q.pose.TransformVector(ctx, w.Numerator, false, 4096)
	if e != nil {
		return out, e
	}
	axis, e := q.pose.TransformVector(ctx, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, false, 4096)
	if e != nil {
		return out, e
	}
	out.WorldBore = &FilletedFrustumWorldBoreWitness{Center: center, RadialNumerator: radial, Axis: axis, Radicand: new(big.Rat).Set(w.Radicand), Root: [2]*big.Rat{new(big.Rat).Set(w.Root[0]), new(big.Rat).Set(w.Root[1])}, Face: w.Face}
	return out, nil
}
