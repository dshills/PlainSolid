package geom

import (
	"context"
	"math/big"
)

func (w *sweepMomentWork) curvedCell(ctx context.Context, p *SurfacePatch) (sweepRawMoments, error) {
	var out sweepRawMoments
	d, err := offsetCellJacobianBounds(ctx, p, w.budget)
	if err != nil {
		return out, err
	}
	if d == nil {
		return w.coarseCell(p)
	}
	a := &d.arithmetic
	var normal, nu, nv, point [3]exactInterval
	for i := range normal {
		normal[i] = a.div(d.normal[i], d.length)
		nu[i], nv[i] = d.normalDerivative(i, 0), d.normalDerivative(i, 1)
		r := w.reference[i]
		point[i] = a.sub(d.point[i], exactInterval{r, r})
	}
	x, y := a.cross(nu, d.tangent[1]), a.cross(d.tangent[0], nv)
	for i := range x {
		x[i] = a.add(x[i], y[i])
	}
	jacobian := [3]exactInterval{
		a.mul(exactInterval{d.scale, d.scale}, d.length),
		a.dot(x, normal),
		a.dot(a.cross(nu, nv), normal),
	}
	uv := normalSweepParameterIntervals(p.Domain(), exactInterval{})
	width := func(s exactInterval) exactInterval {
		return a.sub(exactInterval{s.hi, s.hi}, exactInterval{s.lo, s.lo})
	}
	area := a.mul(width(uv[0]), width(uv[1]))
	var weighted [3]exactInterval
	for i := range weighted {
		v := offsetZeroInterval()
		for j, c := range jacobian {
			m := w.displacement[i+j]
			if m.Sign() != 0 {
				v = a.add(v, a.mul(c, exactInterval{m, m}))
			}
		}
		if w.sweep.orientation < 0 {
			v = a.sub(offsetZeroInterval(), v)
		}
		weighted[i] = a.mul(v, area)
	}
	if a.err != nil {
		return out, a.err
	}
	// The absolute Jacobian and t² are nonnegative. Keeping these
	// signs tightens their use as exact displacement moment weights.
	for _, i := range []int{0, 2} {
		if weighted[i].hi.Sign() < 0 {
			return out, ErrConvergence
		}
		weighted[i].lo = maxSweepMomentZero(weighted[i].lo)
	}

	out[0] = weighted[0]
	var hull [3]exactInterval
	displacement := exactInterval{w.sweep.lower, w.sweep.upper}
	for i := range point {
		hull[i] = a.add(point[i], a.mul(displacement, normal[i]))
		polynomial := a.add(a.mul(point[i], weighted[0]), a.mul(normal[i], weighted[1]))
		out[1+i] = sweepMomentIntersection(polynomial, a.mul(hull[i], weighted[0]))
	}
	for k, axes := range sweepMomentTensorAxes {
		i, j := axes[0], axes[1]
		constant := a.mul(sweepMomentProduct(a, point, i, j), weighted[0])
		linear := a.mul(a.add(a.mul(point[i], normal[j]), a.mul(point[j], normal[i])), weighted[1])
		quadratic := a.mul(sweepMomentProduct(a, normal, i, j), weighted[2])
		polynomial := a.add(a.add(constant, linear), quadratic)
		out[4+k] = sweepMomentIntersection(polynomial, a.mul(sweepMomentProduct(a, hull, i, j), weighted[0]))
	}
	if a.err != nil {
		return out, a.err
	}
	for _, q := range out {
		if q.lo.Cmp(q.hi) > 0 {
			return out, ErrConvergence
		}
	}
	return out, nil
}

func (w *sweepMomentWork) coarseCell(p *SurfacePatch) (sweepRawMoments, error) {
	var out sweepRawMoments
	a := offsetIntervalArithmetic{b: w.budget}
	s := boundedRatArithmetic{b: w.budget}
	var hull [3]exactInterval
	upperVolume := big.NewRat(1, 1)
	for i := range hull {
		lo := new(big.Rat).SetFloat64(coordinate(p.bounds.Min, i))
		hi := new(big.Rat).SetFloat64(coordinate(p.bounds.Max, i))
		if lo == nil || hi == nil {
			return out, ErrConvergence
		}
		hull[i] = exactInterval{s.sub(s.sub(lo, w.reference[i]), w.extension), s.add(s.sub(hi, w.reference[i]), w.extension)}
		upperVolume = s.mul(upperVolume, s.sub(hull[i].hi, hull[i].lo))
	}
	if s.err != nil {
		return out, s.err
	}
	// A verified sweep is embedded. A hull bounds the physical volume
	// over this cell even when the interval normal is unresolved.
	out[0] = exactInterval{new(big.Rat), upperVolume}
	for i := range hull {
		out[1+i] = a.mul(hull[i], out[0])
	}
	for k, axes := range sweepMomentTensorAxes {
		out[4+k] = a.mul(sweepMomentProduct(&a, hull, axes[0], axes[1]), out[0])
	}
	return out, a.err
}

