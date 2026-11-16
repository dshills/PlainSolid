package geom

import (
	"context"
	"errors"
	"math/big"
)

func sweepBoundaryDistanceBox(ctx context.Context, p *SurfacePatch, side NormalOffsetSweepSide, t exactInterval,
	b *polynomialBudget) (box [3]exactInterval, err error) {
	source, homogeneousU, homogeneousV, err := sweepBoundaryJetRanges(ctx, p, side, b)
	if err != nil {
		return box, err
	}
	a := offsetIntervalArithmetic{b: b}
	var du, dv [3]exactInterval
	for i := range du {
		// For Cartesian tangents, the homogeneous quotient contributes
		// one common positive factor 1/W. It cancels on normalization.
		du[i] = a.sub(homogeneousU[i], a.mul(source[i], homogeneousU[3]))
		dv[i] = a.sub(homogeneousV[i], a.mul(source[i], homogeneousV[3]))
	}
	normal := a.cross(du, dv)
	if a.err != nil {
		return box, a.err
	}
	length, normalErr := scaledOffsetNormalLength(&normal, &a)
	if normalErr != nil && (!errors.Is(normalErr, ErrConvergence) || b.operations >= b.maxOperations) {
		return box, normalErr
	}
	unit := exactInterval{big.NewRat(-1, 1), big.NewRat(1, 1)}
	for i := range box {
		direction := unit
		if normalErr == nil {
			direction = a.div(normal[i], length)
			if direction.lo.Cmp(unit.lo) < 0 {
				direction.lo = unit.lo
			}
			if direction.hi.Cmp(unit.hi) > 0 {
				direction.hi = unit.hi
			}
		}
		// Even when a coarse box cannot separate the normal from zero,
		// its original point and the unit-vector box give a safe bound.
		box[i] = a.add(source[i], a.mul(t, direction))
	}
	return box, a.err
}

func sweepBoundaryJetRanges(ctx context.Context, p *SurfacePatch, side NormalOffsetSweepSide,
	b *polynomialBudget) (source [3]exactInterval, du, dv [4]exactInterval, err error) {
	domain := p.Domain()
	countU, countV := p.countU, p.countV
	controls := make([]*big.Rat, len(p.net))
	exact := boundedRatArithmetic{b: b}
	for axis := range 4 {
		if err := ctx.Err(); err != nil {
			return source, du, dv, err
		}
		for i, pole := range p.net {
			controls[i] = pole[axis]
			if err := b.check(controls[i]); err != nil {
				return source, du, dv, err
			}
		}
		u, err := scalarPartial(controls, countU, countV, true, domain, b)
		if err != nil {
			return source, du, dv, err
		}
		v, err := scalarPartial(controls, countU, countV, false, domain, b)
		if err != nil {
			return source, du, dv, err
		}
		du[axis] = sweepBoundaryRange(u, countU-1, countV, side)
		dv[axis] = sweepBoundaryRange(v, countU, countV-1, side)
		if axis < 3 {
			first, last, step := sweepBoundaryRangeIndices(countU, countV, side)
			var hull exactInterval
			for i := first; i < last; i += step {
				r := exact.quo(controls[i], p.net[i][3])
				if exact.err != nil {
					return source, du, dv, exact.err
				}
				if hull.lo == nil || r.Cmp(hull.lo) < 0 {
					hull.lo = r
				}
				if hull.hi == nil || r.Cmp(hull.hi) > 0 {
					hull.hi = r
				}
			}
			source[axis] = hull
		}
	}
	return source, du, dv, nil
}

func sweepBoundaryRange(controls []*big.Rat, nu, nv int, side NormalOffsetSweepSide) exactInterval {
	first, last, step := sweepBoundaryRangeIndices(nu, nv, side)
	r := exactInterval{controls[first], controls[first]}
	for i := first + step; i < last; i += step {
		if x := controls[i]; x.Cmp(r.lo) < 0 {
			r.lo = x
		} else if x.Cmp(r.hi) > 0 {
			r.hi = x
		}
	}
	return r
}

func sweepBoundaryRangeIndices(nu, nv int, side NormalOffsetSweepSide) (first, last, step int) {
	switch side {
	case NormalOffsetSweepUMin:
		return 0, nv, 1
	case NormalOffsetSweepUMax:
		return (nu - 1) * nv, nu * nv, 1
	case NormalOffsetSweepVMin:
		return 0, nu * nv, nv
	case NormalOffsetSweepVMax:
		return nv - 1, nu * nv, nv
	default:
		return 0, nu * nv, 1
	}
}
