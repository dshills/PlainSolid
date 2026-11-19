package boolean

import (
	"math"

	"plainsolid/geom"
)

// corner is the start of one use of a face: the point it leaves and the
// place of that point in the chart of the face as its loops are laid out.
type corner struct {
	loop, index int
	point       int
	at          geom.Vec2
}

// divide applies M6 after M4: a face with a repeated corner, two corners
// at one place of its chart, is divided along a chord between two other
// corners of the same loop, until no face has one. A divider is a new
// edge of the result between two points that exist; no point is added
// and none is moved. M7 is checked on the way: every face has one outer
// loop.
func (a *arrangement) divide(edges []element, sheets []sheet) ([]element, []sheet, error) {
	var out []sheet
	for _, sh := range sheets {
		pending := []sheet{sh}
		for len(pending) > 0 {
			face := pending[0]
			corners, repeated, err := a.cornersOf(edges, face)
			if err != nil {
				return nil, nil, err
			}
			if !repeated {
				out, pending = append(out, face), pending[1:]
				continue
			}
			x, y, chord, found := a.divider(edges, face, corners)
			if !found {
				at := a.points[a.repeatedAt(corners)].pos
				return nil, nil, fail(ErrUnsupported, "M6", "a face from the input faces %v touches itself at %v and no chord between two of its corners divides it", face.sources, at)
			}
			edges = append(edges, element{chord.c, chord.t0, chord.t1, x.point, y.point})
			parts, err := a.parted(edges, face, x, y)
			if err != nil {
				return nil, nil, err
			}
			pending = append(parts, pending[1:]...)
		}
	}
	return edges, out, nil
}

// samePlace reports whether two corners are one chart point.
func (a *arrangement) samePlace(x, y corner) bool {
	return x.point == y.point && math.Hypot(x.at.X-y.at.X, x.at.Y-y.at.Y) <= a.points[x.point].radius
}

// quarter cuts every curved edge into arcs of at most a quarter turn, the
// first step of B2, before M6 looks for corners: the ends of the arcs are
// vertices of the result, and so corners that a divider can run to. The
// loops of the sheets are rewritten in place to walk the arcs.
func (a *arrangement) quarter(edges []element, sheets []sheet) []element {
	var out []element
	parts := table[[]int](edges)
	for i, e := range edges {
		from := e.a
		legs := conditioned(e, 0)
		for k, l := range legs {
			to := e.b
			if k+1 < len(legs) {
				a.points = append(a.points, point{e.c.at(l.t1), a.t.eps})
				to = len(a.points) - 1
			}
			parts[i] = append(parts[i], len(out))
			out = append(out, element{e.c, l.t0, l.t1, from, to})
			from = to
		}
	}
	for _, sh := range sheets {
		for i, loop := range sh.loops {
			var walk []hop
			for _, h := range loop {
				for k := range parts[h.elem] {
					if h.reversed {
						k = len(parts[h.elem]) - 1 - k
					}
					walk = append(walk, hop{parts[h.elem][k], h.reversed})
				}
			}
			sh.loops[i] = walk
		}
	}
	return out
}

// cornersOf numbers the corners of a face, loop by loop, and reports
// whether two of them are at one chart point. Two loops with a corner at
// one chart point contradict F3 and M3.
func (a *arrangement) cornersOf(edges []element, face sheet) ([]corner, bool, error) {
	var out []corner
	repeated := false
	for i, paths := range place(face.s.chart(), legsOf(edges, face.loops)) {
		for k, p := range paths {
			h := face.loops[i][k]
			c := corner{i, k, edges[h.elem].a, p.at(p.t0)}
			if h.reversed {
				c.point = edges[h.elem].b
			}
			for _, other := range out {
				if a.samePlace(c, other) {
					if other.loop != i {
						return nil, false, fail(ErrInconsistent, "M6", "two loops of one face pass through %v", a.points[c.point].pos)
					}
					repeated = true
				}
			}
			out = append(out, c)
		}
	}
	return out, repeated, nil
}

// repeatedAt is the point of the first repeated corner.
func (a *arrangement) repeatedAt(corners []corner) int {
	for i, c := range corners {
		for _, other := range corners[:i] {
			if a.samePlace(c, other) {
				return c.point
			}
		}
	}
	return 0
}

// divider chooses the chord of M6 for a face: among the pairs of corners
// of one loop that meet conditions (a) to (d), the shortest, and of equal
// ones the first in loop and corner order.
func (a *arrangement) divider(edges []element, face sheet, corners []corner) (x, y corner, chord leg, found bool) {
	ch := face.s.chart()
	legs := legsOf(edges, face.loops)
	best := math.Inf(1)
	for i, cx := range corners {
		for _, cy := range corners[i+1:] {
			if cy.loop != cx.loop || a.samePlace(cx, cy) {
				continue
			}
			length := math.Hypot(cy.at.X-cx.at.X, cy.at.Y-cx.at.Y)
			if !(length < best*(1-1e-12)) {
				continue
			}
			// (a) The chord is a line, circle or ellipse in space.
			l, ok := ch.chord(cx.at, cy.at, a.t.eps)
			if !ok || !a.separates(corners, cx, cy) || !a.inside(ch, legs, corners, l, cx, cy) || a.continues(legs, l, cx, cy) {
				continue
			}
			x, y, chord, found, best = cx, cy, l, true, length
		}
	}
	return x, y, chord, found
}

