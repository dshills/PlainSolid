package boolean

import "plainsolid/geom"

// The passes of section 7.3 that start from a vertex: X1, X2 and X4. Each
// pass tests one member of A with one of B, only where their bounds grown
// by the filter margin overlap (T4), and counts every geometric test.

// spot is the bounds of a node at its current position.
func (a *arrangement) spot(n int) bounds { return around(a.at(n).pos) }

// vertexWithVertex is X1: coincident vertices merge into one node.
func (a *arrangement) vertexWithVertex() error {
	for i := range a.vnode[0] {
		for j := range a.vnode[1] {
			x, y := a.find(a.vnode[0][i]), a.find(a.vnode[1][j])
			if x == y || a.spot(x).clear(a.spot(y), a.t.margin) {
				continue
			}
			if err := a.tick(); err != nil {
				return err
			}
			if a.coincident(x, y) {
				if _, err := a.merge(x, y); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// vertexWithEdge is X2: a node on an edge of the other operand joins the
// edge's nodes. Where it is coincident with a node already there, the end
// of the edge included, the two merge.
func (a *arrangement) vertexWithEdge() error {
	for op := range a.ops {
		for i := range a.vnode[op] {
			for e, ed := range a.ops[1-op].edges {
				n := a.find(a.vnode[op][i])
				if a.onEdge(1-op, e, n) || a.spot(n).clear(ed.box, a.t.margin) {
					continue
				}
				if err := a.tick(); err != nil {
					return err
				}
				if p := a.at(n).pos; p.Distance(leg{ed.c, ed.t0, ed.t1}.nearest(p)) > a.at(n).radius {
					continue
				}
				if _, err := a.place(1-op, e, n); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// vertexWithFace is X4: a node on the carrier of a face of the other
// operand and strictly inside its trimmed region, which X1 and X2 did not
// put on the face's boundary, is recorded for the face. It is a touching
// contact when every face at the vertex stays on one side of the carrier.
func (a *arrangement) vertexWithFace() error {
	for op := range a.ops {
		for i := range a.vnode[op] {
			for f, face := range a.ops[1-op].faces {
				n := a.find(a.vnode[op][i])
				if a.spot(n).clear(face.box, a.t.margin) || a.onBorder(1-op, f, n) {
					continue
				}
				if err := a.tick(); err != nil {
					return err
				}
				p, r := a.at(n).pos, a.at(n).radius
				if d := face.s.offset(p); d > r || d < -r {
					continue
				}
				switch a.inFace(1-op, f, p, r) {
				case onRim:
					return fail(ErrTolerance, "X4", "vertex %d of %s is within tolerance of the boundary of face %d of %s but on none of its edges", i, operandName(op), f, operandName(1-op))
				case within:
					a.loose[1-op][f] = append(a.loose[1-op][f], n)
					if faces := a.facesOnOneSide(op, i, face.s); faces != nil {
						c := touch{node: n, lie: -1}
						c.faces[op], c.faces[1-op] = faces, []int{f}
						a.touches = append(a.touches, c)
					}
				}
			}
		}
	}
	return nil
}

// onBorder reports whether node n is on a boundary edge of a face.
func (a *arrangement) onBorder(op, f, n int) bool {
	for _, loop := range a.ops[op].faces[f].loops {
		for _, u := range loop {
			if a.onEdge(op, u.edge, n) {
				return true
			}
		}
	}
	return false
}

// side tells on which side of a surface carrier a direction leaves a
// point of it: +1 along the carrier normal, -1 against it, and 0 when the
// direction stays in the carrier within the angular tolerance (T5).
func (t tolerances) side(s surface, at, direction geom.Vec3) int {
	switch d := s.normal(at).Dot(direction); {
	case d > t.angular:
		return 1
	case d < -t.angular:
		return -1
	}
	return 0
}

// facesOnOneSide lists the faces of an operand at one of its vertices
// when every one of them lies strictly on one side of the carrier s near
// the vertex, and returns nil otherwise. A face does so when its corner
// at the vertex is convex and both of its edges there leave to that side.
func (a *arrangement) facesOnOneSide(op, vertex int, s surface) []int {
	o := a.ops[op]
	var faces []int
	common := 0
	for f, face := range o.faces {
		ch := face.s.chart()
		for i, loop := range face.loops {
			for k, u := range loop {
				ed := o.edges[u.edge]
				if u.reversed && ed.end != vertex || !u.reversed && ed.start != vertex {
					continue
				}
				// The use leaves the vertex; the one before it arrives.
				leave := a.borders[op][f][i][k]
				arrive := cyclic(a.borders[op][f][i], k-1).back()
				out, _ := ch.heading(leave)
				in, _ := ch.heading(arrive)
				at := o.verts[vertex]
				sides := [2]int{a.t.side(s, at, leave.c.tangent(leave.t0).Scale(sign(leave.t1-leave.t0))), a.t.side(s, at, arrive.c.tangent(arrive.t0).Scale(sign(arrive.t1-arrive.t0)))}
				convex := out.X*in.Y-out.Y*in.X > 0
				if !convex || sides[0] == 0 || sides[0] != sides[1] || common != 0 && common != sides[0] {
					return nil
				}
				common = sides[0]
				faces = append(faces, f)
			}
		}
	}
	return faces
}

func sign(x float64) float64 {
	if x < 0 {
		return -1
	}
	return 1
}
