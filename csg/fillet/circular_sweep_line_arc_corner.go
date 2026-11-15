package fillet

import (
	"fmt"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func circularFilletRoot(x q) (q, bool) {
	if x.Sign() < 0 {
		return nil, false
	}
	a, b := new(big.Int).Sqrt(x.Num()), new(big.Int).Sqrt(x.Denom())
	if new(big.Int).Mul(a, a).Cmp(x.Num()) != 0 || new(big.Int).Mul(b, b).Cmp(x.Denom()) != 0 {
		return nil, false
	}
	return new(big.Rat).SetFrac(a, b), true
}

// mixedFilletLineArc intersects the directed line-offset locus with the
// oriented circle-offset locus. Exact signed tangency and finite support
// restrictions decide between the algebraic candidates.
func mixedFilletLineArc(in, out trimmed.ExactCircularLoopSegment, radius float64) (mixedCornerRound, error) {
	var zero mixedCornerRound
	line, arc, lineFirst := in, out, true
	if in.Radius > 0 {
		line, arc, lineFirst = out, in, false
	}
	if line.Radius != 0 || arc.Radius <= 0 {
		return zero, fmt.Errorf("%w: expected a line and circular arc", ErrInvalid)
	}
	dir, err := circularFilletUnitLine(line)
	if err != nil {
		return zero, err
	}
	normal := [2]q{qs(new(big.Rat), dir[1]), dir[0]}
	at := out.Start
	arcSense := mixedFilletCross(mixedFilletPoint(arc.Start, arc.Center), mixedFilletPoint(arc.End, arc.Center)).Sign()
	radial := mixedFilletPoint(at, arc.Center)
	turn := mixedFilletDot(dir, radial).Sign() * arcSense
	if !lineFirst {
		turn = -turn
	}
	if turn == 0 {
		return zero, fmt.Errorf("%w: line and arc do not form a corner", ErrInvalid)
	}
	r, originalRadius := num(radius), num(arc.Radius)
	locusRadius := qs(originalRadius, qm(r, big.NewRat(int64(turn*arcSense), 1)))
	if locusRadius.Sign() <= 0 {
		return zero, fmt.Errorf("%w: internal line-arc round consumes the original radius", ErrRadius)
	}
	coords := func(p geom.Vec2) [2]q { return [2]q{num(p.X), num(p.Y)} }
	a, c := coords(at), coords(arc.Center)
	toSupport := [2]q{qs(c[0], a[0]), qs(c[1], a[1])}
	along := mixedFilletDot(dir, toSupport)
	offset := qm(r, big.NewRat(int64(turn), 1))
	d := qs(offset, mixedFilletDot(normal, toSupport))
	delta2 := qs(qm(locusRadius, locusRadius), qm(d, d))
	if delta2.Sign() < 0 {
		return zero, ErrRadius
	}
	delta, ok := circularFilletRoot(delta2)
	if !ok {
		return zero, geom.ErrUnrepresentable
	}
	scale := new(big.Rat).Quo(originalRadius, locusRadius)
	var chosen mixedCornerRound
	unrepresented := false
	for _, side := range []int64{1, -1} {
		if side < 0 && delta.Sign() == 0 {
			break
		}
		t := qa(along, qm(delta, big.NewRat(side, 1)))
		tangentLine := [2]q{qa(a[0], qm(t, dir[0])), qa(a[1], qm(t, dir[1]))}
		center := [2]q{qa(tangentLine[0], qm(offset, normal[0])), qa(tangentLine[1], qm(offset, normal[1]))}
		tangentArc := [2]q{
			qa(c[0], qm(scale, qs(center[0], c[0]))),
			qa(c[1], qm(scale, qs(center[1], c[1]))),
		}
		p, okP := exactOrthogonalPoint(tangentLine[0], tangentLine[1])
		v, okV := exactOrthogonalPoint(tangentArc[0], tangentArc[1])
		o, okO := exactOrthogonalPoint(center[0], center[1])
		if !okP || !okV || !okO {
			unrepresented = true
			continue
		}
		if p == line.Start || p == line.End || v == arc.Start || v == arc.End {
			continue
		}
		left, right := p, v
		if !lineFirst {
			left, right = v, p
		}
		if mixedFilletCross(mixedFilletPoint(left, o), mixedFilletPoint(right, o)).Sign() != turn {
			continue
		}
		inPart, outPart := in, out
		inPart.End, outPart.Start = left, right
		if _, _, _, err := trimmed.CircularLoopRestriction(in, inPart); err != nil {
			continue
		}
		if _, _, _, err := trimmed.CircularLoopRestriction(out, outPart); err != nil {
			continue
		}
		if chosen.radius > 0 {
			return zero, fmt.Errorf("%w: ambiguous finite line-arc tangent branches", ErrRadius)
		}
		chosen = mixedCornerRound{incoming: left, outgoing: right, center: o, radius: radius, reach2: circularFilletReach2(o, at, r)}
	}
	if unrepresented {
		return zero, geom.ErrUnrepresentable
	}
	if chosen.radius == 0 {
		return zero, ErrRadius
	}
	return chosen, nil
}

// The solved center lies on the same radial ray as each exact tangent
// point. Its distance to that original circle increases monotonically with
// the angle from the tangent on each removed minor arc. The corner and the
// tangent points are therefore the farthest candidates; convexity covers
// any removed straight segment.
func circularFilletReach2(center, corner geom.Vec2, radius q) q {
	r2, d2 := qm(radius, radius), squaredDistance(center, corner)
	if d2.Cmp(r2) > 0 {
		return d2
	}
	return r2
}
