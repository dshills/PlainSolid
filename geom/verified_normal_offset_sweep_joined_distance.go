package geom

import (
	"context"
	"math/big"
)

// BoundPointBoundaryDistanceRational encloses squared distance to the retained
// analytic outer boundary of two joined charts. The removed common wall
// contributes no candidates. One exact work and cell budget covers both
// owners. A coarse lower endpoint requires a full retained face cover.
func (p *VerifiedNormalOffsetSweepPair) BoundPointBoundaryDistanceRational(ctx context.Context, point [3]*big.Rat,
	o NormalOffsetSweepDistanceOptions) (NormalOffsetSweepDistance, error) {
	if p == nil || p.coordinate == nil {
		return NormalOffsetSweepDistance{}, ErrInvalid
	}
	return boundPointSweepOuterDistance(ctx, point, o, p.sweeps[:], p.Boundary())
}

// BoundPointBoundaryDistanceRational encloses squared distance to the fourteen
// retained analytic outer faces of three joined original charts. Neither
// internal wall contributes candidates. A shared budget covers all owners.
// A coarse lower endpoint requires a full retained face cover.
func (c *VerifiedNormalOffsetSweepChain) BoundPointBoundaryDistanceRational(ctx context.Context, point [3]*big.Rat,
	o NormalOffsetSweepDistanceOptions) (NormalOffsetSweepDistance, error) {
	if c == nil || c.pairs[0] == nil {
		return NormalOffsetSweepDistance{}, ErrInvalid
	}
	return boundPointSweepOuterDistance(ctx, point, o, c.sweeps[:], c.Boundary())
}
