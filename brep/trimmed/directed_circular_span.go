package trimmed

import (
	"math/big"
	"sort"

	"plainsolid/geom"
)

// ExactDirectedCircularSpan describes a directed minor, half-turn, major,
// or full circle with exactly represented circle data. FullCircle is
// required to distinguish a complete turn from equal invalid endpoints.
type ExactDirectedCircularSpan struct {
	Start, End, Center    geom.Vec2
	Radius                float64
	Clockwise, FullCircle bool
}

// SplitExactDirectedCircularSpan divides the authored sweep at the original
// circle's traversed coordinate axes. Each returned segment is at most a
// quarter turn and can be used directly in an exact circular loop. Order,
// direction, original endpoints and circle support are preserved. A required
// split point that cannot be represented exactly is rejected.
func SplitExactDirectedCircularSpan(s ExactDirectedCircularSpan) ([]ExactCircularLoopSegment, error) {
	if !uvFinite(s.Start) || !uvFinite(s.End) || !uvFinite(s.Center) || !finite(s.Radius) ||
		s.Radius <= 0 || (s.Start == s.End) != s.FullCircle {
		return nil, geom.ErrInvalid
	}
	r := ratInset(s.Radius)
	r2 := new(big.Rat).Mul(r, r)
	c := sweepOffsetPointOf(s.Center)
	a, b := sweepOffsetSub(sweepOffsetPointOf(s.Start), c), sweepOffsetSub(sweepOffsetPointOf(s.End), c)
	if sweepOffsetDot(a, a).Cmp(r2) != 0 || sweepOffsetDot(b, b).Cmp(r2) != 0 {
		return nil, geom.ErrDomain
	}
	// Dot and cross rotate the phase origin to the exact start ray
	// without normalization. Reversing the oriented cross handles CW.
	phase := func(radial sweepOffsetPoint) sweepOffsetPoint {
		x, y := sweepOffsetDot(a, radial), sweepOffsetCross(a, radial)
		if s.Clockwise {
			y.Neg(y)
		}
		return sweepOffsetPoint{x, y}
	}
	upper := func(v sweepOffsetPoint) bool {
		return v[1].Sign() > 0 || v[1].Sign() == 0 && v[0].Sign() >= 0
	}
	less := func(a, b sweepOffsetPoint) bool {
		if up, other := upper(a), upper(b); up != other {
			return up
		}
		return sweepOffsetCross(a, b).Sign() > 0
	}
	type stop struct {
		ray, key sweepOffsetPoint
	}
	var interior []stop
	z, negative := new(big.Rat), new(big.Rat).Neg(r)
	end := phase(b)
	for _, axis := range []sweepOffsetPoint{{r, z}, {z, r}, {negative, z}, {z, negative}} {
		key := phase(axis)
		if key[1].Sign() == 0 && key[0].Sign() > 0 {
			continue
		}
		if s.FullCircle || less(key, end) {
			interior = append(interior, stop{axis, key})
		}
	}
	sort.Slice(interior, func(i, j int) bool { return less(interior[i].key, interior[j].key) })
	start := s.Start
	out := make([]ExactCircularLoopSegment, 0, len(interior)+1)
	for _, station := range interior {
		x, err := geom.RepresentedExactScalar(new(big.Rat).Add(c[0], station.ray[0]))
		if err != nil {
			return nil, err
		}
		y, err := geom.RepresentedExactScalar(new(big.Rat).Add(c[1], station.ray[1]))
		if err != nil {
			return nil, err
		}
		next := geom.Vec2{X: x, Y: y}
		out = append(out, ExactCircularLoopSegment{Start: start, End: next, Center: s.Center, Radius: s.Radius})
		start = next
	}
	return append(out, ExactCircularLoopSegment{Start: start, End: s.End, Center: s.Center, Radius: s.Radius}), nil
}
