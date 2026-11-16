package geom

import (
	"context"
	"math/big"
)

// MassMomentsRational integrates each original chart once under the shared
// exact work and cell limits. The complete nonadjacent separations and
// zero-volume adjacent walls authorize direct composition of the moments.
func (c *VerifiedNormalOffsetSweepOpenChain) MassMomentsRational(ctx context.Context,
	o NormalOffsetSweepMomentOptions) (NormalOffsetSweepMassMoments, error) {
	if c == nil || len(c.joins) == 0 {
		return NormalOffsetSweepMassMoments{}, ErrInvalid
	}
	return massMomentsOfVerifiedSweeps(ctx, c.sweeps, o)
}

// BoundaryAreaRational measures the retained analytic faces by the
// supported affine and circular formulas under a shared budget.
func (c *VerifiedNormalOffsetSweepOpenChain) BoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaOptions) (NormalOffsetSweepJoinedBoundaryArea, error) {
	if c == nil || len(c.joins) == 0 {
		return NormalOffsetSweepJoinedBoundaryArea{}, ErrInvalid
	}
	return measureJoinedSweepBoundaryArea(ctx, c.sweeps, c.Boundary(), o)
}

// EncloseBoundaryAreaRational integrates directed original derivatives
// with one shared queue over the complete retained owner list.
func (c *VerifiedNormalOffsetSweepOpenChain) EncloseBoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaEnclosureOptions) (NormalOffsetSweepJoinedBoundaryArea, error) {
	if c == nil || len(c.joins) == 0 {
		return NormalOffsetSweepJoinedBoundaryArea{}, ErrInvalid
	}
	return encloseDirectedSweepBoundaryArea(ctx, c.sweeps, c.Boundary(), o)
}

// BoundPointBoundaryDistanceRational compares only retained original
// analytic faces and reports the original owner and station of its witness.
func (c *VerifiedNormalOffsetSweepOpenChain) BoundPointBoundaryDistanceRational(ctx context.Context,
	point [3]*big.Rat, o NormalOffsetSweepDistanceOptions) (NormalOffsetSweepDistance, error) {
	if c == nil || len(c.joins) == 0 {
		return NormalOffsetSweepDistance{}, ErrInvalid
	}
	return boundPointSweepOuterDistance(ctx, point, o, c.sweeps, c.Boundary())
}
