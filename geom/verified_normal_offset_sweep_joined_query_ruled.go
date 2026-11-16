package geom

import (
	"context"
	"errors"
	"math/big"
)

// Use the same complete transverse normal-foot proof as the original
// ruling-line query. The requested point is at the exact line station zero.
func joinedSweepRulingPoint(ctx context.Context, owner int, sweep *VerifiedNormalOffsetSweep, query [3]*big.Rat,
	fixed, omitted uint8, budget *polynomialBudget) (*NormalOffsetSweepJoinedPointOwner, bool, error) {
	work := sweepLineWork{
		sweep: sweep, origin: query, budget: budget,
		original: normalSweepParameterIntervals(sweep.source.Domain(), exactInterval{sweep.lower, sweep.upper}),
	}
	p := sweep.source
	for _, bound := range [2]*big.Rat{sweep.lower, sweep.upper} {
		if err := budget.check(bound); err != nil {
			return nil, false, err
		}
	}
	for axis, otherDegree := range [2]int{p.degreeV, p.degreeU} {
		if otherDegree > 3 {
			continue
		}
		last := p.net[(p.countU-1)*p.countV]
		if axis == 1 {
			last = p.net[p.countV-1]
		}
		first := p.net[0]
		if first[3].Cmp(last[3]) != 0 {
			continue
		}
		a := boundedRatArithmetic{b: budget}
		for i := range work.direction {
			work.direction[i] = a.sub(last[i], first[i])
		}
		if a.err != nil {
			return nil, false, a.err
		}
		ruling, proved, err := work.perpendicularRuling(ctx, axis)
		if err != nil {
			return nil, false, err
		}
		if !proved {
			continue
		}
		span, err := work.intersectRuling(ctx, axis, ruling, nil)
		if errors.Is(err, ErrConvergence) && budget.operations < budget.maxOperations {
			// The enclosure proof may still decide the point if this
			// particular normal-foot problem lacks an isolated witness.
			return nil, false, nil
		}
		if err != nil || span == nil || span.Start.Parameter.Sign() > 0 || span.End.Parameter.Sign() < 0 {
			return nil, true, err
		}
		witness, err := joinedSweepRulingWitness(owner, axis, span, fixed, omitted, budget)
		return witness, true, err
	}
	return nil, false, nil
}

func joinedSweepRulingWitness(owner, axis int, span *NormalOffsetSweepLineSpan, fixed, omitted uint8,
	budget *polynomialBudget) (*NormalOffsetSweepJoinedPointOwner, error) {
	witness := &NormalOffsetSweepJoinedPointOwner{Owner: owner, AlgebraicStation: span.AlgebraicStation}
	start, end := span.Start, span.End
	station, faces := start.Station, span.BoundaryFaces
	switch {
	case start.Parameter.Sign() == 0:
		faces = start.Faces
	case end.Parameter.Sign() == 0:
		station, faces = end.Station, end.Faces
	default:
		a := boundedRatArithmetic{b: budget}
		fraction := a.quo(a.sub(new(big.Rat), start.Parameter), a.sub(end.Parameter, start.Parameter))
		width := a.sub(end.Station[axis], start.Station[axis])
		station[axis] = a.add(start.Station[axis], a.mul(fraction, width))
		if a.err != nil {
			return nil, a.err
		}
	}
	for i, value := range station {
		if value != nil {
			witness.Parameters[i] = copySweepInterval(exactInterval{value, value})
			continue
		}
		cert := span.AlgebraicStation
		if cert == nil || (i != 2 && i != cert.ProfileAxis) {
			return nil, ErrConvergence
		}
		bounds := cert.ProfileParameter
		if i == 2 {
			bounds = cert.SignedDisplacement
		}
		witness.Parameters[i] = copySweepInterval(exactInterval{bounds.Lower, bounds.Upper})
	}
	var incident uint8
	for _, side := range faces {
		incident |= 1 << side
		if omitted&(1<<side) == 0 {
			witness.Faces = append(witness.Faces, side)
		}
	}
	if incident&omitted != fixed {
		return nil, ErrConvergence
	}
	return witness, nil
}
