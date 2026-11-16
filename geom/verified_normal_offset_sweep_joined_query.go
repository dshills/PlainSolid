package geom

import (
	"context"
	"math/big"
	"math/bits"
)

// NormalOffsetSweepJoinedPointOwner retains the directed inverse bounds in
// one original chart's U, V and signed displacement coordinates. Faces lists
// every retained outer face of that chart incident to the point.
// AlgebraicStation, when present, identifies the isolated normal-foot root
// in the original profile and its signed displacement enclosure.
type NormalOffsetSweepJoinedPointOwner struct {
	Owner            int
	Parameters       [3]ExactParameterInterval
	Faces            []NormalOffsetSweepSide
	AlgebraicStation *NormalOffsetSweepLineAlgebraicStation `json:",omitempty"`
}

// NormalOffsetSweepJoinedQueryResult records membership in a verified joined
// volume. Interior points on an omitted wall have both original owners and
// no retained faces. Exterior results have no owners. Unresolved queries
// report work without a partial membership or owner certificate.
type NormalOffsetSweepJoinedQueryResult struct {
	Location                      NormalOffsetSweepLocation
	Owners                        []NormalOffsetSweepJoinedPointOwner
	VisitedCells, ExactOperations int
}

// LocateRational classifies a rational model point in the verified pair.
// The exact operation and visited cell limits are shared across original
// charts. MaxDepth applies separately to each original inverse partition.
// A supported original ruled or whole-wall proof counts as one visited cell.
func (p *VerifiedNormalOffsetSweepPair) LocateRational(ctx context.Context, point [3]*big.Rat,
	o NormalOffsetSweepQueryOptions) (NormalOffsetSweepJoinedQueryResult, error) {
	if p == nil || p.coordinate == nil {
		return NormalOffsetSweepJoinedQueryResult{}, ErrInvalid
	}
	return locateJoinedNormalOffsetSweep(ctx, p.sweeps[:], []*VerifiedNormalOffsetSweepPair{p}, point, o)
}

// LocateRational classifies a rational model point in the three-chart chain.
// Both original joining walls are interior wherever their free parameters
// are strictly inside. Their retained outer perimeter keeps its original
// chart and face incidences. The same limits apply as for a verified pair.
func (c *VerifiedNormalOffsetSweepChain) LocateRational(ctx context.Context, point [3]*big.Rat,
	o NormalOffsetSweepQueryOptions) (NormalOffsetSweepJoinedQueryResult, error) {
	if c == nil || c.pairs[0] == nil || c.pairs[1] == nil {
		return NormalOffsetSweepJoinedQueryResult{}, ErrInvalid
	}
	return locateJoinedNormalOffsetSweep(ctx, c.sweeps[:], c.pairs[:], point, o)
}

