package boolean

import (
	"math"
	"sort"
)

// stop is a node on an input edge, with its parameter on the edge's
// carrier. The stops of an edge are kept in parameter order (AR2).
type stop struct {
	node int
	t    float64
}

// span is the part of one input edge between two consecutive nodes on it
// (AR2). A closed span has both ends at one point.
type span struct {
	op, edge int
	a, b     int // points, in the direction of the edge
	t0, t1   float64
	bundle   int  // the bundle it is a member of, or -1
	elem     int  // the element that stands for it
	reversed bool // it runs against that element
}

// bundle is a set of spans of different edges that occupy one curve
// interval (AR3). It is one edge of the result.
type bundle struct {
	spans    []int
	reversed []bool // each member's direction against the first
	elem     int
}

// onEdge reports whether node n is one of the stops of an edge.
func (a *arrangement) onEdge(op, e, n int) bool {
	n = a.find(n)
	for _, s := range a.stops[op][e] {
		if a.find(s.node) == n {
			return true
		}
	}
	return false
}

// place puts node n on an edge at its parameter there and returns the
// node that then stands for it: n itself or, where n is coincident with a
// stop next to that parameter, the two merged (T3).
func (a *arrangement) place(op, e, n int) (int, error) {
	if a.onEdge(op, e, n) {
		return a.find(n), nil
	}
	ed := a.ops[op].edges[e]
	list := a.stops[op][e]
	u := math.Max(ed.t0, math.Min(ed.t1, ed.c.param(a.at(n).pos)))
	k := sort.Search(len(list), func(i int) bool { return list[i].t >= u })
	for _, j := range [2]int{k - 1, k} {
		if j >= 0 && j < len(list) && a.coincident(n, list[j].node) {
			return a.merge(n, list[j].node)
		}
	}
	list = append(list, stop{})
	copy(list[k+1:], list[k:])
	list[k] = stop{a.find(n), u}
	a.stops[op][e] = list
	return a.find(n), nil
}

// agree makes the two edges of an overlap have the same nodes inside it
// (X3): every node of either that lies in the overlap is put on the other.
func (a *arrangement) agree(l lap) error {
	for side := range l.edges {
		inside := false
		for _, s := range a.stops[side][l.edges[side]] {
			n := a.find(s.node)
			first, last := n == a.find(l.ends[0]), n == a.find(l.ends[1])
			if inside || first || last {
				if _, err := a.place(1-side, l.edges[1-side], n); err != nil {
					return err
				}
			}
			if first || last {
				inside = !inside
			}
		}
	}
	return nil
}

