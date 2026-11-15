package fillet

import (
	"fmt"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func mixedFilletArcPair(in, out trimmed.ExactCircularLoopSegment, radius float64) (mixedCornerRound, error) {
	var zero mixedCornerRound
	orientation := func(s trimmed.ExactCircularLoopSegment) int {
		return mixedFilletCross(mixedFilletPoint(s.Start, s.Center), mixedFilletPoint(s.End, s.Center)).Sign()
	}
	source := [2]trimmed.ExactCircularLoopSegment{in, out}
	a, b := orientation(in), orientation(out)
	corner := out.Start
	turn := a * b * mixedFilletCross(mixedFilletPoint(corner, in.Center), mixedFilletPoint(corner, out.Center)).Sign()
	if turn == 0 {
		return zero, fmt.Errorf("%w: incident circular spans do not form a corner", ErrInvalid)
	}
	r := num(radius)
	var locus2 [2]q
	var scales [2]q
	for i, sense := range [2]int{a, b} {
		s := num(source[i].Radius)
		offset := qs(s, qm(r, big.NewRat(int64(turn*sense), 1)))
		if offset.Sign() <= 0 {
			return zero, fmt.Errorf("%w: internal round consumes the original circle radius", ErrRadius)
		}
		locus2[i] = qm(offset, offset)
		scales[i] = new(big.Rat).Quo(s, offset)
	}
	d := mixedFilletPoint(out.Center, in.Center)
	den := mixedFilletDot(d, d)
	if den.Sign() == 0 {
		return zero, ErrRadius
	}
	// The candidate center is C0 + h*d + v*left(d). Orthogonal
	// projection onto the center line gives h; v is the only root.
	h := new(big.Rat).Quo(qa(qs(locus2[0], locus2[1]), den), qm(big.NewRat(2, 1), den))
	v2 := qs(new(big.Rat).Quo(locus2[0], den), qm(h, h))
	if v2.Sign() < 0 {
		return zero, ErrRadius
	}
	v, ok := circularFilletRoot(v2)
	if !ok {
		return zero, geom.ErrUnrepresentable
	}
	centers := [2][2]q{
		{num(in.Center.X), num(in.Center.Y)},
		{num(out.Center.X), num(out.Center.Y)},
	}
	var chosen mixedCornerRound
	unrepresented := false
	for _, sign := range []int64{1, -1} {
		if sign < 0 && v.Sign() == 0 {
			break
		}
		cross := qm(v, big.NewRat(sign, 1))
		point := [2]q{
			qs(qa(centers[0][0], qm(h, d[0])), qm(cross, d[1])),
			qa(qa(centers[0][1], qm(h, d[1])), qm(cross, d[0])),
		}
		center, exact := exactOrthogonalPoint(point[0], point[1])
		var tangencies [2]geom.Vec2
		for i, c := range centers {
			p, ok := exactOrthogonalPoint(qa(c[0], qm(scales[i], qs(point[0], c[0]))), qa(c[1], qm(scales[i], qs(point[1], c[1]))))
			exact = exact && ok
			tangencies[i] = p
		}
		if !exact {
			unrepresented = true
			continue
		}
		a, b := tangencies[0], tangencies[1]
		if a == corner || b == corner || mixedFilletCross(mixedFilletPoint(a, center), mixedFilletPoint(b, center)).Sign() != turn {
			continue
		}
		t0, t1 := in, out
		t0.End, t1.Start = a, b
		if _, _, _, err := trimmed.CircularLoopRestriction(in, t0); err != nil {
			continue
		}
		if _, _, _, err := trimmed.CircularLoopRestriction(out, t1); err != nil {
			continue
		}
		if chosen.radius != 0 {
			return zero, fmt.Errorf("%w: ambiguous finite circular tangent branches", ErrRadius)
		}
		chosen = mixedCornerRound{incoming: a, outgoing: b, center: center, radius: radius, reach2: circularFilletReach2(center, corner, r)}
	}
	if unrepresented {
		return zero, geom.ErrUnrepresentable
	}
	if chosen.radius == 0 {
		return zero, ErrRadius
	}
	return chosen, nil
}
