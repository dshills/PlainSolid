package geom

import (
	"context"
	"math/big"
)

// MassMomentsRational integrates each complete original chart exactly once.
// The three independently separated nonadjacent pairs and zero-volume
// adjacent walls permit direct composition of extensive moments.
func (c *VerifiedFourChartNormalOffsetSweepChain) MassMomentsRational(ctx context.Context,
	o NormalOffsetSweepMomentOptions) (NormalOffsetSweepMassMoments, error) {
	if c == nil || c.pairs[0] == nil {
		return NormalOffsetSweepMassMoments{}, ErrInvalid
	}
	return massMomentsOfVerifiedSweeps(ctx, c.sweeps[:], o)
}

// BoundaryAreaRational measures only the retained original outer faces by
// the supported affine and circular formulas under one shared budget.
func (c *VerifiedFourChartNormalOffsetSweepChain) BoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaOptions) (NormalOffsetSweepJoinedBoundaryArea, error) {
	if c == nil || c.pairs[0] == nil {
		return NormalOffsetSweepJoinedBoundaryArea{}, ErrInvalid
	}
	return measureJoinedSweepBoundaryArea(ctx, c.sweeps[:], c.Boundary(), o)
}

// EncloseBoundaryAreaRational uses directed original derivative bounds on
// the full retained boundary, with one shared queue across all owners.
func (c *VerifiedFourChartNormalOffsetSweepChain) EncloseBoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaEnclosureOptions) (NormalOffsetSweepJoinedBoundaryArea, error) {
	if c == nil || c.pairs[0] == nil {
		return NormalOffsetSweepJoinedBoundaryArea{}, ErrInvalid
	}
	return encloseDirectedSweepBoundaryArea(ctx, c.sweeps[:], c.Boundary(), o)
}

// BoundPointBoundaryDistanceRational encloses the squared distance to the
// retained boundary with an original chart and face witness. None of the
// three joining walls contributes candidates.
func (c *VerifiedFourChartNormalOffsetSweepChain) BoundPointBoundaryDistanceRational(ctx context.Context,
	point [3]*big.Rat, o NormalOffsetSweepDistanceOptions) (NormalOffsetSweepDistance, error) {
	if c == nil || c.pairs[0] == nil {
		return NormalOffsetSweepDistance{}, ErrInvalid
	}
	return boundPointSweepOuterDistance(ctx, point, o, c.sweeps[:], c.Boundary())
}
