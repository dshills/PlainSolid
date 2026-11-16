package geom

import (
	"context"
	"math/big"
)

// VerifiedNormalOffsetSweepChainSeparator specifies a strict independent
// separating plane for the two end volumes of a nonparallel chain. The first
// end must be below Coordinate in this projection and the last end above it.
// Nil covector components denote zero.
type VerifiedNormalOffsetSweepChainSeparator struct {
	Covector   [3]*big.Rat
	Coordinate *big.Rat
}

// VerifiedNonparallelNormalOffsetSweepChainProof records the whole end-volume
// projection enclosures in the separator's original scale. The ranges and
// positive gap are published only for a complete chain.
type VerifiedNonparallelNormalOffsetSweepChainProof struct {
	Complete        bool
	Separator       *VerifiedNormalOffsetSweepChainSeparator
	EndRanges       [2]ExactParameterInterval
	Gap             *big.Rat
	ExactOperations int
}

// NewVerifiedNonparallelNormalOffsetSweepChain composes the original A/B
// and B/C pair certificates on nonparallel planes. Their common middle
// chart must use opposite sides of one original parameter. This keeps the
// two full swept walls disjoint. The supplied separator must put all of A
// and all of C in strict opposite halfspaces over their full signed layers.
// MaxDepth and MaxCells bound subdivision per end chart. Exact arithmetic
// is shared across the complete separating proof.
func NewVerifiedNonparallelNormalOffsetSweepChain(ctx context.Context, ab, bc *VerifiedNormalOffsetSweepPair,
	separator VerifiedNormalOffsetSweepChainSeparator, options NormalOffsetCoverageOptions) (
	_ *VerifiedNormalOffsetSweepChain, r VerifiedNonparallelNormalOffsetSweepChainProof, err error) {
	if ctx == nil || ab == nil || bc == nil || ab.coordinate == nil || bc.coordinate == nil ||
		separator.Coordinate == nil || !nonzeroSweepSeparatorCovector(separator.Covector) ||
		options.MaxDepth < 0 || options.MaxDepth > 64 || options.MaxCells < 0 || options.MaxCells > 1<<18 {
		return nil, r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	limits, err := patchOptions(options.Patch)
	if err != nil {
		return nil, r, err
	}
	leftMiddle, rightMiddle := ab.join.Edges[1], bc.join.Edges[0]
	if ab.sweeps[1] != bc.sweeps[0] || leftMiddle == rightMiddle || leftMiddle/2 != rightMiddle/2 {
		return nil, r, ErrDomain
	}

	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	x, y := sweepJoinPlaneCovector(ab.join), sweepJoinPlaneCovector(bc.join)
	for _, q := range [3][3]*big.Rat{x, y, separator.Covector} {
		if err := checkSweepSeparatorCovector(q, b); err != nil {
			return nil, r, err
		}
	}
	if err := b.check(separator.Coordinate); err != nil {
		return nil, r, err
	}
	a := boundedRatArithmetic{b: b}
	parallel := sweepLineParallel(&a, x, y)
	if a.err != nil {
		return nil, r, a.err
	}
	if parallel {
		return nil, r, ErrDomain
	}
	separator = copySweepChainSeparator(separator)
	options.Patch = limits
	ends, gap, err := verifySweepChainEndSeparation(ctx, [2]*VerifiedNormalOffsetSweep{ab.sweeps[0], bc.sweeps[1]}, separator, options, b, nil)
	if err != nil {
		return nil, r, err
	}
	ownedSeparator := copySweepChainSeparator(separator)
	r.Complete, r.Separator, r.Gap = true, &ownedSeparator, gap
	r.EndRanges = [2]ExactParameterInterval{copySweepInterval(ends[0]), copySweepInterval(ends[1])}
	return &VerifiedNormalOffsetSweepChain{
		sweeps:       [3]*VerifiedNormalOffsetSweep{ab.sweeps[0], ab.sweeps[1], bc.sweeps[1]},
		pairs:        [2]*VerifiedNormalOffsetSweepPair{ab, bc},
		endSeparator: &separator,
	}, r, nil
}

type sweepChainSeparationCells struct{ max, visited int }

func verifySweepChainEndSeparation(ctx context.Context, sweeps [2]*VerifiedNormalOffsetSweep,
	separator VerifiedNormalOffsetSweepChainSeparator, options NormalOffsetCoverageOptions,
	b *polynomialBudget, cells *sweepChainSeparationCells) (ends [2]exactInterval, gap *big.Rat, err error) {
	a := boundedRatArithmetic{b: b}
	oneNorm := new(big.Rat)
	for _, q := range separator.Covector {
		oneNorm = a.add(oneNorm, new(big.Rat).Abs(q))
	}
	if a.err != nil {
		return ends, nil, a.err
	}
	for i, sweep := range sweeps {
		options.Patch.MaxOperations = b.maxOperations - b.operations
		if options.Patch.MaxOperations <= 0 {
			return ends, nil, ErrConvergence
		}
		if cells != nil {
			options.MaxCells = cells.max - cells.visited
			if options.MaxCells <= 0 {
				return ends, nil, ErrConvergence
			}
		}
		coverage, err := sweep.source.certifyNormalOffsetCells(ctx, sweep.upper, options,
			func(ctx context.Context, patch *SurfacePatch, upper *big.Rat, work *polynomialBudget) (bool, error) {
				// A sealed original sweep is a single Bézier patch,
				// so every visited projection cell reaches this test.
				if cells != nil {
					cells.visited++
				}
				projection, err := normalSweepEndProjection(ctx, patch, exactInterval{sweep.lower, upper}, separator.Covector, oneNorm, work)
				if err != nil {
					return false, err
				}
				lo := projection.lo.Cmp(separator.Coordinate)
				hi := projection.hi.Cmp(separator.Coordinate)
				if i == 0 {
					lo, hi = -hi, -lo
				}
				if hi <= 0 {
					return false, ErrDomain
				}
				if lo <= 0 {
					return false, nil
				}
				hull := &ends[i]
				if hull.lo == nil || hull.lo.Cmp(projection.lo) > 0 {
					hull.lo = projection.lo
				}
				if hull.hi == nil || hull.hi.Cmp(projection.hi) < 0 {
					hull.hi = projection.hi
				}
				return true, nil
			})
		b.operations += coverage.ExactOperations
		if err != nil {
			return ends, nil, err
		}
		if !coverage.Complete {
			return ends, nil, ErrConvergence
		}
	}
	gap = a.sub(ends[1].lo, ends[0].hi)
	if a.err != nil {
		return ends, nil, a.err
	}
	if err := ctx.Err(); err != nil {
		return ends, nil, err
	}
	return ends, gap, nil
}

func copySweepChainSeparator(s VerifiedNormalOffsetSweepChainSeparator) VerifiedNormalOffsetSweepChainSeparator {
	r := VerifiedNormalOffsetSweepChainSeparator{Coordinate: new(big.Rat).Set(s.Coordinate)}
	for i, q := range s.Covector {
		r.Covector[i] = new(big.Rat)
		if q != nil {
			r.Covector[i].Set(q)
		}
	}
	return r
}

func normalSweepEndProjection(ctx context.Context, p *SurfacePatch, displacement exactInterval,
	covector [3]*big.Rat, oneNorm *big.Rat, b *polynomialBudget) (exactInterval, error) {
	a := boundedRatArithmetic{b: b}
	var base exactInterval
	for _, h := range p.net {
		if err := ctx.Err(); err != nil {
			return exactInterval{}, err
		}
		location := a.quo(sweepLineDot(&a, covector, [3]*big.Rat(h[:3])), h[3])
		if a.err != nil {
			return exactInterval{}, a.err
		}
		if base.lo == nil || base.lo.Cmp(location) > 0 {
			base.lo = location
		}
		if base.hi == nil || base.hi.Cmp(location) < 0 {
			base.hi = location
		}
	}
	// Positive source weights keep the Cartesian projected control hull
	// valid. A unit normal has projection at most the covector's L1 norm.
	// Derivative bounds of the original patch can tighten that fallback.
	n := exactInterval{new(big.Rat).Neg(oneNorm), oneNorm}
	d, err := offsetCellJacobianBounds(ctx, p, b)
	if err != nil {
		return exactInterval{}, err
	}
	if d != nil {
		scalars := &d.arithmetic
		projected := offsetZeroInterval()
		for i, q := range covector {
			if q.Sign() != 0 {
				projected = scalars.add(projected, scalars.mul(exactInterval{q, q}, d.normal[i]))
			}
		}
		tighter := scalars.div(projected, d.length)
		if scalars.err != nil {
			return exactInterval{}, scalars.err
		}
		if tighter.lo.Cmp(n.lo) > 0 {
			n.lo = tighter.lo
		}
		if tighter.hi.Cmp(n.hi) < 0 {
			n.hi = tighter.hi
		}
	}
	i := offsetIntervalArithmetic{b: b}
	projection := i.add(base, i.mul(displacement, n))
	return projection, i.err
}
