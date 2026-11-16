package geom

import (
	"context"
	"errors"
	"math/big"
)

// joinedSweepWallPoint applies only after the exact model point has been
// proved in the join plane. This original boundary curve and its original
// unit normal both lie in that plane. Thus a vector from the curve to the
// query is parallel to the surface normal precisely when it is orthogonal
// to the regular edge tangent. The sealed sweep's injectivity gives at most
// one such point with displacement in the allowed interval.
func joinedSweepWallPoint(ctx context.Context, owner int, sweep *VerifiedNormalOffsetSweep, point [3]*big.Rat,
	side NormalOffsetSweepSide, budget *polynomialBudget) (*NormalOffsetSweepJoinedPointOwner, bool, error) {
	fixedAxis := int(side) / 2
	profileAxis := 1 - fixedAxis
	degrees := [2]int{sweep.source.degreeU, sweep.source.degreeV}
	if degrees[profileAxis] > 3 {
		return nil, false, nil
	}
	work := sweepLineWork{
		sweep: sweep, origin: point, budget: budget,
		original: normalSweepParameterIntervals(sweep.source.Domain(), exactInterval{sweep.lower, sweep.upper}),
	}
	p, err := work.wallNormalFootPolynomials(ctx, SurfaceEdge(side), profileAxis)
	if err != nil {
		return nil, false, err
	}
	root, sign, exactT, err := work.findNormalFoot(ctx, p, 1)
	if errors.Is(err, ErrConvergence) && budget.operations < budget.maxOperations {
		return nil, false, nil
	}
	if root == nil || err != nil {
		return nil, true, err
	}
	normalized, profile, exactProfile, err := work.normalFootProfileBounds(ctx, root, p, profileAxis)
	if err != nil {
		return nil, true, err
	}
	profileDomain := work.original[profileAxis]
	if exactProfile != nil {
		a := boundedRatArithmetic{b: budget}
		t := a.quo(a.sub(exactProfile, profileDomain.lo), a.sub(profileDomain.hi, profileDomain.lo))
		if a.err != nil {
			return nil, true, a.err
		}
		normalized = exactInterval{t, t}
	}
	shift, err := work.encloseRulingShift(p, normalized, sign, exactT)
	if err != nil {
		return nil, true, err
	}
	wallStation := work.original[fixedAxis].lo
	if side == NormalOffsetSweepUMax || side == NormalOffsetSweepVMax {
		wallStation = work.original[fixedAxis].hi
	}
	witness := &NormalOffsetSweepJoinedPointOwner{Owner: owner}
	witness.Parameters[fixedAxis] = copySweepInterval(exactInterval{wallStation, wallStation})
	witness.Parameters[profileAxis] = copySweepInterval(profile)
	witness.Parameters[2] = copySweepInterval(shift)
	if profile.lo.Cmp(profile.hi) != 0 || shift.lo.Cmp(shift.hi) != 0 {
		witness.AlgebraicStation = work.normalFootCertificate(p, profileAxis, normalized, profile, shift)
	}
	// Endpoint roots are isolated as singletons. All other roots
	// are strictly inside the original edge even if a bound touches
	// the domain. Cap equality was proved at the isolated root.
	if exactProfile != nil {
		switch {
		case exactProfile.Cmp(profileDomain.lo) == 0:
			witness.Faces = append(witness.Faces, NormalOffsetSweepSide(profileAxis*2))
		case exactProfile.Cmp(profileDomain.hi) == 0:
			witness.Faces = append(witness.Faces, NormalOffsetSweepSide(profileAxis*2+1))
		}
	}
	if exactT != nil {
		switch {
		case exactT.Cmp(sweep.lower) == 0:
			witness.Faces = append(witness.Faces, NormalOffsetSweepLower)
		case exactT.Cmp(sweep.upper) == 0:
			witness.Faces = append(witness.Faces, NormalOffsetSweepUpper)
		}
	}
	return witness, true, nil
}

func (w *sweepLineWork) wallNormalFootPolynomials(ctx context.Context, side SurfaceEdge, profileAxis int) (p sweepLineRulingPolynomials, err error) {
	b := w.budget
	source := w.sweep.source
	jet, err := offsetEdgePowers(ctx, source, side, false, nil, b)
	if err != nil {
		return p, err
	}
	p.weight = jet[0][3]
	indices := source.edgeIndices(side)
	weights := make([]*big.Rat, len(indices))
	for i, index := range indices {
		weights[i] = source.net[index][3]
	}
	p.weightHull = coefficientRange(weights)
	var residual, tangent [3]rationalPolynomial
	freeJet := jet[profileAxis+1]
	for i := range residual {
		qw, err := polynomialProduct(p.weight, rationalPolynomial{w.origin[i]}, b)
		if err != nil {
			return p, err
		}
		residual[i], err = graphPolynomialSum(qw, jet[0][i], -1, b)
		if err != nil {
			return p, err
		}
		// The Cartesian tangent has numerator H'_i W - H_i W'.
		tangent[i], err = offsetPolynomialDet(freeJet[i], freeJet[3], jet[0][i], p.weight, b)
		if err != nil {
			return p, err
		}
	}
	normal, err := offsetNormalPowers(ctx, jet, b)
	if err != nil {
		return p, err
	}
	p.normalFoot, err = sweepLinePolynomialDot(residual, tangent, b)
	if err != nil {
		return p, err
	}
	p.direction, err = sweepLinePolynomialDot(residual, normal, b)
	if err != nil {
		return p, err
	}
	p.squared, err = sweepLinePolynomialDot(residual, residual, b)
	return p, err
}
