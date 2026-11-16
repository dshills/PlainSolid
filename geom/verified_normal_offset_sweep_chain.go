package geom

import (
	"context"
	"math/big"
)

// VerifiedNormalOffsetSweepChain is an embedded joined volume of three
// original sweep charts with two disjoint swept walls.
type VerifiedNormalOffsetSweepChain struct {
	sweeps       [3]*VerifiedNormalOffsetSweep
	pairs        [2]*VerifiedNormalOffsetSweepPair
	endSeparator *VerifiedNormalOffsetSweepChainSeparator
}

// VerifiedNormalOffsetSweepChainProof records the positive gap between
// the two walls in the first join's original projection scale. The end
// charts lie in closed halfspaces separated by this gap.
type VerifiedNormalOffsetSweepChainProof struct {
	Complete        bool
	PlaneGap        *big.Rat
	ExactOperations int
}

// VerifiedNormalOffsetSweepChainJoin retains the original chart owner
// indices, original station correspondence and exact separating constant.
type VerifiedNormalOffsetSweepChainJoin struct {
	Owners     [2]int
	Original   VerifiedNormalOffsetSweepJoin
	Coordinate *big.Rat
}

// NewVerifiedNormalOffsetSweepChain composes independent complete joins
// between A and B and between B and C. The two certificates must retain
// the same original middle sweep and distinct opposing sides on that
// chart. Their exact plane covectors must be proportional. In one common
// projection scale the middle chart spans the stripe, while the closed
// halfspaces containing A and C are separated by a positive plane gap.
// This excludes end-chart contact over all displacement layers.
func NewVerifiedNormalOffsetSweepChain(ctx context.Context, ab, bc *VerifiedNormalOffsetSweepPair,
	o SurfacePatchOptions) (_ *VerifiedNormalOffsetSweepChain, r VerifiedNormalOffsetSweepChainProof, err error) {
	if ctx == nil || ab == nil || bc == nil || ab.coordinate == nil || bc.coordinate == nil {
		return nil, r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	limits, err := patchOptions(o)
	if err != nil {
		return nil, r, err
	}
	leftMiddle, rightMiddle := ab.join.Edges[1], bc.join.Edges[0]
	if ab.sweeps[1] != bc.sweeps[0] || leftMiddle == rightMiddle || leftMiddle/2 != rightMiddle/2 {
		return nil, r, ErrDomain
	}

	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	x, y := sweepJoinPlaneCovector(ab.join), sweepJoinPlaneCovector(bc.join)
	scalars := boundedRatArithmetic{b: budget}
	scale, err := sweepChainPlaneScale(x, y, &scalars)
	if err != nil {
		return nil, r, err
	}
	// The first end and the middle (on the second join) point toward
	// the same side of their respective planes in the common scale.
	// Their signs are opposite to the direction from plane A/B to B/C.
	endSign := ab.join.AInteriorSign
	middleSign := bc.join.AInteriorSign * scale.Sign()
	if endSign != middleSign {
		return nil, r, ErrDomain
	}
	for _, c := range []*big.Rat{ab.coordinate, bc.coordinate} {
		if err := budget.check(c); err != nil {
			return nil, r, err
		}
	}
	right := scalars.quo(bc.coordinate, scale)
	gap := scalars.sub(right, ab.coordinate)
	if scalars.err != nil {
		return nil, r, scalars.err
	}
	if gap.Sign() != -endSign {
		return nil, r, ErrDomain
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	r.Complete, r.PlaneGap = true, gap.Abs(gap)
	return &VerifiedNormalOffsetSweepChain{
		sweeps: [3]*VerifiedNormalOffsetSweep{ab.sweeps[0], ab.sweeps[1], bc.sweeps[1]},
		pairs:  [2]*VerifiedNormalOffsetSweepPair{ab, bc},
	}, r, nil
}

// Sweeps returns the three immutable original sweep charts in chain order.
func (c *VerifiedNormalOffsetSweepChain) Sweeps() [3]*VerifiedNormalOffsetSweep {
	if c == nil {
		return [3]*VerifiedNormalOffsetSweep{}
	}
	return c.sweeps
}

// Joins returns both certified joins with their original projection
// scales, exact station maps and owner indices. Rational values are owned
// by the caller.
func (c *VerifiedNormalOffsetSweepChain) Joins() [2]VerifiedNormalOffsetSweepChainJoin {
	var result [2]VerifiedNormalOffsetSweepChainJoin
	if c == nil || c.pairs[0] == nil {
		return result
	}
	for i, pair := range c.pairs {
		original, coordinate := pair.Join()
		result[i] = VerifiedNormalOffsetSweepChainJoin{Owners: [2]int{i, i + 1}, Original: original, Coordinate: coordinate}
	}
	return result
}

// Boundary returns the fourteen retained outer analytic faces in original
// chart and side order. Owner is the original chart index (0, 1 or 2).
// The four common-wall copies are interior to the joined volume.
func (c *VerifiedNormalOffsetSweepChain) Boundary() []VerifiedNormalOffsetSweepBoundaryFace {
	if c == nil || c.pairs[0] == nil {
		return nil
	}
	omitted := [3]uint8{}
	for i, pair := range c.pairs {
		omitted[i] |= 1 << pair.join.Edges[0]
		omitted[i+1] |= 1 << pair.join.Edges[1]
	}
	result := make([]VerifiedNormalOffsetSweepBoundaryFace, 0, 14)
	for owner, sweep := range c.sweeps {
		for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
			if omitted[owner]&(1<<side) == 0 {
				result = append(result, VerifiedNormalOffsetSweepBoundaryFace{
					Owner: owner, Side: side, Face: VerifiedNormalOffsetSweepFace{sweep, side},
				})
			}
		}
	}
	return result
}

func sweepJoinPlaneCovector(join VerifiedNormalOffsetSweepJoin) (q [3]*big.Rat) {
	if join.Covector != nil {
		q = *join.Covector
	} else {
		q[join.Axis] = big.NewRat(1, 1)
	}
	zero := new(big.Rat)
	for i := range q {
		if q[i] == nil {
			q[i] = zero
		}
	}
	return q
}

func sweepChainPlaneScale(a, b [3]*big.Rat, exact *boundedRatArithmetic) (*big.Rat, error) {
	for i := range a {
		for _, q := range []*big.Rat{a[i], b[i]} {
			if err := exact.b.check(q); err != nil {
				return nil, err
			}
		}
	}
	var scale *big.Rat
	for i, q := range a {
		if q.Sign() != 0 {
			scale = exact.quo(b[i], q)
			break
		}
	}
	if exact.err != nil {
		return nil, exact.err
	}
	if scale == nil || scale.Sign() == 0 {
		return nil, ErrDomain
	}
	for i, q := range a {
		value := exact.mul(q, scale)
		if exact.err != nil {
			return nil, exact.err
		}
		if value.Cmp(b[i]) != 0 {
			return nil, ErrDomain
		}
	}
	return scale, nil
}
