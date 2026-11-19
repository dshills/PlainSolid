package poly

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"runtime"
	"sort"
)

// EdgeOverlap describes a positive-length exact overlap with an original
// topological edge. OutputFrom/To increase along result Edge.Start->End.
// InputFrom/To are corresponding parameters along the original Start->End and
// may decrease. All four rationals are caller-owned and lie in [0,1].
type EdgeOverlap struct {
	Operand                                  Operand
	Edge                                     int
	OutputFrom, OutputTo, InputFrom, InputTo *big.Rat
}

// EdgeAncestry aligns with the result edge array. AdjacentFaces records source
// ancestry in ascending output face order. Overlaps is exhaustive across both
// operands, including coincident non-owning boundaries. HasGeneratedPortion says
// the union of these overlaps does not cover the entire output edge. A generated
// portion can be a Boolean intersection or a conforming triangulation edge; it
// is not necessarily a new geometric crease.
type EdgeAncestry struct {
	AdjacentFaces       []Source
	Overlaps            []EdgeOverlap
	HasGeneratedPortion bool
}

// TopologyHistory combines face ownership with exact geometric edge provenance.
// Edge comparisons use pre-conversion rational output positions. Identity is
// local to this operation, not persistent naming. Errors discard all history.
type TopologyHistory struct {
	History
	Edges []EdgeAncestry
}

// CombineWithTopologyHistory adds exact edge provenance to CombineWithHistory.
// The additional edge comparison pass has its own MaxWork budget, using the
// same option value. Its cost is O(output edges * input topological edges).
func CombineWithTopologyHistory(ctx context.Context, a, b brep.Solid, op Operation, o Options) (brep.Solid, TopologyHistory, error) {
	var sources []Source
	var points []point
	s, e := combineDetailed(ctx, a, b, op, o, &sources, &points)
	if e != nil {
		return brep.Solid{}, TopologyHistory{}, e
	}
	o, e = defaults(o)
	if e != nil {
		return brep.Solid{}, TopologyHistory{}, e
	}
	bu := &budget{ctx: ctx, o: o}
	h := TopologyHistory{History: faceHistory(a, b, s, sources)}
	h.Edges, e = edgeAncestry(s, points, h.Faces, [2]brep.Solid{a, b}, bu)
	if e != nil {
		return brep.Solid{}, TopologyHistory{}, e
	}
	return s, h, nil
}

func edgeAncestry(s brep.Solid, points []point, faces []Source, inputs [2]brep.Solid, bu *budget) ([]EdgeAncestry, error) {
	h := make([]EdgeAncestry, len(s.Edges()))
	for i, f := range s.Faces() {
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, u := range loop {
				h[u.Edge].AdjacentFaces = append(h[u.Edge].AdjacentFaces, faces[i])
			}
		}
	}
	type original struct {
		operand Operand
		index   int
		a, b    point
	}
	var originals []original
	for oi, in := range inputs {
		vs := in.Vertices()
		for i, edge := range in.Edges() {
			if !bu.tick(1) {
				return nil, bu.err
			}
			originals = append(originals, original{Operand(oi), i, vec(vs[edge.Start]), vec(vs[edge.End])})
		}
	}
	for i, edge := range s.Edges() {
		x, y := points[edge.Start], points[edge.End]
		d := diff(y, x)
		axis := 0
		for d[axis].Sign() == 0 {
			axis++
		}
		entry := &h[i]
		for _, original := range originals {
			if !bu.tick(1) {
				return nil, bu.err
			}
			p, q := original.a, original.b
			if !zero(cross(d, diff(p, x))) || !zero(cross(d, diff(q, x))) {
				continue
			}
			t0, t1 := div(sub(p[axis], x[axis]), d[axis]), div(sub(q[axis], x[axis]), d[axis])
			runtime.KeepAlive(&q) // golang/go#82026: keeps q in the stack map. Remove once the Go fix is in use.
			lo, hi := t0, t1
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			if lo.Sign() < 0 {
				lo = new(big.Rat)
			}
			if hi.Cmp(big.NewRat(1, 1)) > 0 {
				hi = big.NewRat(1, 1)
			}
			if lo.Cmp(hi) >= 0 {
				continue
			}
			u0, u1 := div(sub(lo, t0), sub(t1, t0)), div(sub(hi, t0), sub(t1, t0))
			for _, r := range []*big.Rat{lo, hi, u0, u1} {
				bu.check(r)
			}
			if bu.err != nil {
				return nil, bu.err
			}
			entry.Overlaps = append(entry.Overlaps, EdgeOverlap{original.operand, original.index, new(big.Rat).Set(lo), new(big.Rat).Set(hi), u0, u1})
		}
		// Sort a separate interval list so public provenance remains operand/index
		// ordered, independent of interval ordering and ownership choices.
		intervals := append([]EdgeOverlap(nil), entry.Overlaps...)
		sort.Slice(intervals, func(i, j int) bool { return intervals[i].OutputFrom.Cmp(intervals[j].OutputFrom) < 0 })
		end := new(big.Rat)
		for _, v := range intervals {
			if v.OutputFrom.Cmp(end) > 0 {
				break
			}
			if v.OutputTo.Cmp(end) > 0 {
				end = v.OutputTo
			}
		}
		entry.HasGeneratedPortion = end.Cmp(big.NewRat(1, 1)) < 0
	}
	if !bu.tick(1) {
		return nil, bu.err
	}
	return h, nil
}
