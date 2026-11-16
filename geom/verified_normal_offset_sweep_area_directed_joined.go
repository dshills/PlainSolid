package geom

import "context"

// EncloseBoundaryAreaRational bounds the retained original faces of the
// verified pair with directed analytic integration. Both original joining
// walls are excluded before the first area cell is evaluated.
func (p *VerifiedNormalOffsetSweepPair) EncloseBoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaEnclosureOptions) (NormalOffsetSweepJoinedBoundaryArea, error) {
	if p == nil || p.coordinate == nil {
		return NormalOffsetSweepJoinedBoundaryArea{}, ErrInvalid
	}
	return encloseDirectedSweepBoundaryArea(ctx, p.sweeps[:], p.Boundary(), o)
}

// EncloseBoundaryAreaRational bounds the complete retained outer boundary
// of the verified chain. All original faces share one adaptive priority
// queue and exact work budget. Only a full retained-face cover can publish
// a global total, possibly coarser than requested if refinement is limited.
func (c *VerifiedNormalOffsetSweepChain) EncloseBoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaEnclosureOptions) (NormalOffsetSweepJoinedBoundaryArea, error) {
	if c == nil || c.pairs[0] == nil || c.pairs[1] == nil {
		return NormalOffsetSweepJoinedBoundaryArea{}, ErrInvalid
	}
	return encloseDirectedSweepBoundaryArea(ctx, c.sweeps[:], c.Boundary(), o)
}
