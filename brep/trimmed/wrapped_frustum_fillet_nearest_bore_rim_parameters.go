//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// LocateNearestBoreRimRational localizes a proved rational nearest witness on
// an original cap/bore or blind-floor/bore rim. It uses the same aggregate
// extraction/inversion budget and exact original parameter contract as
// LocateNearestFilletRimRational. Quarter vertices retain both incident edges.
// Nonexact nearest witnesses keep only the conservative baseline; no rounded
// or unrecovered algebraic parameter is presented as a rational preimage.
func (q FilletedFrustumMaterial) LocateNearestBoreRimRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumRimParameterOptions) (FilletedFrustumNearestRimParameters, error) {
	return q.locateNearestNativeRimRational(ctx, p, o, true)
}

// LocateNearestBoreRimRational preserves original edge parameters and vertex
// IDs through a proper rational rigid occurrence. Only the exact model witness
// changes frame. Pose arithmetic is limited to4096 bits; failures are atomic.
func (q FilletedFrustumOccurrence) LocateNearestBoreRimRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumRimParameterOptions) (out FilletedFrustumOccurrenceRimParameters, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceRimParameters{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.LocateNearestBoreRimRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	if original.Located {
		out.WorldWitness, e = q.pose.TransformPoint(ctx, original.Baseline.Witness, false, 4096)
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
