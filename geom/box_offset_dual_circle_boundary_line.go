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

// FirstBoxOffsetDualCircleBoundaryHitRational adds exact outward owner normals
// and earliest contact selection to the offset boundary query. Circle normals
// point toward their actual centerY; line parameter units are unchanged.
func FirstBoxOffsetDualCircleBoundaryHitRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, centerY, stopZ *big.Rat, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleFirstHit, error) {
	return firstBoxDualCircleBoundaryHitRational(ctx, min, max, xs, radii, centerY, stopZ, origin, direction, minT, maxT, o)
}

// IntersectBoxOffsetDualCircleBoundaryTransitionsRational preserves exact
// one-sided material germs at isolated offset contacts and continuous span ends.
// Missing finite-range sides remain unknown, never extrapolated.
func IntersectBoxOffsetDualCircleBoundaryTransitionsRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, centerY, stopZ *big.Rat, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleBoundaryTransitions, error) {
	return intersectBoxDualCircleBoundaryTransitionsRational(ctx, min, max, xs, radii, centerY, stopZ, origin, direction, minT, maxT, o)
}