// seal fixes the arrangement for the building stage. Nodes become points;
// the stops of every edge become spans; every overlap becomes bundles of
// corresponding spans; spans, bundles and traces become elements; and the
// lists of AR5 to AR7 are rewritten over points and elements.
func (a *arrangement) seal() error {
	pointOf := filled(a.nodes, -1)
	for i, n := range a.nodes {
		if n.into < 0 {
			pointOf[i] = len(a.points)
			a.points = append(a.points, point{n.pos, n.radius})
		}
	}
	point := func(n int) int { return pointOf[a.find(n)] }
	for _, l := range a.laps {
		if err := a.agree(l); err != nil {
			return err
		}
	}
	// Spans, in the order of operands and edges.
	var first [2][]int
	for i, op := range a.ops {
		for e := range op.edges {
			first[i] = append(first[i], len(a.spans))
			list := a.stops[i][e]
			for k := 0; k+1 < len(list); k++ {
				p, q := point(list[k].node), point(list[k+1].node)
				if p != q {
					a.spans = append(a.spans, span{op: i, edge: e, a: p, b: q, t0: list[k].t, t1: list[k+1].t, bundle: -1, elem: -1})
				} else if len(list) == 2 {
					return fail(ErrTolerance, "arrangement", "edge %d of %s is shorter than the tolerance", e, operandName(i))
				}
			}
		}
		first[i] = append(first[i], len(a.spans))
	}
	// between lists the spans of an edge from point p to point q.
	between := func(op, e, p, q int) []int {
		var out []int
		for s := first[op][e]; s < first[op][e+1]; s++ {
			if len(out) > 0 || a.spans[s].a == p {
				out = append(out, s)
				if a.spans[s].b == q {
					return out
				}
			}
		}
		return nil
	}
	// Bundles: inside an overlap the spans of the two edges correspond.
	for _, l := range a.laps {
		p, q := point(l.ends[0]), point(l.ends[1])
		mine := between(0, l.edges[0], p, q)
		if l.opposed {
			p, q = q, p
		}
		theirs := between(1, l.edges[1], p, q)
		if mine == nil || len(mine) != len(theirs) {
			return fail(ErrTolerance, "arrangement", "edge %d of A and edge %d of B overlap but their spans do not correspond (CC4)", l.edges[0], l.edges[1])
		}
		for k, s := range mine {
			o := theirs[k]
			if l.opposed {
				o = theirs[len(theirs)-1-k]
			}
			x, y := &a.spans[s], &a.spans[o]
			matched := x.a == y.a && x.b == y.b
			if l.opposed {
				matched = x.a == y.b && x.b == y.a
			}
			if x.bundle >= 0 || y.bundle >= 0 || !matched {
				return fail(ErrTolerance, "arrangement", "edge %d of A and edge %d of B overlap but their spans do not correspond (CC4)", l.edges[0], l.edges[1])
			}
			x.bundle, y.bundle = len(a.bundles), len(a.bundles)
			a.bundles = append(a.bundles, bundle{spans: []int{s, o}, reversed: []bool{false, l.opposed}, elem: -1})
		}
	}
	// Elements: a span in no bundle, or a bundle with the carrier of its
	// first member, which is the span of A.
	for i := range a.spans {
		s := &a.spans[i]
		if s.bundle >= 0 && a.bundles[s.bundle].elem >= 0 {
			s.elem, s.reversed = a.bundles[s.bundle].elem, a.bundles[s.bundle].reversed[1]
			continue
		}
		s.elem = len(a.elems)
		a.elems = append(a.elems, element{a.ops[s.op].edges[s.edge].c, s.t0, s.t1, s.a, s.b})
		if s.bundle >= 0 {
			a.bundles[s.bundle].elem = s.elem
		}
	}
	for i := range a.traces {
		tr := &a.traces[i]
		tr.ends = [2]int{point(tr.ends[0]), point(tr.ends[1])}
		tr.elem = len(a.elems)
		a.elems = append(a.elems, element{tr.c, tr.t0, tr.t1, tr.ends[0], tr.ends[1]})
	}
	// The boundary of every input face as hops over elements.
	for i, op := range a.ops {
		a.inner[i], a.along[i], a.free[i] = table[[]int](op.faces), table[[]int](op.faces), table[[]int](op.faces)
		for _, f := range op.faces {
			var loops [][]hop
			for _, loop := range f.loops {
				var hops []hop
				for _, u := range loop {
					for s := first[i][u.edge]; s < first[i][u.edge+1]; s++ {
						k := s
						if u.reversed {
							k = first[i][u.edge] + first[i][u.edge+1] - 1 - s
						}
						hops = append(hops, hop{a.spans[k].elem, u.reversed != a.spans[k].reversed})
					}
				}
				loops = append(loops, hops)
			}
			a.rims[i] = append(a.rims[i], loops)
		}
	}
	// AR5: what lies in each face, along its boundary, and loose in it.
	within := func(l lying) []int { return between(l.op, l.edge, point(l.ends[0]), point(l.ends[1])) }
	for _, l := range a.lies {
		found := within(l)
		if found == nil {
			return fail(ErrTolerance, "arrangement", "a part of edge %d of %s that lay in face %d of %s is no longer between two of its nodes", l.edge, operandName(l.op), l.face, operandName(1-l.op))
		}
		for _, s := range found {
			a.inner[1-l.op][l.face] = append(a.inner[1-l.op][l.face], a.spans[s].elem)
		}
	}
	for _, tr := range a.traces {
		for i, f := range tr.faces {
			a.inner[i][f] = append(a.inner[i][f], tr.elem)
		}
	}
	for _, b := range a.bundles {
		for k, s := range b.spans {
			other := a.spans[b.spans[1-k]]
			for _, f := range a.owners[other.op][other.edge] {
				a.along[other.op][f] = append(a.along[other.op][f], s)
			}
		}
	}
	for i := range a.loose {
		for f, list := range a.loose[i] {
			for _, n := range list {
				a.free[i][f] = append(a.free[i][f], point(n))
			}
		}
	}
	// AR7: touching nodes, touching spans, then touching traces.
	for _, c := range a.touches {
		if c.node >= 0 {
			a.contacts = append(a.contacts, contact{point(c.node), -1, c.faces})
			continue
		}
		for _, s := range within(a.lies[c.lie]) {
			a.contacts = append(a.contacts, contact{-1, a.spans[s].elem, c.faces})
		}
	}
	for _, tr := range a.traces {
		if tr.touching {
			a.contacts = append(a.contacts, contact{-1, tr.elem, [2][]int{{tr.faces[0]}, {tr.faces[1]}}})
		}
	}
	return nil
}