func locateJoinedNormalOffsetSweep(ctx context.Context, sweeps []*VerifiedNormalOffsetSweep,
	joins []*VerifiedNormalOffsetSweepPair, point [3]*big.Rat,
	o NormalOffsetSweepQueryOptions) (r NormalOffsetSweepJoinedQueryResult, err error) {
	if ctx == nil || o.MaxDepth < 0 || o.MaxDepth > 64 || o.MaxCells < 0 || o.MaxCells > 1<<18 {
		return r, ErrInvalid
	}
	for _, q := range point {
		if q == nil {
			return r, ErrInvalid
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 4_000_000
	}
	limits, err := patchOptions(o.Patch)
	if err != nil {
		return r, err
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	for _, q := range point {
		if err := budget.check(q); err != nil {
			return r, err
		}
	}

	// Strict separation for the original pairs and the chain's ordered
	// parallel planes excludes entire chart groups. On a plane, its two
	// adjacent closed charts may meet the point only on the same wall.
	eligible := uint8((1 << len(sweeps)) - 1)
	var omitted, fixed [3]uint8
	a := boundedRatArithmetic{b: budget}
	for i, pair := range joins {
		leftWall, rightWall := uint8(1<<pair.join.Edges[0]), uint8(1<<pair.join.Edges[1])
		omitted[i] |= leftWall
		omitted[i+1] |= rightWall
		covector := sweepJoinPlaneCovector(pair.join)
		if err := budget.check(pair.coordinate); err != nil {
			return r, err
		}
		for _, q := range covector {
			if err := budget.check(q); err != nil {
				return r, err
			}
		}
		projection := sweepLineDot(&a, covector, point)
		if a.err != nil {
			return r, a.err
		}
		leftCharts := uint8((1 << (i + 1)) - 1)
		switch projection.Cmp(pair.coordinate) * pair.join.AInteriorSign {
		case 1:
			eligible &= leftCharts
		case -1:
			eligible &^= leftCharts
		default:
			eligible &= 3 << i
			fixed[i], fixed[i+1] = leftWall, rightWall
		}
	}
	var owners []NormalOffsetSweepJoinedPointOwner
	var hits uint8
	boundary := false
	for owner, sweep := range sweeps {
		bit := uint8(1 << owner)
		if eligible&bit == 0 {
			continue
		}
		if r.VisitedCells >= o.MaxCells || budget.operations >= budget.maxOperations {
			return r, ErrConvergence
		}
		query := NormalOffsetSweepQueryOptions{Patch: limits, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells - r.VisitedCells}
		witness, visited, err := locateJoinedSweepPointOwner(ctx, owner, sweep, point, fixed[owner], omitted[owner], query, budget)
		r.VisitedCells += visited
		if err != nil {
			return r, err
		}
		if witness == nil {
			continue
		}
		boundary = boundary || len(witness.Faces) != 0
		owners = append(owners, *witness)
		hits |= bit
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if eligible == 0 || (hits != 0 && hits != eligible) {
		return r, ErrConvergence
	}
	r.Owners = owners
	switch {
	case hits == 0:
		r.Location = NormalOffsetSweepExterior
	case boundary:
		r.Location = NormalOffsetSweepBoundary
	default:
		// The relative interior of a common wall has two regular
		// incident volume charts covering its opposite local sides.
		r.Location = NormalOffsetSweepInterior
	}
	return r, nil
}

func locateJoinedSweepPointOwner(ctx context.Context, owner int, sweep *VerifiedNormalOffsetSweep, point [3]*big.Rat,
	fixed, omitted uint8, query NormalOffsetSweepQueryOptions, budget *polynomialBudget) (*NormalOffsetSweepJoinedPointOwner, int, error) {
	if len(sweep.source.net) > query.Patch.MaxPoles {
		return nil, 0, ErrConvergence
	}
	// A higher-degree original chart can supply an exact ruled normal-foot
	// proof. Its entire original extrusion counts as one visited cell.
	if sweep.source.degreeU > 1 || sweep.source.degreeV > 1 {
		witness, proved, err := joinedSweepRulingPoint(ctx, owner, sweep, point, fixed, omitted, budget)
		if proved || err != nil {
			return witness, 1, err
		}
		if fixed != 0 {
			side := NormalOffsetSweepSide(bits.TrailingZeros8(fixed))
			witness, proved, err := joinedSweepWallPoint(ctx, owner, sweep, point, side, budget)
			if proved || err != nil {
				return witness, 1, err
			}
		}
	}
	if budget.operations >= budget.maxOperations {
		return nil, 0, ErrConvergence
	}
	query.Patch.MaxOperations = budget.maxOperations - budget.operations
	original, err := sweep.LocateRational(ctx, point, query)
	budget.operations += original.ExactOperations
	if err != nil || original.Location == NormalOffsetSweepExterior {
		return nil, original.VisitedCells, err
	}
	witness, err := joinedSweepPointOwner(owner, sweep, original.Parameters, fixed, omitted)
	return &witness, original.VisitedCells, err
}

func joinedSweepPointOwner(owner int, sweep *VerifiedNormalOffsetSweep, parameters [3]ExactParameterInterval,
	fixed, omitted uint8) (NormalOffsetSweepJoinedPointOwner, error) {
	witness := NormalOffsetSweepJoinedPointOwner{Owner: owner, Parameters: parameters}
	original := normalSweepParameterIntervals(sweep.source.Domain(), exactInterval{sweep.lower, sweep.upper})
	for axis, domain := range original {
		bounds := witness.Parameters[axis]
		if bounds.Lower == nil || bounds.Upper == nil {
			return NormalOffsetSweepJoinedPointOwner{}, ErrConvergence
		}
		lo, hi := NormalOffsetSweepSide(2*axis), NormalOffsetSweepSide(2*axis+1)
		// The exact plane already pins a wall coordinate. The original
		// inverse enclosure can independently be wider in that axis.
		if fixed&(1<<lo|1<<hi) != 0 {
			station := domain.lo
			if fixed&(1<<hi) != 0 {
				station = domain.hi
			}
			if bounds.Lower.Cmp(station) > 0 || bounds.Upper.Cmp(station) < 0 {
				return NormalOffsetSweepJoinedPointOwner{}, ErrConvergence
			}
			bounds = copySweepInterval(exactInterval{station, station})
		}
		witness.Parameters[axis] = bounds
		switch {
		case bounds.Lower.Cmp(domain.lo) == 0 && bounds.Upper.Cmp(domain.lo) == 0:
			if omitted&(1<<lo) == 0 {
				witness.Faces = append(witness.Faces, lo)
			}
		case bounds.Lower.Cmp(domain.hi) == 0 && bounds.Upper.Cmp(domain.hi) == 0:
			if omitted&(1<<hi) == 0 {
				witness.Faces = append(witness.Faces, hi)
			}
		case bounds.Lower.Cmp(domain.lo) > 0 && bounds.Upper.Cmp(domain.hi) < 0:
		default:
			// Existence alone does not distinguish a retained perimeter
			// from the relative interior of a face or joined wall.
			return NormalOffsetSweepJoinedPointOwner{}, ErrConvergence
		}
	}
	return witness, nil
}
