package geom

import (
	"context"
	"math/big"
)

// IntersectBoxOffsetDualCircleBoundaryRational is the exact finite-line boundary
// query for a box minus two transverse circular cutters with common centerY at
// or below the opening Y side. Both circles intersect that side strictly and
// clear the other three side planes. The center ordinate is independent of the
// stock bound: all radial predicates use the original circle equations.
// This standalone query does not certify the connectivity of a native cut.
// Ordered isolated roots, boundary spans, masks, budgets and raw line units
// have the same contract as IntersectBoxDualCircleBoundaryRational.
func IntersectBoxOffsetDualCircleBoundaryRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, centerY, stopZ *big.Rat, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleBoundaryLine, error) {
	return intersectBoxDualCircleBoundaryRational(ctx, min, max, xs, radii, centerY, stopZ, origin, direction, minT, maxT, o)
}
