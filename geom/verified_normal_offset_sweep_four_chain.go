package geom

import (
	"context"
	"math/big"
)

// VerifiedFourChartNormalOffsetSweepChain is an embedded open chain of four
// original normal sweeps with three complete swept-wall joins.
type VerifiedFourChartNormalOffsetSweepChain struct {
	sweeps [4]*VerifiedNormalOffsetSweep
	pairs  [3]*VerifiedNormalOffsetSweepPair
}

// VerifiedNormalOffsetSweepNonadjacentProof records a directed projection
// separation between two specific original owners. Ranges and gap use the
// scale of that pair's independent separator.
type VerifiedNormalOffsetSweepNonadjacentProof struct {
	Owners    [2]int
	Separator VerifiedNormalOffsetSweepChainSeparator
	Ranges    [2]ExactParameterInterval
	Gap       *big.Rat
}

// VerifiedFourChartNormalOffsetSweepChainProof retains each independent
// nonadjacent separation when all three certificates succeed.
type VerifiedFourChartNormalOffsetSweepChainProof struct {
	Complete        bool
	Nonadjacent     [3]VerifiedNormalOffsetSweepNonadjacentProof
	ExactOperations int
}

// NewVerifiedFourChartNormalOffsetSweepChain composes three complete joins
// in original chart order. Each interior chart uses opposite original
// walls of one parameter. Supply one independent separator for each
// nonadjacent pair in order (0,2), (0,3), (1,3), opening a strict gap between
// the complete swept images. The join planes may be parallel or nonparallel.
// Cell and depth limits apply per end-range proof and exact arithmetic is
// shared across the three proofs. No chain or separation records are
// returned until the complete topology is certified.
func NewVerifiedFourChartNormalOffsetSweepChain(ctx context.Context, pairs [3]*VerifiedNormalOffsetSweepPair,
	separators [3]VerifiedNormalOffsetSweepChainSeparator, options NormalOffsetCoverageOptions) (
	_ *VerifiedFourChartNormalOffsetSweepChain, r VerifiedFourChartNormalOffsetSweepChainProof, err error) {
	if ctx == nil || options.MaxDepth < 0 || options.MaxDepth > 64 || options.MaxCells < 0 || options.MaxCells > 1<<18 {
		return nil, r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	limits, err := patchOptions(options.Patch)
	if err != nil {
		return nil, r, err
	}
	for i, pair := range pairs {
		if pair == nil || pair.coordinate == nil || separators[i].Coordinate == nil || !nonzeroSweepSeparatorCovector(separators[i].Covector) {
			return nil, r, ErrInvalid
		}
	}
	for i := 1; i < len(pairs); i++ {
		before, after := pairs[i-1], pairs[i]
		incoming, outgoing := before.join.Edges[1], after.join.Edges[0]
		if before.sweeps[1] != after.sweeps[0] || incoming == outgoing || incoming/2 != outgoing/2 {
			return nil, r, ErrDomain
		}
	}
	sweeps := [4]*VerifiedNormalOffsetSweep{pairs[0].sweeps[0], pairs[0].sweeps[1], pairs[1].sweeps[1], pairs[2].sweeps[1]}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	options.Patch = limits
	var proofs [3]VerifiedNormalOffsetSweepNonadjacentProof
	for i, owners := range [3][2]int{{0, 2}, {0, 3}, {1, 3}} {
		separator := separators[i]
		if err := checkSweepSeparatorCovector(separator.Covector, b); err != nil {
			return nil, r, err
		}
		if err := b.check(separator.Coordinate); err != nil {
			return nil, r, err
		}
		separator = copySweepChainSeparator(separator)
		ends := [2]*VerifiedNormalOffsetSweep{sweeps[owners[0]], sweeps[owners[1]]}
		ranges, gap, err := verifySweepChainEndSeparation(ctx, ends, separator, options, b)
		if err != nil {
			return nil, r, err
		}
		proofs[i] = VerifiedNormalOffsetSweepNonadjacentProof{
			Owners: owners, Separator: separator, Gap: gap,
			Ranges: [2]ExactParameterInterval{copySweepInterval(ranges[0]), copySweepInterval(ranges[1])},
		}
	}
	r.Complete, r.Nonadjacent = true, proofs
	return &VerifiedFourChartNormalOffsetSweepChain{sweeps: sweeps, pairs: pairs}, r, nil
}

// Sweeps returns the four immutable original sweep charts in chain order.
func (c *VerifiedFourChartNormalOffsetSweepChain) Sweeps() [4]*VerifiedNormalOffsetSweep {
	if c == nil {
		return [4]*VerifiedNormalOffsetSweep{}
	}
	return c.sweeps
}

// Joins returns the three original adjacent wall certificates and their
// owner indices. The returned chart maps and constants are caller-owned.
func (c *VerifiedFourChartNormalOffsetSweepChain) Joins() (r [3]VerifiedNormalOffsetSweepChainJoin) {
	if c == nil || c.pairs[0] == nil {
		return r
	}
	for i, pair := range c.pairs {
		original, coordinate := pair.Join()
		r[i] = VerifiedNormalOffsetSweepChainJoin{Owners: [2]int{i, i + 1}, Original: original, Coordinate: coordinate}
	}
	return r
}

// Boundary returns the eighteen retained original outer faces in chart
// and side order. All six copies of the joining walls are interior.
func (c *VerifiedFourChartNormalOffsetSweepChain) Boundary() []VerifiedNormalOffsetSweepBoundaryFace {
	if c == nil || c.pairs[0] == nil {
		return nil
	}
	return verifiedSweepChainBoundary(c.sweeps[:], c.pairs[:])
}
