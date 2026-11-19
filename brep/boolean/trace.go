package boolean

import "sort"

// lay cuts a carrier curve of a face of A and a face of B at the nodes
// already on it (X6). Each interval between consecutive nodes that lies
// inside both trimmed regions becomes a trace. An interval that runs
// along the boundary of one face and inside the other is a span that
// exists; it is recorded as lying in the other face. An interval along
// both boundaries is a bundle that exists. No node is created here.
func (a *arrangement) lay(c curve, fa, fb int, touching bool) error {
	faces := [2]int{fa, fb}
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
	for k := 0; k+1 < len(marks); k++ {
		from, to := marks[k], marks[k+1]
		if a.find(from.node) == a.find(to.node) {
			continue
		}
		if a.coincident(from.node, to.node) {
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
				e, ends := a.rimEdge(i, f, from.node, to.node)
				if e < 0 {
					return fail(ErrInconsistent, "X6", "face %d of A and face %d of B meet along the boundary of one of them where it has no span", fa, fb)
				}
				if in[1-i] == within {
					a.lie(lying{op: i, edge: e, ends: ends, face: faces[1-i]})
				}
			}
		}
	}
	return nil
}

// rimEdge finds the boundary edge of a face on which two nodes are next
// to each other, and gives the two in the order of the edge. It returns
// -1 when there is none.
func (a *arrangement) rimEdge(op, f, x, y int) (int, [2]int) {
	x, y = a.find(x), a.find(y)
	for _, loop := range a.ops[op].faces[f].loops {
		for _, u := range loop {
			list := a.stops[op][u.edge]
			for k := 0; k+1 < len(list); k++ {
				if p, q := a.find(list[k].node), a.find(list[k+1].node); p == x && q == y || p == y && q == x {
					return u.edge, [2]int{p, q}
				}
			}
		}
	}
	return -1, [2]int{}
}
