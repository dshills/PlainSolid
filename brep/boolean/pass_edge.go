package boolean

import (
	"math"

	"plainsolid/geom"
)

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
				// A point of radius ε must lie on both edges (T3). One
				// at an end of an edge merges with the vertex there.
				p, q := leg{x.c, x.t0, x.t1}.nearest(h.p), leg{y.c, y.t0, y.t1}.nearest(h.p)
				if h.gap > 2*a.t.eps || p.Distance(h.p) > a.t.eps || q.Distance(h.p) > a.t.eps {
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
// second edge is projected onto the first carrier; each part of the first
// edge it covers is tested at its two ends and its middle against the
// second carrier. On a closed carrier the parameter is periodic: there
// may be two such parts, a part may run through the vertex of a closed
// first edge, and two closed edges overlap wholly or not at all.
func (a *arrangement) overlap(ea, eb int) error {
	x, y := a.ops[0].edges[ea], a.ops[1].edges[eb]
	period := x.c.period()
	mine, theirs := a.stops[0][ea], a.stops[1][eb]
	// test counts how many of three points of the first carrier lie on
	// the second, and refuses a mixed answer.
	test := func(points [3]geom.Vec3) (bool, error) {
		on := 0
		for _, p := range points {
			if off(y.c, p) <= a.t.eps {
				on++
			}
		}
		if on == 1 || on == 2 {
			return false, fail(ErrTolerance, "X3", "edge %d of A and edge %d of B coincide at some points of their overlap and not at others (CC4)", ea, eb)
		}
		return on == 3, nil
	}
	if x.closed() && y.closed() {
		third := period / 3
		same, err := test([3]geom.Vec3{x.c.at(x.t0), x.c.at(x.t0 + third), x.c.at(x.t0 + 2*third)})
		if err != nil || !same {
			return err
		}
		l := lap{edges: [2]int{ea, eb}, whole: true, opposed: x.c.tangent(x.t0).Dot(y.c.tangent(y.c.param(x.c.at(x.t0)))) < 0}
		a.laps = append(a.laps, l)
		return a.agree(l)
	}
	// The second edge in the parameter of the first carrier: it starts
	// at one of its ends and runs for its own length.
	opposed := x.c.tangent(x.c.param(y.c.at(y.t0))).Dot(y.c.tangent(y.t0)) < 0
	begin, first, last := x.c.param(y.c.at(y.t0)), theirs[0].node, cyclic(theirs, -1).node
	if opposed {
		begin, first, last = x.c.param(y.c.at(y.t1)), last, first
	}
	length := y.t1 - y.t0
	type part struct {
		u0, u1 float64
		ends   [2]int
	}
	var parts []part
	cover := func(begin float64) {
		p := part{math.Max(x.t0, begin), math.Min(x.t1, begin+length), [2]int{first, last}}
		if p.u0 == x.t0 && begin < x.t0 {
			p.ends[0] = mine[0].node
		}
		if p.u1 == x.t1 && begin+length > x.t1 {
			p.ends[1] = cyclic(mine, -1).node
		}
		if p.u0 <= p.u1 {
			parts = append(parts, p)
		}
	}
	switch {
	case period == 0:
		cover(begin)
	case x.closed():
		// One part, which may run through the vertex of the first edge.
		begin += period * math.Ceil((x.t0-begin)/period)
		parts = append(parts, part{begin, begin + length, [2]int{first, last}})
	default:
		begin += period * math.Ceil((x.t0-begin)/period)
		cover(begin)
		cover(begin - period)
	}
	for _, p := range parts {
		ends := [2]geom.Vec3{x.c.at(p.u0), x.c.at(p.u1)}
		middle := x.c.at((p.u0 + p.u1) / 2)
		if middle.Distance(ends[0]) <= a.t.eps && middle.Distance(ends[1]) <= a.t.eps {
			continue // degenerate: no overlap, and its one point is a vertex
		}
		same, err := test([3]geom.Vec3{ends[0], middle, ends[1]})
		if err != nil {
			return err
		}
		if !same {
			continue
		}
		l := lap{edges: [2]int{ea, eb}, opposed: opposed}
		for k, n := range p.ends {
			if n, err = a.place(0, ea, n); err != nil {
				return err
			}
			if l.ends[k], err = a.place(1, eb, n); err != nil {
				return err
			}
		}
		a.laps = append(a.laps, l)
		if err := a.agree(l); err != nil {
			return err
		}
	}
	return nil
}

// lapped reports whether the part of an edge between two of its nodes
// runs along the boundary of a face of the other operand: an overlap
// joins the edge to a boundary edge of the face that has both nodes and
// passes through the middle of the part.
func (a *arrangement) lapped(op, e int, from, to stop, f int) bool {
	middle := a.ops[op].edges[e].c.at((from.t + to.t) / 2)
	for _, l := range a.laps {
		other := l.edges[1-op]
		if l.edges[op] != e || a.owners[1-op][other][0] != f && a.owners[1-op][other][1] != f {
			continue
		}
		ed := a.ops[1-op].edges[other]
		if a.onEdge(1-op, other, from.node) && a.onEdge(1-op, other, to.node) && middle.Distance(leg{ed.c, ed.t0, ed.t1}.nearest(middle)) <= a.t.eps {
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
				if m.open {
					// No closed form (7.5, last row): the conic must be
					// proved clear of the face (CC5).
					k := ed.c.(conic)
					major, _, _ := k.axes()
					p0, p1 := a.extent(1-op, f)
					if segmentDistance(k.center, k.center, p0, p1) > major+face.s.(cylinder).radius+2*a.t.margin {
						continue
					}
					return fail(ErrUnsupported, "X5", "edge %d of %s is a conic that the table cannot meet with the cylinder of face %d of %s", e, operandName(op), f, operandName(1-op))
				}
				for k := 0; k+1 < len(a.stops[op][e]); k++ {
					from, to := a.stops[op][e][k], a.stops[op][e][k+1]
					if a.lapped(op, e, from, to, f) {
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
	ed := a.ops[op].edges[e]
	for k := 0; k+1 < len(a.stops[op][e]); k++ {
		from, to := a.stops[op][e][k], a.stops[op][e][k+1]
		if u, on := (leg{ed.c, from.t, to.t}).reach(h.p); !on || u <= from.t || u >= to.t || ed.c.at(u).Distance(h.p) > a.t.eps {
			continue
		}
		if a.covers(from.node, h.p) || a.covers(to.node, h.p) || a.lapped(op, e, from, to, f) {
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
			// The edge touches the carrier at the point. That is a
			// touching contact of the operands only when the faces
			// of the edge stay on one side of the carrier there; an
			// edge that is tangent to a surface which one of its own
			// faces passes through is a place where they cross.
			if h.touching && a.oneSided(op, e, f, h.p) {
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
	ed := a.ops[op].edges[e]
	middle := ed.c.at((from.t + to.t) / 2)
	on := 0
	for _, n := range [3]point{{a.at(from.node).pos, a.at(from.node).radius}, {a.at(to.node).pos, a.at(to.node).radius}, {middle, a.t.eps}} {
		if d := face.s.offset(n.pos); d <= n.radius && d >= -n.radius {
			on++
		}
	}
	if on == 0 {
		return nil
	}
	where := a.inFace(1-op, f, middle, a.t.eps)
	if on < 3 || where == onRim {
		return fail(ErrTolerance, "X5", "edge %d of %s neither lies in face %d of %s nor is clear of it", e, operandName(op), f, operandName(1-op))
	}
	if where == without {
		return nil
	}
	record := a.lie(lying{op: op, edge: e, ends: [2]int{from.node, to.node}, face: f})
	if !a.oneSided(op, e, f, middle) {
		return nil
	}
	c := touch{node: -1, lie: record}
	c.faces[op], c.faces[1-op] = a.owners[op][e][:], []int{f}
	a.touches = append(a.touches, c)
	return nil
}

// oneSided reports whether the boundary of an operand lies, near the
// point p of its edge e, on one and the same side of the carrier of face
// f of the other operand: the condition for a touching contact where an
// edge lies in a face or meets it at a point (X5). Both uses of the edge
// are asked, each for its own face; a seam is used twice by one face,
// which leaves it to both sides.
//
// A face is judged by where its points near p lie, not by a direction at
// p. The contacts this decides are tangent ones, where a direction that
// leaves the edge is in the tangent plane of the carrier and tells
// nothing, and where one face of the edge can lie along the carrier
// while the other passes through it. A face with points on both sides
// crosses the carrier. A face that lies on the carrier, as at a flush
// corner, is on neither side: that is a same-surface matter and not a
// touching contact. The edge is one-sided when every face of it is off
// the carrier, crosses nowhere near p, and all are on one side.
func (a *arrangement) oneSided(op, e, f int, p geom.Vec3) bool {
	carrier := a.ops[1-op].faces[f].s
	common := 0
	owners := a.owners[op][e]
	for k, own := range owners {
		use := 0
		if k == 1 && owners[0] == own {
			use = 1
		}
		side := 0
		for _, q := range a.near(op, own, e, use, p) {
			switch d := carrier.offset(q); {
			case d > a.t.eps && side < 0, d < -a.t.eps && side > 0:
				return false // the face has points on both sides
			case d > a.t.eps:
				side = 1
			case d < -a.t.eps:
				side = -1
			}
		}
		if side == 0 || common != 0 && side != common {
			return false
		}
		common = side
	}
	return true
}

// near gives points of face f of an operand close to the point p of its
// boundary edge e: a fan of chart steps from p into the face, from near
// one direction of the edge round to near the other. Where the face uses
// the edge more than once, use counts the uses to pass over.
func (a *arrangement) near(op, f, e, use int, p geom.Vec3) []geom.Vec3 {
	face := a.ops[op].faces[f]
	ch := face.s.chart()
	for i, loop := range face.loops {
		for k, u := range loop {
			if u.edge != e {
				continue
			}
			if use--; use >= 0 {
				continue
			}
			// The face is on the left of its boundary, so a quarter
			// turn to the left of the heading points into it.
			l := a.borders[op][f][i][k]
			h, _ := heading(ch, leg{l.c, l.c.param(p), l.t1})
			q, step := ch.locate(p), a.t.size/1024
			var out []geom.Vec3
			for _, turn := range [5]float64{math.Pi / 6, math.Pi / 3, math.Pi / 2, 2 * math.Pi / 3, 5 * math.Pi / 6} {
				s, c := math.Sincos(turn)
				out = append(out, ch.lift(geom.Vec2{X: q.X + step*(c*h.X-s*h.Y), Y: q.Y + step*(c*h.Y+s*h.X)}))
			}
			return out
		}
	}
	return nil
}