func sweepMomentProduct(a *offsetIntervalArithmetic, q [3]exactInterval, i, j int) exactInterval {
	if i == j {
		return a.square(q[i])
	}
	return a.mul(q[i], q[j])
}

func sweepMomentIntersection(a, b exactInterval) exactInterval {
	if b.lo.Cmp(a.lo) > 0 {
		a.lo = b.lo
	}
	if b.hi.Cmp(a.hi) < 0 {
		a.hi = b.hi
	}
	return a
}

// affineCell verifies the original homogeneous net and integrates its
// uniform parallelepiped exactly when the normal length is rational.
func (w *sweepMomentWork) affineCell(ctx context.Context, p *SurfacePatch) (out sweepRawMoments, affine bool, err error) {
	s := boundedRatArithmetic{b: w.budget}
	h := p.net[0]
	for _, pole := range p.net {
		if pole[3].Cmp(h[3]) != 0 {
			return out, false, nil
		}
	}
	var u, v [3]*big.Rat
	for i := range u {
		u[i] = s.sub(p.net[p.degreeU*p.countV][i], h[i])
		v[i] = s.sub(p.net[p.degreeV][i], h[i])
	}
	for i := 0; i <= p.degreeU; i++ {
		if err := ctx.Err(); err != nil {
			return out, false, err
		}
		t := big.NewRat(int64(i), int64(p.degreeU))
		for j := 0; j <= p.degreeV; j++ {
			r := big.NewRat(int64(j), int64(p.degreeV))
			for k := range u {
				expect := s.add(h[k], s.add(s.mul(t, u[k]), s.mul(r, v[k])))
				if s.err != nil {
					return out, false, s.err
				}
				if p.net[i*p.countV+j][k].Cmp(expect) != 0 {
					return out, false, nil
				}
			}
		}
	}
	var first, second, origin [3]exactInterval
	asInterval := func(q *big.Rat) exactInterval { return exactInterval{q, q} }
	for i := range u {
		first[i] = asInterval(s.quo(u[i], h[3]))
		second[i] = asInterval(s.quo(v[i], h[3]))
		origin[i] = asInterval(s.sub(s.quo(h[i], h[3]), w.reference[i]))
	}
	if s.err != nil {
		return out, false, s.err
	}
	a := offsetIntervalArithmetic{b: w.budget}
	n := a.cross(first, second)
	if a.err != nil {
		return out, false, a.err
	}
	scale := new(big.Rat)
	for _, q := range n {
		if magnitude := new(big.Rat).Abs(q.lo); magnitude.Cmp(scale) > 0 {
			scale = magnitude
		}
	}
	if scale.Sign() <= 0 {
		return out, false, ErrConvergence
	}
	for i := range n {
		n[i] = a.div(n[i], asInterval(scale))
	}
	length2 := a.norm2(n)
	if a.err != nil {
		return out, false, a.err
	}
	low, high, err := rationalSqrtBounds(length2.lo, max(96, w.precision), w.budget)
	if err != nil {
		return out, false, err
	}
	length := exactInterval{low, high}
	thickness := asInterval(w.displacement[0])
	midpoint := s.quo(s.add(w.sweep.lower, w.sweep.upper), big.NewRat(2, 1))
	if s.err != nil {
		return out, false, s.err
	}
	out[0] = a.mul(a.mul(length, asInterval(scale)), thickness)
	half, oneTwelfth := asInterval(big.NewRat(1, 2)), asInterval(big.NewRat(1, 12))
	var center, direction [3]exactInterval
	for i := range n {
		unit := a.div(n[i], length)
		direction[i] = a.mul(unit, thickness)
		center[i] = a.add(a.add(origin[i], a.mul(a.add(first[i], second[i]), half)), a.mul(unit, asInterval(midpoint)))
		out[1+i] = a.mul(center[i], out[0])
	}
	for k, axes := range sweepMomentTensorAxes {
		i, j := axes[0], axes[1]
		shape := a.add(a.add(sweepMomentProduct(&a, first, i, j), sweepMomentProduct(&a, second, i, j)),
			sweepMomentProduct(&a, direction, i, j))
		raw := a.add(sweepMomentProduct(&a, center, i, j), a.mul(shape, oneTwelfth))
		out[4+k] = a.mul(raw, out[0])
	}
	return out, true, a.err
}
