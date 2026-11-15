package fillet

import (
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func circularFilletUnitLine(s trimmed.ExactCircularLoopSegment) ([2]q, error) {
	d := mixedFilletPoint(s.End, s.Start)
	length, ok := circularFilletRoot(mixedFilletDot(d, d))
	if !ok {
		return [2]q{}, geom.ErrUnrepresentable
	}
	if length.Sign() == 0 {
		return [2]q{}, ErrInvalid
	}
	return [2]q{new(big.Rat).Quo(d[0], length), new(big.Rat).Quo(d[1], length)}, nil
}

// mixedFilletObliqueLines intersects the signed parallel offsets, then
// projects onto each original line. The source chart may have oblique
// rational unit directions even though those directions are not binary64.
func mixedFilletObliqueLines(in, out trimmed.ExactCircularLoopSegment, radius float64) (mixedCornerRound, error) {
	var zero mixedCornerRound
	u, err := circularFilletUnitLine(in)
	if err != nil {
		return zero, err
	}
	v, err := circularFilletUnitLine(out)
	if err != nil {
		return zero, err
	}
	den := mixedFilletCross(u, v)
	sgn := den.Sign()
	if sgn == 0 {
		return zero, ErrInvalid
	}
	r := num(radius)
	sr := qm(r, big.NewRat(int64(sgn), 1))
	o := [2]q{qs(new(big.Rat), qm(sr, u[1])), qm(sr, u[0])}
	p := [2]q{qs(new(big.Rat), qm(sr, v[1])), qm(sr, v[0])}
	d := [2]q{qs(p[0], o[0]), qs(p[1], o[1])}
	t := new(big.Rat).Quo(mixedFilletCross(d, v), den)
	c := [2]q{qa(qa(num(out.Start.X), o[0]), qm(t, u[0])), qa(qa(num(out.Start.Y), o[1]), qm(t, u[1]))}
	center, hasCenter := exactOrthogonalPoint(c[0], c[1])
	left, hasLeft := exactOrthogonalPoint(qs(c[0], o[0]), qs(c[1], o[1]))
	right, hasRight := exactOrthogonalPoint(qs(c[0], p[0]), qs(c[1], p[1]))
	if !hasCenter || !hasLeft || !hasRight {
		return zero, geom.ErrUnrepresentable
	}
	if left == out.Start || right == out.Start || mixedFilletCross(mixedFilletPoint(left, center), mixedFilletPoint(right, center)).Sign() != sgn {
		return zero, ErrRadius
	}
	a, b := in, out
	a.End, b.Start = left, right
	if _, _, _, err := trimmed.CircularLoopRestriction(in, a); err != nil {
		return zero, ErrRadius
	}
	if _, _, _, err := trimmed.CircularLoopRestriction(out, b); err != nil {
		return zero, ErrRadius
	}
	return mixedCornerRound{incoming: left, outgoing: right, center: center, radius: radius, reach2: circularFilletReach2(center, out.Start, r)}, nil
}
