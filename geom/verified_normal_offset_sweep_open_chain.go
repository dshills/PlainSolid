package geom

import (
	"context"
	"slices"
)

// VerifiedNormalOffsetSweepOpenChain is an embedded open chain of two
// through eight original sweep charts joined along complete swept walls.
type VerifiedNormalOffsetSweepOpenChain struct {
	sweeps     []*VerifiedNormalOffsetSweep
	joins      []*VerifiedNormalOffsetSweepPair
	separators []VerifiedNormalOffsetSweepNonadjacentSeparator
}

// VerifiedNormalOffsetSweepNonadjacentSeparator supplies the original
// owner indices and independent strict projection plane for one
// nonadjacent pair. The first owner index is smaller.
type VerifiedNormalOffsetSweepNonadjacentSeparator struct {
	Owners    [2]int
	Separator VerifiedNormalOffsetSweepChainSeparator
}

// VerifiedNormalOffsetSweepOpenChainProof retains the complete original
// nonadjacent proofs in lexicographic owner order. Resource counts include
// every attempted independent projected range. An incomplete proof
// publishes only its resource counts.
type VerifiedNormalOffsetSweepOpenChainProof struct {
	Complete                      bool
	Nonadjacent                   []VerifiedNormalOffsetSweepNonadjacentProof
	VisitedCells, ExactOperations int
}

// NewVerifiedNormalOffsetSweepOpenChain composes one through seven
// original adjacent pair certificates in chart order. Each interior
// chart retains two opposite original parameter walls. Supply exactly
// one independent separator for every nonadjacent pair, in any order.
// The exact-operation and visited-cell limits are shared across all
// nonadjacent proofs. MaxDepth limits each original range subdivision.
func NewVerifiedNormalOffsetSweepOpenChain(ctx context.Context, joins []*VerifiedNormalOffsetSweepPair,
	separators []VerifiedNormalOffsetSweepNonadjacentSeparator, options NormalOffsetCoverageOptions) (
	_ *VerifiedNormalOffsetSweepOpenChain, r VerifiedNormalOffsetSweepOpenChainProof, err error) {
	if ctx == nil || len(joins) < 1 || len(joins) > 7 || options.MaxDepth < 0 || options.MaxDepth > 64 ||
		options.MaxCells < 0 || options.MaxCells > 1<<18 {
		return nil, r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	limits, err := patchOptions(options.Patch)
	if err != nil {
		return nil, r, err
	}
	count := len(joins) + 1
	expected := (count - 1) * (count - 2) / 2
	if len(separators) != expected {
		return nil, r, ErrInvalid
	}
	byOwners := make(map[[2]int]VerifiedNormalOffsetSweepChainSeparator, expected)
	for _, entry := range separators {
		i, j := entry.Owners[0], entry.Owners[1]
		separator := entry.Separator
		if i < 0 || j < 0 || j >= count || i >= j-1 || separator.Coordinate == nil ||
			!nonzeroSweepSeparatorCovector(separator.Covector) {
			return nil, r, ErrInvalid
		}
		if _, duplicate := byOwners[entry.Owners]; duplicate {
			return nil, r, ErrInvalid
		}
		byOwners[entry.Owners] = separator
	}
	sweeps := make([]*VerifiedNormalOffsetSweep, count)
	for i, join := range joins {
		if join == nil || join.coordinate == nil {
			return nil, r, ErrInvalid
		}
		if i == 0 {
			sweeps[0] = join.sweeps[0]
		} else {
			incoming, outgoing := joins[i-1].join.Edges[1], join.join.Edges[0]
			if sweeps[i] != join.sweeps[0] || incoming == outgoing || incoming/2 != outgoing/2 {
				return nil, r, ErrDomain
			}
		}
		sweeps[i+1] = join.sweeps[1]
	}
	options.Patch = limits
	if options.MaxCells == 0 {
		options.MaxCells = 4096
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	cells := sweepChainSeparationCells{max: options.MaxCells}
	defer func() { r.VisitedCells, r.ExactOperations = cells.visited, b.operations }()
	proofs := make([]VerifiedNormalOffsetSweepNonadjacentProof, 0, expected)
	for i := 0; i < count-2; i++ {
		for j := i + 2; j < count; j++ {
			owners := [2]int{i, j}
			separator := byOwners[owners]
			if err := checkSweepSeparatorCovector(separator.Covector, b); err != nil {
				return nil, r, err
			}
			if err := b.check(separator.Coordinate); err != nil {
				return nil, r, err
			}
			separator = copySweepChainSeparator(separator)
			ends := [2]*VerifiedNormalOffsetSweep{sweeps[i], sweeps[j]}
			ranges, gap, err := verifySweepChainEndSeparation(ctx, ends, separator, options, b, &cells)
			if err != nil {
				return nil, r, err
			}
			proofs = append(proofs, VerifiedNormalOffsetSweepNonadjacentProof{
				Owners: owners, Separator: separator, Gap: gap,
				Ranges: [2]ExactParameterInterval{copySweepInterval(ranges[0]), copySweepInterval(ranges[1])},
			})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	originals := make([]VerifiedNormalOffsetSweepNonadjacentSeparator, len(proofs))
	for i, proof := range proofs {
		originals[i] = VerifiedNormalOffsetSweepNonadjacentSeparator{
			Owners: proof.Owners, Separator: copySweepChainSeparator(proof.Separator),
		}
	}
	r.Complete, r.Nonadjacent = true, proofs
	return &VerifiedNormalOffsetSweepOpenChain{sweeps: sweeps, joins: slices.Clone(joins), separators: originals}, r, nil
}

// Sweeps returns the immutable original charts in order. The caller owns
// the returned slice.
func (c *VerifiedNormalOffsetSweepOpenChain) Sweeps() []*VerifiedNormalOffsetSweep {
	if c == nil {
		return nil
	}
	return slices.Clone(c.sweeps)
}

// Joins returns the complete adjacent original wall maps and owner indices
// in chain order. The caller owns the returned values.
func (c *VerifiedNormalOffsetSweepOpenChain) Joins() []VerifiedNormalOffsetSweepChainJoin {
	if c == nil || len(c.joins) == 0 {
		return nil
	}
	r := make([]VerifiedNormalOffsetSweepChainJoin, len(c.joins))
	for i, pair := range c.joins {
		join, coordinate := pair.Join()
		r[i] = VerifiedNormalOffsetSweepChainJoin{Owners: [2]int{i, i + 1}, Original: join, Coordinate: coordinate}
	}
	return r
}

// NonadjacentSeparators returns the exact original separator planes in
// lexicographic owner order. The caller owns the returned values.
func (c *VerifiedNormalOffsetSweepOpenChain) NonadjacentSeparators() []VerifiedNormalOffsetSweepNonadjacentSeparator {
	if c == nil {
		return nil
	}
	result := make([]VerifiedNormalOffsetSweepNonadjacentSeparator, len(c.separators))
	for i, entry := range c.separators {
		result[i] = VerifiedNormalOffsetSweepNonadjacentSeparator{
			Owners: entry.Owners, Separator: copySweepChainSeparator(entry.Separator),
		}
	}
	return result
}

// Boundary returns the retained outer faces in original chart and side
// order. A chain of N owners retains 4*N+2 faces.
func (c *VerifiedNormalOffsetSweepOpenChain) Boundary() []VerifiedNormalOffsetSweepBoundaryFace {
	if c == nil || len(c.joins) == 0 {
		return nil
	}
	return verifiedSweepChainBoundary(c.sweeps, c.joins)
}
