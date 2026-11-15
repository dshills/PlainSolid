package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// CircularLoopRestriction proves that child is a positive directed
// subinterval of parent on the same exact straight or circular support.
// The returned rational map evaluates the original parameter using
// (Map[0][0]+Map[0][1]*t)/(Map[1][0]+Map[1][1]*t) on child parameter [0,1].
// The caller owns the returned coefficients.
func CircularLoopRestriction(parent, child ExactCircularLoopSegment) (lo, hi *big.Rat, m [2][2]*big.Rat, err error) {
	for _, s := range []ExactCircularLoopSegment{parent, child} {
		if s.Start == s.End || !uvFinite(s.Start) || !uvFinite(s.End) ||
			!uvFinite(s.Center) || !finite(s.Radius) || s.Radius < 0 {
			return nil, nil, m, geom.ErrInvalid
		}
		if _, err := exactCircularLoopControls(s, true); err != nil {
			return nil, nil, m, err
		}
	}
	if child.Radius != parent.Radius || parent.Radius > 0 && child.Center != parent.Center {
		return nil, nil, m, geom.ErrDomain
	}
	a, aok := circularSweepOverlayParameter(parent, child.Start)
	b, bok := circularSweepOverlayParameter(parent, child.End)
	if !aok || !bok || a.Sign() < 0 || a.Cmp(b) >= 0 || b.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, nil, m, geom.ErrDomain
	}
	m, err = circularSweepOverlayLaw(parent, child, a, b)
	if err != nil {
		return nil, nil, [2][2]*big.Rat{}, err
	}
	return a, b, m, nil
}
