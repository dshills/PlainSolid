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
	for si, sh := range sheets {
		// sole reports an element that no other face walks.
		sole := func(elem int) bool {
			for j, other := range sheets {
				for _, loop := range other.loops {
					for _, h := range loop {
						if j != si && h.elem == elem {
							return false
						}
					}
				}
			}
			return true
		}
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
				cut, parts, crossed, err := a.acrossSeam(edges, face, corners, sole)
				if err != nil {
					return nil, nil, err
				}
				if crossed {
					edges, pending = cut, append(parts, pending[1:]...)
					continue
				}
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
	return a.apportion(edges, face, loops)
}

// apportion makes the two faces of a divided face from its loops as the
// dividers left them: a loop of positive chart area is the outer loop of
// a face, and every other loop is a hole of the smallest face that
// contains it (F4, M7).
func (a *arrangement) apportion(edges []element, face sheet, loops [][]hop) ([]sheet, error) {
	ch := face.s.chart()
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

// acrossSeam is M6 for a face on a carrier that repeats, when no chord
// between two of its corners divides it: a chord round the carrier,
// between two corners of one hole at one height, that crosses the seam
// of the face. In the chart such a chord is two segments, from the
// corner on the left to the left edge of the face and from the right
// edge to the corner on the right, and their two ends at the edges are
// one point of the seam. That point becomes a vertex of the seam, the one
// vertex M6 adds, and the two segments become dividers. The face is then
// two faces, one on each side of the chord, each with its part of the
// seam.
//
// The seam is cut only where it is an edge of this face alone, which
// sole reports, walked once each way by the outer loop. Conditions (a)
// to (d) of M6 hold of the chord as of any other, each segment being
// tested for (b), and the shortest chord is taken.
func (a *arrangement) acrossSeam(edges []element, face sheet, corners []corner, sole func(elem int) bool) ([]element, []sheet, bool, error) {
	ch := face.s.chart()
	period := ch.period()
	if period == 0 || len(face.loops) < 2 {
		return edges, nil, false, nil
	}
	legs := legsOf(edges, face.loops)
	rim := place(ch, legs)[0]
	outer := face.loops[0]
	points := len(a.points)
	// build makes the division by the chord between two corners, lo to
	// the left of hi, and reports whether it is one.
	build := func(lo, hi corner) ([]element, [][]hop, bool) {
		for k, h := range outer {
			twin := -1
			for m, other := range outer {
				if m != k && other.elem == h.elem && other.reversed != h.reversed {
					twin = m
				}
			}
			if twin < 0 || rim[k].curved() || !sole(h.elem) {
				continue
			}
			// The hop on the left edge, its twin on the right, a whole
			// turn apart, and the height of the chord strictly between
			// the ends of the seam edge.
			left, right := rim[k].at(rim[k].t0), rim[twin].at(rim[twin].t0)
			from, to := left.Y, rim[k].at(rim[k].t1).Y
			if math.Abs(right.X-left.X-period) > a.t.eps || math.Abs(rim[k].at(rim[k].t1).X-left.X) > a.t.eps ||
				!(lo.at.Y > math.Min(from, to)+a.t.eps && lo.at.Y < math.Max(from, to)-a.t.eps) ||
				!(lo.at.X > left.X+a.t.eps && hi.at.X < right.X-a.t.eps) {
				continue
			}
			near, ok := ch.chord(lo.at, geom.Vec2{X: left.X, Y: lo.at.Y}, a.t.eps)
			far, also := ch.chord(geom.Vec2{X: right.X, Y: hi.at.Y}, hi.at, a.t.eps)
			e := edges[h.elem]
			at := e.c.param(near.c.at(near.t1))
			if !ok || !also || !(at > e.t0 && at < e.t1) {
				continue
			}
			// The seam edge becomes two at the new vertex.
			a.points = append(a.points, point{e.c.at(at), a.t.eps})
			vertex := len(a.points) - 1
			cut := append([]element(nil), edges...)
			cut[h.elem] = element{e.c, e.t0, at, e.a, vertex}
			cut = append(cut, element{e.c, at, e.t1, vertex, e.b})
			upper := len(cut) - 1
			var walk []hop
			var leaves [2]int // where the outer loop leaves the vertex: on the left, on the right
			for m, g := range outer {
				switch {
				case g.elem != h.elem:
					walk = append(walk, g)
					continue
				case g.reversed:
					walk = append(walk, hop{upper, true}, hop{h.elem, true})
				default:
					walk = append(walk, hop{h.elem, false}, hop{upper, false})
				}
				if m == k {
					leaves[0] = len(walk) - 1
				} else {
					leaves[1] = len(walk) - 1
				}
			}
			// (b) and (c), on the face with its seam cut and no divider.
			split := sheet{s: face.s, reversed: face.reversed, loops: append([][]hop{walk}, face.loops[1:]...), sources: face.sources}
			all, _, err := a.cornersOf(cut, split)
			if err != nil {
				return nil, nil, false
			}
			sides := legsOf(cut, split.loops)
			seam := corner{point: vertex}
			if !a.inside(ch, sides, all, near, lo, seam) || !a.inside(ch, sides, all, far, seam, hi) {
				return nil, nil, false
			}
			for _, end := range []struct {
				c     corner
				along geom.Vec3
			}{{lo, near.c.tangent(near.t0)}, {hi, far.c.tangent(far.t1)}} {
				leave, arrive := legs[end.c.loop][end.c.index], cyclic(legs[end.c.loop], end.c.index-1)
				if a.t.parallel(end.along, leave.c.tangent(leave.t0)) || a.t.parallel(end.along, arrive.c.tangent(arrive.t1)) {
					return nil, nil, false
				}
			}
			// The dividers, in arcs of at most a quarter turn.
			var out, back, in, home []hop // lo to the seam and back, the seam to hi and back
			for i, l := range [2]leg{near, far} {
				ends := [2]int{lo.point, vertex}
				if i == 1 {
					ends = [2]int{vertex, hi.point}
				}
				arcs := conditioned(element{l.c, l.t0, l.t1, ends[0], ends[1]}, 0)
				start := ends[0]
				for n, arc := range arcs {
					stop := ends[1]
					if n+1 < len(arcs) {
						a.points = append(a.points, point{l.c.at(arc.t1), a.t.eps})
						stop = len(a.points) - 1
					}
					cut = append(cut, element{l.c, arc.t0, arc.t1, start, stop})
					start = stop
					if i == 0 {
						out, back = append(out, hop{len(cut) - 1, false}), append([]hop{{len(cut) - 1, true}}, back...)
					} else {
						in, home = append(in, hop{len(cut) - 1, false}), append([]hop{{len(cut) - 1, true}}, home...)
					}
				}
			}
			// The hole from lo round to hi goes with the outer loop from
			// the right edge round to the left, and the rest with the
			// rest: every loop keeps the face on its left.
			hole := face.loops[lo.loop]
			round := func(loop []hop, from, to int) []hop {
				var part []hop
				for i := from; i != to; i = (i + 1) % len(loop) {
					part = append(part, loop[i])
				}
				return part
			}
			first := append(append(append(round(hole, lo.index, hi.index), home...), round(walk, leaves[1], leaves[0])...), back...)
			second := append(append(append(round(hole, hi.index, lo.index), out...), round(walk, leaves[0], leaves[1])...), in...)
			loops := [][]hop{first, second}
			for _, loop := range loops {
				if !(chartArea(ch, legsOf(cut, [][]hop{loop})) > 0) {
					return nil, nil, false
				}
			}
			for i, loop := range face.loops {
				if i != 0 && i != lo.loop {
					loops = append(loops, loop)
				}
			}
			return cut, loops, true
		}
		return nil, nil, false
	}
	var chosen [2]corner
	best := math.Inf(1)
	for i, cx := range corners {
		for _, cy := range corners[i+1:] {
			if cy.loop != cx.loop || cx.loop == 0 || a.samePlace(cx, cy) || math.Abs(cy.at.Y-cx.at.Y) > a.t.eps {
				continue
			}
			lo, hi := cx, cy
			if hi.at.X < lo.at.X {
				lo, hi = hi, lo
			}
			length := period - (hi.at.X - lo.at.X)
			if !(length < best*(1-1e-12)) || !a.separates(corners, cx, cy) {
				continue
			}
			_, _, ok := build(lo, hi)
			a.points = a.points[:points]
			if ok {
				chosen, best = [2]corner{lo, hi}, length
			}
		}
	}
	if math.IsInf(best, 1) {
		return edges, nil, false, nil
	}
	cut, loops, _ := build(chosen[0], chosen[1])
	parts, err := a.apportion(cut, face, loops)
	return cut, parts, err == nil, err
}
