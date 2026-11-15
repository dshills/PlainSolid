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

type directedCircularSpanPhase struct {
	center, start, end sweepOffsetPoint
	radius, radius2    *big.Rat
	clockwise          bool
}

func circularSpanPhase(s ExactDirectedCircularSpan) (directedCircularSpanPhase, error) {
	var zero directedCircularSpanPhase
	if !uvFinite(s.Start) || !uvFinite(s.End) || !uvFinite(s.Center) || !finite(s.Radius) ||
		s.Radius <= 0 || (s.Start == s.End) != s.FullCircle {
		return zero, geom.ErrInvalid
	}
	r := ratInset(s.Radius)
	r2 := new(big.Rat).Mul(r, r)
	c := sweepOffsetPointOf(s.Center)
	a, b := sweepOffsetSub(sweepOffsetPointOf(s.Start), c), sweepOffsetSub(sweepOffsetPointOf(s.End), c)
	if sweepOffsetDot(a, a).Cmp(r2) != 0 || sweepOffsetDot(b, b).Cmp(r2) != 0 {
		return zero, geom.ErrDomain
	}
	return directedCircularSpanPhase{center: c, start: a, end: b, radius: r, radius2: r2, clockwise: s.Clockwise}, nil
}

// Dot and cross rotate the phase origin to the exact start ray without
// normalization. Reversing the oriented cross handles CW.
func (p directedCircularSpanPhase) key(radial sweepOffsetPoint) sweepOffsetPoint {
	x, y := sweepOffsetDot(p.start, radial), sweepOffsetCross(p.start, radial)
	if p.clockwise {
		y.Neg(y)
	}
	return sweepOffsetPoint{x, y}
}

func circularSpanPhaseLess(a, b sweepOffsetPoint) bool {
	upper := func(v sweepOffsetPoint) bool {
		return v[1].Sign() > 0 || v[1].Sign() == 0 && v[0].Sign() >= 0
	}
	if up, other := upper(a), upper(b); up != other {
		return up
	}
	return sweepOffsetCross(a, b).Sign() > 0
}

// SplitExactDirectedCircularSpan divides the authored sweep at the original
// circle's traversed coordinate axes. Each returned segment is at most a
// quarter turn and can be used directly in an exact circular loop. Order,
// direction, original endpoints and circle support are preserved. A required
// split point that cannot be represented exactly is rejected.
func SplitExactDirectedCircularSpan(s ExactDirectedCircularSpan) ([]ExactCircularLoopSegment, error) {
	p, err := circularSpanPhase(s)
	if err != nil {
		return nil, err
	}
	type stop struct {
		ray, key sweepOffsetPoint
	}
	var interior []stop
	r, z := p.radius, new(big.Rat)
	negative := new(big.Rat).Neg(r)
	end := p.key(p.end)
	for _, axis := range []sweepOffsetPoint{{r, z}, {z, r}, {negative, z}, {z, negative}} {
		key := p.key(axis)
		if key[1].Sign() == 0 && key[0].Sign() > 0 {
			continue
		}
		if s.FullCircle || circularSpanPhaseLess(key, end) {
			interior = append(interior, stop{axis, key})
		}
	}
	sort.Slice(interior, func(i, j int) bool { return circularSpanPhaseLess(interior[i].key, interior[j].key) })
	start := s.Start
	out := make([]ExactCircularLoopSegment, 0, len(interior)+1)
	for _, station := range interior {
		x, err := geom.RepresentedExactScalar(new(big.Rat).Add(p.center[0], station.ray[0]))
		if err != nil {
			return nil, err
		}
		y, err := geom.RepresentedExactScalar(new(big.Rat).Add(p.center[1], station.ray[1]))
		if err != nil {
			return nil, err
		}
		next := geom.Vec2{X: x, Y: y}
		out = append(out, ExactCircularLoopSegment{Start: start, End: next, Center: s.Center, Radius: s.Radius})
		start = next
	}
	return append(out, ExactCircularLoopSegment{Start: start, End: s.End, Center: s.Center, Radius: s.Radius}), nil
}

// PartitionExactDirectedCircularSpan retains the supplied ordered interior
// points instead of requiring coordinate axis stations. All points must
// be exactly on the support and strictly inside the directed source span.
// Each adjacent pair, including the endpoints, must bound a minor arc.
func PartitionExactDirectedCircularSpan(s ExactDirectedCircularSpan, stations []geom.Vec2) ([]ExactCircularLoopSegment, error) {
	if len(stations) > 63 {
		return nil, geom.ErrConvergence
	}
	p, err := circularSpanPhase(s)
	if err != nil {
		return nil, err
	}
	previous, ray, key := s.Start, p.start, p.key(p.start)
	last := p.key(p.end)
	winding := 1
	if s.Clockwise {
		winding = -1
	}
	out := make([]ExactCircularLoopSegment, 0, len(stations)+1)
	for _, point := range stations {
		if !uvFinite(point) {
			return nil, geom.ErrInvalid
		}
		v := sweepOffsetSub(sweepOffsetPointOf(point), p.center)
		k := p.key(v)
		if sweepOffsetDot(v, v).Cmp(p.radius2) != 0 || !circularSpanPhaseLess(key, k) ||
			!s.FullCircle && !circularSpanPhaseLess(k, last) || sweepOffsetCross(ray, v).Sign() != winding {
			return nil, geom.ErrDomain
		}
		out = append(out, ExactCircularLoopSegment{Start: previous, End: point, Center: s.Center, Radius: s.Radius})
		previous, ray, key = point, v, k
	}
	if sweepOffsetCross(ray, p.end).Sign() != winding {
		return nil, geom.ErrDomain
	}
	return append(out, ExactCircularLoopSegment{Start: previous, End: s.End, Center: s.Center, Radius: s.Radius}), nil
}
