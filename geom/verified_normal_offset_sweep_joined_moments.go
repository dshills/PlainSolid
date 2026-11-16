package geom

import "context"

// MassMomentsRational encloses the unit-density material moments of the
// original two joined charts. Their common wall has zero volume. It sums
// both charts' extensive moments before deriving the centroid and inertia.
// Exact work and visited-cell budgets are shared across the pair.
// A partial numerical result covers both charts and requires a proved
// positive total volume.
func (p *VerifiedNormalOffsetSweepPair) MassMomentsRational(ctx context.Context,
	o NormalOffsetSweepMomentOptions) (NormalOffsetSweepMassMoments, error) {
	if p == nil || p.coordinate == nil {
		return NormalOffsetSweepMassMoments{}, ErrInvalid
	}
	return massMomentsOfVerifiedSweeps(ctx, p.sweeps[:], o)
}

// MassMomentsRational encloses the unit-density material moments of the
// three original charts, counting the common middle chart once. The
// certified wall joins and end-chart gap give disjoint open material.
// Exact work and visited-cell budgets are shared across the chain.
// A partial numerical result covers all three charts and requires a
// proved positive total volume.
func (c *VerifiedNormalOffsetSweepChain) MassMomentsRational(ctx context.Context,
	o NormalOffsetSweepMomentOptions) (NormalOffsetSweepMassMoments, error) {
	if c == nil || c.pairs[0] == nil {
		return NormalOffsetSweepMassMoments{}, ErrInvalid
	}
	return massMomentsOfVerifiedSweeps(ctx, c.sweeps[:], o)
}
