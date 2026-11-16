package geom

import (
	"context"
	"math/big"
)

func (w *sweepLineWork) intersectExtrusion(ctx context.Context, seed *big.Rat) (*NormalOffsetSweepLineSpan, error) {
	for axis := range 2 {
		ruling, ok, err := w.perpendicularRuling(ctx, axis)
		if err != nil {
			return nil, err
		}
		if ok {
			return w.intersectRuling(ctx, axis, ruling, seed)
		}
	}
	return nil, ErrDomain
}

// perpendicularRuling proves the original homogeneous net is
// w_i*(C_i+v*D,1), with C_i in one plane perpendicular to D. The
// original degree and free knots are retained, including rational weights.
func (w *sweepLineWork) perpendicularRuling(ctx context.Context, axis int) (ruling [3]*big.Rat, proved bool, err error) {
	p := w.sweep.source
	ruledCount, profileCount := p.countU, p.countV
	stride, step := p.countV, 1
	if axis == 1 {
		ruledCount, profileCount = p.countV, p.countU
		stride, step = 1, p.countV
	}
	p0, p1 := p.net[0], p.net[(ruledCount-1)*stride]
	if p0[3].Cmp(p1[3]) != 0 {
		return ruling, false, nil
	}
	a := boundedRatArithmetic{b: w.budget}
	var base [3]*big.Rat
	for i := range ruling {
		base[i] = a.quo(p0[i], p0[3])
		ruling[i] = a.quo(a.sub(p1[i], p0[i]), p0[3])
	}
	parallel := sweepLineParallel(&a, ruling, w.direction)
	if a.err != nil || !parallel || sweepLineZero(ruling) {
		return ruling, false, a.err
	}
	for profile := 0; profile < profileCount; profile++ {
		if err := ctx.Err(); err != nil {
			return ruling, false, err
		}
		h := p.net[profile*step]
		var delta, increment [3]*big.Rat
		for i := range delta {
			delta[i] = a.sub(h[i], a.mul(base[i], h[3]))
			increment[i] = a.mul(ruling[i], h[3])
		}
		projection := sweepLineDot(&a, ruling, delta)
		if a.err != nil || projection.Sign() != 0 {
			return ruling, false, a.err
		}
		for j := 0; j < ruledCount; j++ {
			pole := p.net[profile*step+j*stride]
			if pole[3].Cmp(h[3]) != 0 {
				return ruling, false, nil
			}
			n := big.NewRat(int64(j), int64(ruledCount-1))
			for i, d := range increment {
				expect := a.add(h[i], a.mul(d, n))
				if a.err != nil || expect.Cmp(pole[i]) != 0 {
					return ruling, false, a.err
				}
			}
		}
	}
	return ruling, true, nil
}

func (w *sweepLineWork) intersectRuling(ctx context.Context, axis int, ruling [3]*big.Rat,
	seed *big.Rat) (*NormalOffsetSweepLineSpan, error) {
	profile := w.original[1-axis]
	probes := []*big.Rat{exactSweepMidpoint(profile), profile.lo, profile.hi}
	if seed != nil {
		if seed.Cmp(profile.lo) < 0 || seed.Cmp(profile.hi) > 0 {
			return nil, ErrDomain
		}
		probes = []*big.Rat{seed}
	}
	for _, station := range probes {
		span, err := w.tryRulingStation(ctx, axis, ruling, station)
		if span != nil || err != nil {
			return span, err
		}
	}
	miss, err := w.rulingHullMiss(ctx, ruling)
	if err != nil || miss {
		return nil, err
	}
	if seed != nil {
		return nil, ErrConvergence
	}
	return w.intersectRulingAlgebraic(ctx, axis, ruling)
}

