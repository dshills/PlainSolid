package boolean

import "plainsolid/geom"

// The passes of section 7.3 that start from an edge: X3 and X5.

// edgeWithEdge is X3. A pair of edges gives isolated points, which become
// nodes on both, or an overlap (CC4), whose ends become nodes on both and
// inside which the two edges then have the same nodes.
func (a *arrangement) edgeWithEdge() error {
	for ea, x := range a.ops[0].edges {
		for eb, y := range a.ops[1].edges {
			if x.box.clear(y.box, a.t.margin) {
				continue
			}
			if err := a.tick(); err != nil {
				return err
			}
			m, err := curveMeets(x.c, y.c, a.t)
			if err != nil {
				return failAt(err, "edge %d of A with edge %d of B", ea, eb)
			}
			if m.alike {
				if err := a.overlap(ea, eb); err != nil {
					return err
				}
				continue
			}
			for _, h := range m.hits {
				// A point at an end of either edge is a vertex, which X1
				// and X2 have settled; a point of radius ε must lie on
				// both carriers (T3).
				if h.t < x.t0 || h.t > x.t1 || h.s < y.t0 || h.s > y.t1 || h.gap > 2*a.t.eps {
					continue
				}
				n, err := a.add(h.p)
				if err != nil {
					return err
				}
				if n, err = a.place(0, ea, n); err != nil {
					return err
				}
				if _, err = a.place(1, eb, n); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// overlap applies CC4 to two edges whose carriers are candidates. The
// second edge is projected onto the first carrier; the part of the first
// edge it covers is tested at its two ends and its middle against the
// second carrier.
func (a *arrangement) overlap(ea, eb int) error {
	x, y := a.ops[0].edges[ea], a.ops[1].edges[eb]
	s0, s1 := x.c.param(y.c.at(y.t0)), x.c.param(y.c.at(y.t1))
	opposed := s1 < s0
	if opposed {
		s0, s1 = s1, s0
	}
	u0, u1 := max(x.t0, s0), min(x.t1, s1)
	if u0 > u1 {
		return nil
	}
	ends := [2]geom.Vec3{x.c.at(u0), x.c.at(u1)}
	middle := x.c.at((u0 + u1) / 2)
	if middle.Distance(ends[0]) <= a.t.eps && middle.Distance(ends[1]) <= a.t.eps {
		return nil // degenerate: no overlap, and its one point is a vertex
	}
	on := 0
	for _, p := range [3]geom.Vec3{ends[0], middle, ends[1]} {
		if off(y.c, p) <= a.t.eps {
			on++
		}
	}
	switch on {
	case 0:
		return nil
	case 1, 2:
		return fail(ErrTolerance, "X3", "edge %d of A and edge %d of B coincide at some points of their overlap and not at others (CC4)", ea, eb)
	}
	// Each end of the interval is an end of one of the two edges.
	l := lap{edges: [2]int{ea, eb}, opposed: opposed}
	mine, theirs := a.stops[0][ea], a.stops[1][eb]
	for k, u := range [2]float64{u0, u1} {
		own := [2]int{mine[0].node, cyclic(mine, -1).node}[k]
		far := [2]int{theirs[0].node, cyclic(theirs, -1).node}[k]
		if opposed {
			far = [2]int{cyclic(theirs, -1).node, theirs[0].node}[k]
		}
		n := far
		if u == [2]float64{x.t0, x.t1}[k] {
			n = own
		}
		var err error
		if n, err = a.place(0, ea, n); err != nil {
			return err
		}
		if l.ends[k], err = a.place(1, eb, n); err != nil {
			return err
		}
	}
	a.laps = append(a.laps, l)
	return a.agree(l)
}

// lapped reports whether the part of an edge between two of its nodes
// runs along the boundary of a face of the other operand: an overlap
// joins the edge to a boundary edge of the face that has both nodes.
func (a *arrangement) lapped(op, e, from, to, f int) bool {
	for _, l := range a.laps {
		other := l.edges[1-op]
		if l.edges[op] != e || a.owners[1-op][other][0] != f && a.owners[1-op][other][1] != f {
			continue
		}
		if a.onEdge(1-op, other, from) && a.onEdge(1-op, other, to) {
			return true
		}
	}
	return false
}

// edgeWithFace is X5. Every span, as split so far, is met with every
// face of the other operand. A point strictly inside the span and inside
// the face's trimmed region becomes a node of the span, recorded for the
// face. A span on the carrier and inside the trimmed region is recorded
// as lying in the face, and as a touching contact when both faces of its
// edge stay on one side of the carrier.
func (a *arrangement) edgeWithFace() error {
	for op := range a.ops {
		for e, ed := range a.ops[op].edges {
			for f, face := range a.ops[1-op].faces {
				if ed.box.clear(face.box, a.t.margin) {
					continue
				}
				m, err := curveMeets(ed.c, face.s, a.t)
				if err != nil {
					return failAt(err, "edge %d of %s with face %d of %s", e, operandName(op), f, operandName(1-op))
				}
				for k := 0; k+1 < len(a.stops[op][e]); k++ {
					from, to := a.stops[op][e][k], a.stops[op][e][k+1]
					if a.lapped(op, e, from.node, to.node, f) {
						continue // along the boundary: already in a bundle
					}
					if err := a.tick(); err != nil {
						return err
					}
					if m.alike {
						if err := a.spanInFace(op, e, from, to, f); err != nil {
							return err
						}
					}
				}
				for _, h := range m.hits {
					if err := a.pierce(op, e, f, h); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// pierce handles an isolated point where an edge meets the carrier of a
// face of the other operand.
func (a *arrangement) pierce(op, e, f int, h hit) error {
	for k := 0; k+1 < len(a.stops[op][e]); k++ {
		from, to := a.stops[op][e][k], a.stops[op][e][k+1]
		if h.t <= from.t || h.t >= to.t {
			continue
		}
		if a.covers(from.node, h.p) || a.covers(to.node, h.p) || a.lapped(op, e, from.node, to.node, f) {
			return nil // an end of the span is a node that exists
		}
		switch a.inFace(1-op, f, h.p, a.t.eps) {
		case onRim:
			return fail(ErrTolerance, "X5", "edge %d of %s meets the boundary of face %d of %s where no edge of it was found", e, operandName(op), f, operandName(1-op))
		case within:
			n, err := a.add(h.p)
			if err != nil {
				return err
			}
			if n, err = a.place(op, e, n); err != nil {
				return err
			}
			a.loose[1-op][f] = append(a.loose[1-op][f], n)
			if h.touching {
				c := touch{node: n, lie: -1}
				c.faces[op], c.faces[1-op] = a.owners[op][e][:], []int{f}
				a.touches = append(a.touches, c)
			}
		}
		return nil
	}
	return nil
}

// spanInFace handles a span whose carrier may lie in the carrier of a
// face. The two nodes of the span decide: both on the carrier and the
// span's middle inside the trimmed region, and the span lies in the face.
func (a *arrangement) spanInFace(op, e int, from, to stop, f int) error {
	face := a.ops[1-op].faces[f]
	on := 0
	for _, s := range [2]stop{from, to} {
		if n := a.at(s.node); n.radius >= face.s.offset(n.pos) && -n.radius <= face.s.offset(n.pos) {
			on++
		}
	}
	if on == 0 {
		return nil
	}
	ed := a.ops[op].edges[e]
	middle := ed.c.at((from.t + to.t) / 2)
	where := a.inFace(1-op, f, middle, a.t.eps)
	if on == 1 || where == onRim {
		return fail(ErrTolerance, "X5", "edge %d of %s neither lies in face %d of %s nor is clear of it", e, operandName(op), f, operandName(1-op))
	}
	if where == without {
		return nil
	}
	record := a.lie(lying{op: op, edge: e, ends: [2]int{from.node, to.node}, face: f})
	// A touching contact: both faces of the edge leave the span to one
	// side of the carrier.
	common := 0
	for _, own := range a.owners[op][e] {
		side := a.t.side(face.s, middle, a.inward(op, own, e, middle))
		if side == 0 || common != 0 && side != common {
			return nil
		}
		common = side
	}
	c := touch{node: -1, lie: record}
	c.faces[op], c.faces[1-op] = a.owners[op][e][:], []int{f}
	a.touches = append(a.touches, c)
	return nil
}

// inward is the direction in which face f of an operand leaves its
// boundary edge e at point p: across the edge, into the face.
func (a *arrangement) inward(op, f, e int, p geom.Vec3) geom.Vec3 {
	face := a.ops[op].faces[f]
	ch := face.s.chart()
	for i, loop := range face.loops {
		for k, u := range loop {
			if u.edge != e {
				continue
			}
			l := a.borders[op][f][i][k]
			h, _ := ch.heading(leg{l.c, l.c.param(p), l.t1})
			q := ch.locate(p)
			d, err := ch.lift(geom.Vec2{X: q.X - h.Y, Y: q.Y + h.X}).Sub(ch.lift(q)).Unit()
			if err == nil {
				return d
			}
		}
	}
	return geom.Vec3{}
}
