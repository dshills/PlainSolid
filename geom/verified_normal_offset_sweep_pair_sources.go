package geom

// Sweeps returns the two immutable original sweep charts in owner order.
func (p *VerifiedNormalOffsetSweepPair) Sweeps() [2]*VerifiedNormalOffsetSweep {
	if p == nil {
		return [2]*VerifiedNormalOffsetSweep{}
	}
	return p.sweeps
}
