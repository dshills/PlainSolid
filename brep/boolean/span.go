package boolean

import "sort"

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
	u, on := leg{ed.c, ed.t0, ed.t1}.reach(a.at(n).pos)
	if !on {
		u = ed.t0
		if p := a.at(n).pos; p.Distance(ed.c.at(ed.t1)) < p.Distance(ed.c.at(ed.t0)) {
			u = ed.t1
		}
	}
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

// within lists the positions of the stops of an edge that an overlap
// covers on one side: from one end node of the overlap to the other in
// the direction of that edge, around the vertex of a closed edge where
// the overlap runs through it, or every stop for a whole overlap.
func (a *arrangement) within(l lap, side int) []int {
	list := a.stops[side][l.edges[side]]
	var out []int
	if l.whole {
		for k := range list {
			out = append(out, k)
		}
		return out
	}
	from, to := a.find(l.ends[0]), a.find(l.ends[1])
	if side == 1 && l.opposed {
		from, to = to, from
	}
	start := -1
	for k, s := range list {
		if a.find(s.node) == from {
			start = k
			break
		}
	}
	if start < 0 {
		return nil
	}
	// On a closed edge the last stop is the first again.
	closed := a.find(list[0].node) == a.find(cyclic(list, -1).node)
	for k := start; ; k++ {
		if k == len(list) {
			if !closed {
				return nil
			}
			k = 0 // the vertex again, now as the start of the first span
		}
		out = append(out, k)
		if a.find(list[k].node) == to && (k != start || from != to) || len(out) > 2*len(list) {
			return out
		}
	}
}

// agree makes the two edges of an overlap have the same nodes inside it
// (X3): every node of either that lies in the overlap is put on the other.
func (a *arrangement) agree(l lap) error {
	for side := range l.edges {
		var nodes []int
		for _, k := range a.within(l, side) {
			nodes = append(nodes, a.stops[side][l.edges[side]][k].node)
		}
		for _, n := range nodes {
			if _, err := a.place(1-side, l.edges[1-side], n); err != nil {
				return err
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
	// Spans, in the order of operands and edges. spanAt gives the span
	// that starts at each stop, or -1.
	var spanAt [2][][]int
	for i, op := range a.ops {
		for e, ed := range op.edges {
			list := a.stops[i][e]
			at := filled(list, -1)
			for k := 0; k+1 < len(list); k++ {
				p, q := point(list[k].node), point(list[k+1].node)
				switch {
				case p != q || len(list) == 2 && ed.c.period() > 0 && ed.t1-ed.t0 >= ed.c.period():
					// A span, or the closed span of a closed edge.
					at[k] = len(a.spans)
					a.spans = append(a.spans, span{op: i, edge: e, a: p, b: q, t0: list[k].t, t1: list[k+1].t, bundle: -1, elem: -1})
				case len(list) == 2:
					return fail(ErrTolerance, "arrangement", "edge %d of %s is shorter than the tolerance", e, operandName(i))
				}
			}
			spanAt[i] = append(spanAt[i], at)
		}
	}
	// spansOf lists the spans that start at the given stops of an edge,
	// the last of them left out: a span ends there.
	spansOf := func(op, e int, stops []int) []int {
		var out []int
		for _, k := range stops[:max(len(stops), 1)-1] {
			if k < len(spanAt[op][e]) && spanAt[op][e][k] >= 0 {
				out = append(out, spanAt[op][e][k])
			}
		}
		return out
	}
	// between lists the spans of an edge from node x to node y, all of
	// them when the two are one node of a closed edge.
	between := func(op, e, x, y int) []int {
		l := lap{ends: [2]int{x, y}, whole: a.find(x) == a.find(y)}
		l.edges[op] = e
		stops := a.within(l, op)
		return spansOf(op, e, stops)
	}
	// Bundles: inside an overlap the spans of the two edges correspond,
	// each to the one with the same two points in the matching order.
	for _, l := range a.laps {
		var lists [2][]int
		for side := range lists {
			lists[side] = spansOf(side, l.edges[side], a.within(l, side))
		}
		mismatch := fail(ErrTolerance, "arrangement", "edge %d of A and edge %d of B overlap but their spans do not correspond (CC4)", l.edges[0], l.edges[1])
		if lists[0] == nil || len(lists[0]) != len(lists[1]) {
			return mismatch
		}
		for _, s := range lists[0] {
			x := &a.spans[s]
			found := -1
			for _, o := range lists[1] {
				if y := a.spans[o]; y.bundle < 0 && (!l.opposed && x.a == y.a && x.b == y.b || l.opposed && x.a == y.b && x.b == y.a) {
					found = o
					break
				}
			}
			if x.bundle >= 0 || found < 0 {
				return mismatch
			}
			x.bundle, a.spans[found].bundle = len(a.bundles), len(a.bundles)
			a.bundles = append(a.bundles, bundle{spans: []int{s, found}, reversed: []bool{false, l.opposed}, elem: -1})
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
					var own []int
					for _, s := range spanAt[i][u.edge] {
						if s >= 0 {
							own = append(own, s)
						}
					}
					for k := range own {
						s := own[k]
						if u.reversed {
							s = cyclic(own, -1-k)
						}
						hops = append(hops, hop{a.spans[s].elem, u.reversed != a.spans[s].reversed})
					}
				}
				loops = append(loops, hops)
			}
			a.rims[i] = append(a.rims[i], loops)
		}
	}
	// AR5: what lies in each face, along its boundary, and loose in it.
	within := func(l lying) []int { return between(l.op, l.edge, l.ends[0], l.ends[1]) }
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
	// A span may have been found in a face by more than one pass: each
	// list names an element once, in index order.
	for i := range a.inner {
		for f, list := range a.inner[i] {
			sort.Ints(list)
			unique := list[:0]
			for k, e := range list {
				if k == 0 || e != list[k-1] {
					unique = append(unique, e)
				}
			}
			a.inner[i][f] = unique
		}
	}
	for _, b := range a.bundles {
		for k, s := range b.spans {
			other := a.spans[b.spans[1-k]]
			for _, f := range a.owners[other.op][other.edge] {
				if f >= 0 {
					a.along[other.op][f] = append(a.along[other.op][f], s)
				}
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
