package geom

import (
	"context"
	"math/big"
)

func (w *sweepBoundaryDistanceWork) affine(ctx context.Context, p *SurfacePatch) (bool, error) {
	a := boundedRatArithmetic{b: w.budget}
	u, v, affine, err := sweepHomogeneousAffineSpans(ctx, p, &a)
	if err != nil || !affine {
		return false, err
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
		return false, a.err
	}
	normal, err := w.evaluateOffset(ctx, jet, big.NewRat(1, 1))
	if err != nil {
		return false, err
	}
	span := a.sub(w.original[2].hi, w.original[2].lo)
	for i, n := range normal {
		if n.lo.Cmp(n.hi) != 0 {
			// An affine chart with an irrational unit normal can
			// use the general directed boundary search.
			return false, a.err
		}
		frame[2][i] = a.mul(span, n.lo)
		origin[i] = a.add(origin[i], a.mul(w.original[2].lo, n.lo))
	}
	if a.err != nil {
		return false, a.err
	}
	for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
		if err := ctx.Err(); err != nil {
			return true, err
		}
		if w.result.VisitedCells >= w.options.MaxCells {
			return true, ErrConvergence
		}
		w.result.VisitedCells++
		if err := w.affineFace(side, origin, frame); err != nil {
			return true, err
		}
		if w.result.Squared.Upper.Sign() == 0 {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return true, err
	}
	w.result.Squared.Lower = new(big.Rat).Set(w.result.Squared.Upper)
	w.result.Complete = true
	return true, nil
}

func (w *sweepBoundaryDistanceWork) affineFace(side NormalOffsetSweepSide, origin [3]*big.Rat, frame [3][3]*big.Rat) error {
	a := boundedRatArithmetic{b: w.budget}
	fixed := int(side) / 2
	free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[fixed]
	u, v := frame[free[0]], frame[free[1]]
	zero, one := new(big.Rat), big.NewRat(1, 1)
	on := big.NewRat(int64(int(side)%2), 1)
	var delta, base [3]*big.Rat
	for i := range delta {
		base[i] = a.add(origin[i], a.mul(on, frame[fixed][i]))
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
		return w.recordAffineFace(side, free, on, base, u, v, s, t, &a)
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
		if err := w.recordAffineFace(side, free, on, base, u, v, end, t, &a); err != nil {
			return err
		}
		s := clamp(a.quo(a.sub(up, a.mul(end, uv)), uu))
		if err := w.recordAffineFace(side, free, on, base, u, v, s, end, &a); err != nil {
			return err
		}
	}
	return nil
}

func (w *sweepBoundaryDistanceWork) recordAffineFace(side NormalOffsetSweepSide, free [2]int, on *big.Rat,
	base, u, v [3]*big.Rat, s, t *big.Rat, a *boundedRatArithmetic) error {
	var parameters [3]*big.Rat
	parameters[int(side)/2], parameters[free[0]], parameters[free[1]] = on, s, t
	var box [3]exactInterval
	for i := range parameters {
		p := a.add(base[i], a.add(a.mul(s, u[i]), a.mul(t, v[i])))
		box[i] = exactInterval{p, p}
		d := w.original[i]
		parameters[i] = a.add(d.lo, a.mul(parameters[i], a.sub(d.hi, d.lo)))
	}
	if a.err != nil {
		return a.err
	}
	return w.record(side, parameters, box)
}
