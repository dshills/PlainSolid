package geom

import (
	"context"
	"math/big"
)

type sweepBoundaryAffineFrame struct {
	axes     [3][3]*big.Rat
	origin   [3]*big.Rat
	original [3]exactInterval
}

func (w *sweepBoundaryDistanceWork) affine(ctx context.Context, p *SurfacePatch, original [3]exactInterval) (*sweepBoundaryAffineFrame, error) {
	a := boundedRatArithmetic{b: w.budget}
	u, v, affine, err := sweepHomogeneousAffineSpans(ctx, p, &a)
	if err != nil || !affine {
		return nil, err
	}
	h := p.net[0]
	var frame [3][3]*big.Rat
	var origin [3]*big.Rat
	jet := RationalSurfacePatchJet{Du: u, Dv: v}
	for i := range origin {
		jet.Point[i] = new(big.Rat)
		frame[0][i], frame[1][i], origin[i] = a.quo(u[i], h[3]), a.quo(v[i], h[3]), a.quo(h[i], h[3])
	}
	if a.err != nil {
		return nil, a.err
	}
	normal, err := w.evaluateOffset(ctx, jet, big.NewRat(1, 1))
	if err != nil {
		return nil, err
	}
	span := a.sub(original[2].hi, original[2].lo)
	for i, n := range normal {
		if n.lo.Cmp(n.hi) != 0 {
			// An affine chart with an irrational unit normal can
			// use the general directed boundary search.
			return nil, a.err
		}
		frame[2][i] = a.mul(span, n.lo)
		origin[i] = a.add(origin[i], a.mul(original[2].lo, n.lo))
	}
	if a.err != nil {
		return nil, a.err
	}
	return &sweepBoundaryAffineFrame{axes: frame, origin: origin, original: original}, nil
}

func (w *sweepBoundaryDistanceWork) affineFace(owner int, side NormalOffsetSweepSide, f *sweepBoundaryAffineFrame) error {
	a := boundedRatArithmetic{b: w.budget}
	fixed := int(side) / 2
	free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[fixed]
	u, v := f.axes[free[0]], f.axes[free[1]]
	zero, one := new(big.Rat), big.NewRat(1, 1)
	on := big.NewRat(int64(int(side)%2), 1)
	var delta, base [3]*big.Rat
	for i := range delta {
		base[i] = a.add(f.origin[i], a.mul(on, f.axes[fixed][i]))
		delta[i] = a.sub(w.point[i], base[i])
	}
	dot := func(x, y [3]*big.Rat) *big.Rat {
		return a.add(a.add(a.mul(x[0], y[0]), a.mul(x[1], y[1])), a.mul(x[2], y[2]))
	}
	uu, uv, vv := dot(u, u), dot(u, v), dot(v, v)
	up, vp := dot(u, delta), dot(v, delta)
	det := a.sub(a.mul(uu, vv), a.mul(uv, uv))
	if a.err != nil {
		return a.err
	}
	if det.Sign() <= 0 {
		return ErrConvergence
	}
	s := a.quo(a.sub(a.mul(vv, up), a.mul(uv, vp)), det)
	t := a.quo(a.sub(a.mul(uu, vp), a.mul(uv, up)), det)
	if a.err != nil {
		return a.err
	}
	if s.Sign() >= 0 && s.Cmp(one) <= 0 && t.Sign() >= 0 && t.Cmp(one) <= 0 {
		return w.recordAffineFace(owner, side, free, on, f.original, base, u, v, s, t, &a)
	}
	clamp := func(q *big.Rat) *big.Rat {
		if q.Sign() < 0 {
			return zero
		}
		if q.Cmp(one) > 0 {
			return one
		}
		return q
	}
	// A constrained minimum outside the face interior occurs on
	// an edge. Clamping the exact line projection includes vertices.
	for _, end := range [2]*big.Rat{zero, one} {
		t := clamp(a.quo(a.sub(vp, a.mul(end, uv)), vv))
		if err := w.recordAffineFace(owner, side, free, on, f.original, base, u, v, end, t, &a); err != nil {
			return err
		}
		s := clamp(a.quo(a.sub(up, a.mul(end, uv)), uu))
		if err := w.recordAffineFace(owner, side, free, on, f.original, base, u, v, s, end, &a); err != nil {
			return err
		}
	}
	return nil
}

func (w *sweepBoundaryDistanceWork) recordAffineFace(owner int, side NormalOffsetSweepSide, free [2]int, on *big.Rat, original [3]exactInterval,
	base, u, v [3]*big.Rat, s, t *big.Rat, a *boundedRatArithmetic) error {
	var parameters [3]*big.Rat
	parameters[int(side)/2], parameters[free[0]], parameters[free[1]] = on, s, t
	var box [3]exactInterval
	for i := range parameters {
		p := a.add(base[i], a.add(a.mul(s, u[i]), a.mul(t, v[i])))
		box[i] = exactInterval{p, p}
		d := original[i]
		parameters[i] = a.add(d.lo, a.mul(parameters[i], a.sub(d.hi, d.lo)))
	}
	if a.err != nil {
		return a.err
	}
	return w.record(owner, side, parameters, box)
}
