package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// ClipRationalLine preserves original rational line coordinates/parameters and
// exact source owner masks on the immutable native sphere-pair binding. Native
// topology restrictions remain independent from the analytic material query.
func (s SpherePairSet) ClipRationalLine(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o geom.ToroidLineOptions) (geom.SpherePairMaterialLine, error) {
	if !s.valid {
		return geom.SpherePairMaterialLine{}, geom.ErrInvalid
	}
	return s.query.ClipRationalLine(ctx, origin, direction, minT, maxT, o)
}
