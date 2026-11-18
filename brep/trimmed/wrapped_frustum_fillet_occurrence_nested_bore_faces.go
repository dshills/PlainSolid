//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumOccurrenceNestedBoreFaces preserves the one source proof,
// including unresolved equality, and transforms both attained candidates only
// when strict owner ordering is complete. Only Original.Tied is nearest.
type FilletedFrustumOccurrenceNestedBoreFaces struct {
	Original    FilletedFrustumNestedBoreFaces
	WorldNested *FilletedFrustumWorldNestedToroidWitness
	WorldBore   *FilletedFrustumWorldBoreWitness
}

// ClassifyNestedTorusBoreFacesRational performs one exact inverse pose and one
// bounded source query, then transforms existing nested-radical coefficients.
// No second baseline or refinement scan is performed. Proper rigid invariance
// preserves squared bounds/owners;4096-bit pose failures are atomic.
func (q FilletedFrustumOccurrence) ClassifyNestedTorusBoreFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedBoreOptions) (out FilletedFrustumOccurrenceNestedBoreFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceNestedBoreFaces{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	source, e := q.source.ClassifyNestedTorusBoreFacesRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = source
	if !source.Complete {
		return out, nil
	}
	nested, e := q.transformFilletedNestedDistance(ctx, FilletedFrustumNestedDistance{Nested: source.Nested, Lower: source.Lower, Upper: source.Upper})
	if e != nil {
		return out, e
	}
	bore, e := q.transformFilletedBoreDistance(ctx, FilletedFrustumBoreDistance{Bore: source.Bore, Lower: source.Lower, Upper: source.Upper})
	if e != nil {
		return out, e
	}
	out.WorldNested = nested.WorldNested
	out.WorldBore = bore.WorldBore
	return out, nil
}
