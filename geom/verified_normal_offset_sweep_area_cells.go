package geom

import (
	"context"
	"math/big"
)

func (w *directedSweepAreaWork) estimate(ctx context.Context, cell *directedSweepAreaCell) error {
	wall := cell.side < NormalOffsetSweepLower
	var d *offsetCellJacobian
	if wall {
		edge := SurfaceEdge(cell.side)
		h, err := offsetHomogeneousRangesOn(ctx, cell.patch, &edge, w.budget)
		if err != nil {
			return err
		}
		jet, err := offsetCartesianRangesFromHomogeneous(h, w.budget)
		if err != nil {
			return err
		}
		d, err = offsetCellJacobianFromCartesian(jet, w.budget)
		if err != nil {
			return err
		}
	} else {
		jet, err := offsetCartesianRanges(ctx, cell.patch, w.budget)
		if err != nil {
			return err
		}
		d, err = offsetCellJacobianFromCartesian(jet, w.budget)
		if err != nil {
			return err
		}
		// If both tangent derivatives in an axis vanish identically,
		// the cap density cannot vary along that axis.
		cell.flatCapAxes = 0
		if zeroSweepAreaVector(jet[1][1]) {
			if zeroSweepAreaVector(jet[2][0]) {
				cell.flatCapAxes |= 1
			}
			if zeroSweepAreaVector(jet[0][2]) {
				cell.flatCapAxes |= 2
			}
		}
	}
	if d == nil {
		// The source is regular by its sealed proof, but this hull
		// does not yet strictly separate its normal from zero.
		return nil
	}
	a := &d.arithmetic
	var area exactInterval
	rect := cell.patch.Domain()
	original := normalSweepParameterIntervals(rect, cell.displacement)
	measure := func(axis int) exactInterval {
		x := original[axis]
		return a.sub(sweepAreaSingleton(x.hi), sweepAreaSingleton(x.lo))
	}
	if wall {
		free := 1 - int(cell.side)/2
		speed, splitLayer, err := directedSweepWallAverageSpeed(d, free, cell.displacement)
		if err != nil {
			return err
		}
		cell.splitLayer = splitLayer
		area = a.mul(speed, a.mul(measure(free), measure(2)))
	} else {
		distance := cell.displacement.lo
		du, dv := directedSweepLayerTangent(d, 0, distance), directedSweepLayerTangent(d, 1, distance)
		speed, err := directedSweepAreaNorm(a, a.cross(du, dv))
		if err != nil {
			return err
		}
		area = a.mul(speed, a.mul(measure(0), measure(1)))
	}
	if a.err != nil {
		return a.err
	}
	if area.lo.Cmp(area.hi) != 0 {
		var err error
		area, err = roundSweepVolumeBounds(area, w.factor, w.budget)
		if err != nil {
			return err
		}
	}
	cell.area = area
	cell.width = new(big.Rat).Sub(area.hi, area.lo)
	return w.budget.check(cell.width)
}

func zeroSweepAreaVector(v [3]exactInterval) bool {
	for _, x := range v {
		if x.lo.Sign() != 0 || x.hi.Sign() != 0 {
			return false
		}
	}
	return true
}

func directedSweepLayerTangent(d *offsetCellJacobian, axis int, displacement *big.Rat) (out [3]exactInterval) {
	a := &d.arithmetic
	for i := range out {
		out[i] = a.add(d.tangent[axis][i], a.mul(sweepAreaSingleton(displacement), d.normalDerivative(i, axis)))
	}
	return out
}

func directedSweepWallAverageSpeed(d *offsetCellJacobian, axis int, displacement exactInterval) (exactInterval, bool, error) {
	a := &d.arithmetic
	var normalDerivative [3]exactInterval
	for i := range normalDerivative {
		normalDerivative[i] = d.normalDerivative(i, axis)
	}
	mid := exactSweepMidpoint(displacement)
	if err := a.b.check(mid); err != nil {
		return exactInterval{}, false, err
	}
	var norms [3]exactInterval
	for station, t := range [3]*big.Rat{displacement.lo, mid, displacement.hi} {
		var tangent [3]exactInterval
		for i, p := range d.tangent[axis] {
			tangent[i] = a.add(p, a.mul(sweepAreaSingleton(t), normalDerivative[i]))
		}
		var err error
		norms[station], err = directedSweepAreaNorm(a, tangent)
		if err != nil {
			return exactInterval{}, false, err
		}
	}
	// The wall cross product with the unit normal has the length of
	// its tangential factor. The norm of this vector is convex as
	// a function of displacement. Its midpoint and its endpoint
	// average bound the true average speed for every profile station.
	two := sweepAreaSingleton(big.NewRat(2, 1))
	endpointAverage := a.div(a.add(norms[0], norms[2]), two)
	lower, upper := norms[1].lo, endpointAverage.hi
	// Subdivide displacement when its separate convexity gap is
	// large enough to dominate spatial uncertainty. This only selects
	// a partition direction and has no role in enclosure validity.
	scalars := boundedRatArithmetic{b: a.b}
	bend := scalars.sub(endpointAverage.lo, norms[1].hi)
	uncertainty := scalars.sub(upper, lower)
	splitLayer := bend.Sign() > 0 && scalars.add(bend, bend).Cmp(uncertainty) >= 0
	if a.err != nil {
		return exactInterval{}, false, a.err
	}
	if scalars.err != nil {
		return exactInterval{}, false, scalars.err
	}
	if lower.Cmp(upper) > 0 {
		return exactInterval{}, false, ErrConvergence
	}
	return exactInterval{lower, upper}, splitLayer, nil
}

func directedSweepAreaNorm(a *offsetIntervalArithmetic, vector [3]exactInterval) (exactInterval, error) {
	squared := a.norm2(vector)
	if a.err != nil {
		return exactInterval{}, a.err
	}
	lo, _, err := rationalSqrtBounds(squared.lo, min(96, (a.b.maxBits-squared.lo.Num().BitLen())/2), a.b)
	if err != nil {
		return exactInterval{}, err
	}
	_, hi, err := rationalSqrtBounds(squared.hi, min(96, (a.b.maxBits-squared.hi.Num().BitLen())/2), a.b)
	if err != nil {
		return exactInterval{}, err
	}
	return exactInterval{lo, hi}, nil
}
