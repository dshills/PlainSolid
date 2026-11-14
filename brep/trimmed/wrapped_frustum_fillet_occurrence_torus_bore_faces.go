package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumOccurrenceTorusBoreFaces keeps both attained candidates in
// world coordinates. Only Original.Tied denotes nearest owners. The original
// retained proof, squared enclosure and exact comparison remain unchanged.
type FilletedFrustumOccurrenceTorusBoreFaces struct {
	Original    FilletedFrustumTorusBoreFaces
	WorldToroid *FilletedFrustumWorldToroidWitness
	WorldBore   *FilletedFrustumWorldBoreWitness
}

// ClassifyTorusBoreFacesRational performs one inverse placement and one source
// query, then transforms its algebraic coefficients without reevaluation.
// Unresolved has no world witnesses;4096-bit pose errors and budgets are atomic.
func (q FilletedFrustumOccurrence) ClassifyTorusBoreFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumTorusBoreOptions) (out FilletedFrustumOccurrenceTorusBoreFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceTorusBoreFaces{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	source, e := q.source.ClassifyTorusBoreFacesRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = source
	if !source.Complete {
		return out, nil
	}
	torus, e := q.transformFilletedToroidDistance(ctx, FilletedFrustumToroidDistance{Toroid: source.Toroid, Lower: source.Lower, Upper: source.Upper})
	if e != nil {
		return out, e
	}
	bore, e := q.transformFilletedBoreDistance(ctx, FilletedFrustumBoreDistance{Bore: source.Bore, Lower: source.Lower, Upper: source.Upper})
	if e != nil {
		return out, e
	}
	out.WorldToroid = torus.WorldToroid
	out.WorldBore = bore.WorldBore
	return out, nil
}
