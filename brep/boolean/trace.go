package boolean

import (
	"sort"

	"plainsolid/geom"
)

// lay cuts a carrier curve of a face of A and a face of B at the nodes
// already on it (X6). Each interval between consecutive nodes that lies
// inside both trimmed regions becomes a trace. An interval that runs
// along the boundary of one face and inside the other is a span that
// exists; it is recorded as lying in the other face. An interval along
// both boundaries is a bundle that exists. A closed curve with one node
// is one interval from that node round to itself. A closed curve with no
// node, inside both regions, first receives a node at its parameter zero
// and is then one closed trace.
func (a *arrangement) lay(c curve, fa, fb int, touching bool) error {
	faces := [2]int{fa, fb}
	period := c.period()
	// The nodes of either face on the curve, in the order of its
	// parameter: those on the boundaries and those loose inside.
	var marks []stop
	seen := filled(a.nodes, false)
	mark := func(n int) {
		n = a.find(n)
		if p := a.nodes[n]; !seen[n] && off(c, p.pos) <= p.radius {
			seen[n] = true
			marks = append(marks, stop{n, c.param(p.pos)})
		}
	}
	for i, f := range faces {
		for _, loop := range a.ops[i].faces[f].loops {
			for _, u := range loop {
				for _, s := range a.stops[i][u.edge] {
					mark(s.node)
				}
			}
		}
		for _, n := range a.loose[i][f] {
			mark(n)
		}
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].t < marks[j].t })
	if period > 0 {
		if marks == nil {
			for _, t := range [2]float64{0, period / 2} {
				if a.inFace(0, fa, c.at(t), a.t.eps) != within || a.inFace(1, fb, c.at(t), a.t.eps) != within {
					return nil
				}
			}
			n, err := a.add(c.at(0))
			if err != nil {
				return err
			}
			marks = []stop{{n, 0}}
		}
		// Round to the first node again.
		marks = append(marks, stop{marks[0].node, marks[0].t + period})
	}
	for k := 0; k+1 < len(marks); k++ {
		from, to := marks[k], marks[k+1]
		if a.find(from.node) == a.find(to.node) && (period == 0 || to.t-from.t < period/2) {
			continue
		}
		if a.find(from.node) != a.find(to.node) && a.coincident(from.node, to.node) {
			n, err := a.merge(from.node, to.node)
			if err != nil {
				return err
			}
			marks[k+1].node = n
			continue
		}
		middle := c.at((from.t + to.t) / 2)
		var in [2]where
		for i, f := range faces {
			in[i] = a.inFace(i, f, middle, a.t.eps)
		}
		switch {
		case in[0] == without || in[1] == without:
		case in[0] == within && in[1] == within:
			a.traces = append(a.traces, trace{c: c, t0: from.t, t1: to.t, ends: [2]int{from.node, to.node}, faces: faces, touching: touching, elem: -1})
		default:
			// A span of one face's boundary occupies the interval.
			for i, f := range faces {
				if in[i] != onRim {
					continue
				}
				e, ends := a.rimEdge(i, f, from.node, to.node, middle)
				if e < 0 {
					return fail(ErrInconsistent, "X6", "face %d of A and face %d of B meet along the boundary of one of them where it has no span", fa, fb)
				}
				if in[1-i] == within {
					record := a.lie(lying{op: i, edge: e, ends: ends, face: faces[1-i]})
					if touching {
						c := touch{node: -1, lie: record}
						c.faces[i], c.faces[1-i] = a.owners[i][e][:], []int{faces[1-i]}
						a.touches = append(a.touches, c)
					}
				}
			}
		}
	}
	return nil
}

// rimEdge finds the boundary edge of a face on which two nodes are next
// to each other with the given point between them, and gives the two in
// the order of the edge. It returns -1 when there is none.
func (a *arrangement) rimEdge(op, f, x, y int, middle geom.Vec3) (int, [2]int) {
	x, y = a.find(x), a.find(y)
	for _, loop := range a.ops[op].faces[f].loops {
		for _, u := range loop {
			ed := a.ops[op].edges[u.edge]
			list := a.stops[op][u.edge]
			for k := 0; k+1 < len(list); k++ {
				p, q := a.find(list[k].node), a.find(list[k+1].node)
				if (p == x && q == y || p == y && q == x) && middle.Distance(leg{ed.c, list[k].t, list[k+1].t}.nearest(middle)) <= a.t.eps {
					return u.edge, [2]int{p, q}
				}
			}
		}
	}
	return -1, [2]int{}
}