// separates is condition (d): some chart point holds a corner numbered
// strictly between x and y and also one numbered below x or above y.
func (a *arrangement) separates(corners []corner, x, y corner) bool {
	for _, c := range corners {
		if c.loop != x.loop || c.index <= x.index || c.index >= y.index {
			continue
		}
		for _, other := range corners {
			if other.loop == x.loop && (other.index < x.index || other.index > y.index) && a.samePlace(c, other) {
				return true
			}
		}
	}
	return false
}

// inside is condition (b): no other corner of the face is on the chord,
// the chord meets no use of the face except at its two ends, and its
// middle is inside the face.
func (a *arrangement) inside(ch chart, legs [][]leg, corners []corner, chord leg, x, y corner) bool {
	ends := [2]point{a.points[x.point], a.points[y.point]}
	atEnd := func(p geom.Vec3) bool {
		return p.Distance(ends[0].pos) <= ends[0].radius+a.t.eps || p.Distance(ends[1].pos) <= ends[1].radius+a.t.eps
	}
	for _, c := range corners {
		if p := a.points[c.point]; c.point != x.point && c.point != y.point && p.pos.Distance(chord.nearest(p.pos)) <= p.radius {
			return false
		}
	}
	for _, loop := range legs {
		for _, use := range loop {
			m, err := curveMeets(chord.c, use.c, a.t)
			if err != nil {
				return false
			}
			if mid := use.c.at((use.t0 + use.t1) / 2); m.alike && mid.Distance(chord.nearest(mid)) <= a.t.eps {
				return false // the use runs along the chord
			}
			for _, h := range m.hits {
				if h.gap <= 2*a.t.eps && !atEnd(h.p) && h.p.Distance(chord.nearest(h.p)) <= a.t.eps && h.p.Distance(use.nearest(h.p)) <= a.t.eps {
					return false
				}
			}
		}
	}
	return locate(ch, legs, chord.c.at((chord.t0+chord.t1)/2), a.t.eps) == within
}

// continues is condition (c): at one of its ends the chord is parallel to
// one of the two uses of the loop that meet at that corner. Such a chord
// would give one of the new faces three corners on one line.
func (a *arrangement) continues(legs [][]leg, chord leg, x, y corner) bool {
	for i, c := range [2]corner{x, y} {
		along := chord.c.tangent([2]float64{chord.t0, chord.t1}[i])
		leave := legs[c.loop][c.index]
		arrive := cyclic(legs[c.loop], c.index-1)
		if a.t.parallel(along, leave.c.tangent(leave.t0)) || a.t.parallel(along, arrive.c.tangent(arrive.t1)) {
			return true
		}
	}
	return false
}

// parted replaces the loop of x and y by the two loops the divider makes
// of it, the divider being the last of edges, and gives the two faces:
// a loop of positive chart area is the outer loop of a face, and every
// other loop is a hole of the smallest face that contains it (F4, M7).
func (a *arrangement) parted(edges []element, face sheet, x, y corner) ([]sheet, error) {
	ch := face.s.chart()
	divider := len(edges) - 1
	old := face.loops[x.loop]
	first := append(append([]hop(nil), old[x.index:y.index]...), hop{divider, true})
	second := append(append(append([]hop(nil), old[y.index:]...), old[:x.index]...), hop{divider, false})
	loops := [][]hop{first, second}
	for i, loop := range face.loops {
		if i != x.loop {
			loops = append(loops, loop)
		}
	}
	areas := table[float64](loops)
	var out []sheet
	owner := filled(loops, -1)
	for i, loop := range loops {
		if areas[i] = chartArea(ch, legsOf(edges, [][]hop{loop})); areas[i] > 0 {
			owner[i] = len(out)
			out = append(out, sheet{s: face.s, reversed: face.reversed, loops: [][]hop{loop}, sources: face.sources})
		}
	}
	if len(out) != 2 {
		return nil, fail(ErrValidation, "M7", "a divided face has %d outer loops", len(out))
	}
	for i, loop := range loops {
		if areas[i] > 0 {
			continue
		}
		best := -1
		for j := range loops {
			if areas[j] > 0 && (best < 0 || areas[j] < areas[best]) && a.encloses(ch, edges, loops[j], loop) {
				best = j
			}
		}
		if best < 0 {
			return nil, fail(ErrInconsistent, "F4", "a hole of a divided face lies in neither part")
		}
		out[owner[best]].loops = append(out[owner[best]].loops, loop)
	}
	return out, nil
}

// encloses reports whether the region inside an outer loop holds a hole
// loop: no point of the hole is outside it and one is strictly inside.
func (a *arrangement) encloses(ch chart, edges []element, outer, hole []hop) bool {
	border := legsOf(edges, [][]hop{outer})
	inside := false
	for _, l := range legsOf(edges, [][]hop{hole})[0] {
		for _, p := range [2]geom.Vec3{l.c.at(l.t0), l.c.at((l.t0 + l.t1) / 2)} {
			switch locate(ch, border, p, a.t.eps) {
			case without:
				return false
			case within:
				inside = true
			}
		}
	}
	return inside
}