func (w *sweepLineWork) tryRulingStation(ctx context.Context, axis int, ruling [3]*big.Rat,
	station *big.Rat) (*NormalOffsetSweepLineSpan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := w.budget.check(station); err != nil {
		return nil, err
	}
	var uv [2]*big.Rat
	uv[axis], uv[1-axis] = w.original[axis].lo, station
	point, derivative, err := exactSurfaceJet(w.sweep.source, uv[0], uv[1], w.budget)
	if err != nil {
		return nil, err
	}
	a := boundedRatArithmetic{b: w.budget}
	var delta [3]*big.Rat
	for i := range delta {
		delta[i] = a.sub(w.origin[i], point[i])
	}
	norm2 := sweepLineDot(&a, ruling, ruling)
	if a.err != nil {
		return nil, a.err
	}
	along := a.quo(sweepLineDot(&a, delta, ruling), norm2)
	for i := range delta {
		delta[i] = a.sub(delta[i], a.mul(along, ruling[i]))
	}
	if a.err != nil {
		return nil, a.err
	}
	t := new(big.Rat)
	if !sweepLineZero(delta) {
		var u, v [3]*big.Rat
		for i, d := range derivative {
			u[i], v[i] = d[0], d[1]
		}
		n, rational, err := w.unitNormal(ctx, u, v)
		if err != nil || !rational {
			return nil, err
		}
		t = sweepLineDot(&a, delta, n)
		for i, d := range delta {
			x := a.mul(t, n[i])
			if a.err != nil || x.Cmp(d) != 0 {
				return nil, a.err
			}
		}
	}
	if t.Cmp(w.original[2].lo) < 0 || t.Cmp(w.original[2].hi) > 0 {
		return nil, nil
	}
	var at, rate [3]*big.Rat
	for i := range rate {
		rate[i] = new(big.Rat)
	}
	at[axis] = along
	rate[axis] = a.quo(sweepLineDot(&a, ruling, w.direction), norm2)
	normalize := func(i int, q *big.Rat) *big.Rat {
		original := w.original[i]
		return a.quo(a.sub(q, original.lo), a.sub(original.hi, original.lo))
	}
	at[1-axis], at[2] = normalize(1-axis, station), normalize(2, t)
	if a.err != nil {
		return nil, a.err
	}
	// At ruling station zero every normal fiber lies in the same
	// original plane perpendicular to D. Another transverse preimage
	// would produce the same full sweep point at that original ruling
	// station, contradicting the sealed sweep injectivity. Thus the
	// fixed transverse station and moving ruling are exhaustive.
	return w.clipUnitCube(at, rate)
}

func (w *sweepLineWork) rulingHullMiss(ctx context.Context, ruling [3]*big.Rat) (bool, error) {
	a := boundedRatArithmetic{b: w.budget}
	abs := func(q *big.Rat) *big.Rat { return new(big.Rat).Abs(q) }
	maxT := abs(w.original[2].lo)
	if h := abs(w.original[2].hi); h.Cmp(maxT) > 0 {
		maxT = h
	}
	zero := new(big.Rat)
	for axis := range ruling {
		i, j := (axis+1)%3, (axis+2)%3
		direction := [3]*big.Rat{zero, zero, zero}
		direction[i], direction[j] = ruling[j], a.sub(zero, ruling[i])
		if a.err != nil {
			return false, a.err
		}
		if sweepLineZero(direction) {
			continue
		}
		level := sweepLineDot(&a, direction, w.origin)
		// The L1 length bounds the projection of any unit normal.
		extension := a.mul(maxT, a.add(abs(direction[i]), abs(direction[j])))
		var lo, hi *big.Rat
		for _, h := range w.sweep.source.net {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			position := a.quo(sweepLineDot(&a, direction, [3]*big.Rat(h[:3])), h[3])
			if a.err != nil {
				return false, a.err
			}
			if lo == nil || position.Cmp(lo) < 0 {
				lo = position
			}
			if hi == nil || position.Cmp(hi) > 0 {
				hi = position
			}
		}
		lower, upper := a.sub(lo, extension), a.add(hi, extension)
		if a.err != nil {
			return false, a.err
		}
		if level.Cmp(lower) < 0 || level.Cmp(upper) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func sweepLineParallel(a *boundedRatArithmetic, x, y [3]*big.Rat) bool {
	for i := range x {
		j := (i + 1) % 3
		cross := a.sub(a.mul(x[i], y[j]), a.mul(x[j], y[i]))
		if cross.Sign() != 0 {
			return false
		}
	}
	return a.err == nil
}

func sweepLineZero(q [3]*big.Rat) bool {
	return q[0].Sign() == 0 && q[1].Sign() == 0 && q[2].Sign() == 0
}
