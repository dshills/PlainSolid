package boolean

import (
	"math"

	"plainsolid/geom"
)

// coalesce merges the kept pieces into the faces of the result (section
// 7.10) and returns the edges and sheets that assembly builds from.
// Merging is part of the contract (M5), not an option.
func (a *arrangement) coalesce(pieces []piece, keep []kept) ([]element, []sheet, error) {
	users := table[[]int](a.elems)
	for i, k := range keep {
		for _, loop := range pieces[k.piece].loops {
			for _, h := range loop {
				users[h.elem] = append(users[h.elem], i)
			}
		}
	}
	// M1: an element whose two uses are by kept pieces on one surface,
	// facing the same way, can go.
	removable, removed := table[bool](a.elems), table[bool](a.elems)
	for e, u := range users {
		if len(u) != 2 {
			continue
		}
		removable[e], removed[e] = a.oneFace(pieces, keep[u[0]], keep[u[1]]), u[0] == u[1]
	}
	group := table[int](keep)
	members := table[[]int](keep)
	for i := range group {
		group[i], members[i] = i, []int{i}
	}
	for e, u := range users {
		if !removable[e] || removed[e] || group[u[0]] == group[u[1]] {
			continue
		}
		x, y := group[u[0]], group[u[1]]
		if y < x {
			x, y = y, x
		}
		// Merging two groups removes every removable element between
		// and inside them; the face they would make is then laid out,
		// which keeps the elements it needs as a cut (M2). A face that
		// would visit a point twice is not made (M3).
		saved := append([]bool(nil), removed...)
		for f, v := range users {
			if removable[f] && (group[v[0]] == x || group[v[0]] == y) && (group[v[1]] == x || group[v[1]] == y) {
				removed[f] = true
			}
		}
		joined := append(append([]int(nil), members[x]...), members[y]...)
		if _, simple, err := a.layout(pieces, keep, joined, users, removed); err != nil || !simple {
			copy(removed, saved)
			continue
		}
		for _, i := range members[y] {
			group[i] = x
		}
		members[x], members[y] = joined, nil
	}
	// One sheet for every group, in the order of its first piece, with
	// the carrier of that piece (M1) and the sources of all (P5).
	var sheets []sheet
	for _, list := range members {
		if list == nil {
			continue
		}
		// A group that is one piece with a repeated corner keeps its
		// loop as the walk traced it; M6 divides it.
		sh, _, err := a.layout(pieces, keep, list, users, removed)
		if err != nil {
			return nil, nil, err
		}
		var from []piece
		for _, i := range list {
			from = append(from, pieces[keep[i].piece])
		}
		sh.sources = sourcesOf(from)
		sheets = append(sheets, sh)
	}
	return a.divide(a.quarter(a.chain(users, removed, sheets), sheets), sheets)
}

// layout traces the boundary of the face that a list of kept pieces
// makes once the removed elements are gone, in the chart of the first
// piece's carrier, and reports whether the face is simple: no point is
// visited twice at one place of the chart. An element both of whose uses
// are in the face is walked both ways, as a seam is.
//
// A face that winds fully around a cylinder has loops that do not close
// in the chart. Removed elements of the face are then put back, along a
// shortest chain from one such loop to another, until every loop closes:
// the chain is the cut along which the face unrolls. So a seam is removed
// only from a face that no longer winds around its axis (M2).
func (a *arrangement) layout(pieces []piece, keep []kept, list []int, users [][]int, removed []bool) (sheet, bool, error) {
	lead := list[0]
	for _, i := range list {
		lead = min(lead, i)
	}
	first := pieces[keep[lead].piece]
	sh := sheet{s: a.ops[first.op].faces[first.face].s, reversed: a.against(pieces, keep[lead])}
	ch := sh.s.chart()
	member := table[bool](keep)
	for _, i := range list {
		member[i] = true
	}
	for round := 0; ; round++ {
		// Into the chart of the sheet: a piece that faces the other way
		// against its own carrier is walked the other way round.
		var halves []half
		for _, i := range list {
			turned := a.against(pieces, keep[i]) != sh.reversed
			for _, loop := range pieces[keep[i].piece].loops {
				for _, h := range loop {
					if !removed[h.elem] {
						halves = append(halves, a.halfOf(ch, hop{h.elem, h.reversed != turned}))
					}
				}
			}
		}
		next, taken := filled(halves, -1), filled(halves, false)
		for i := range halves {
			k, err := a.turn(halves, i)
			if err != nil {
				return sheet{}, false, err
			}
			if taken[k] {
				return sheet{}, false, fail(ErrInconsistent, "merging", "two walks leave point %d along one element", halves[i].to)
			}
			next[i], taken[k] = k, true
		}
		var loops, open [][]hop
		done := filled(halves, false)
		for i := range halves {
			var loop []hop
			for k := i; !done[k]; k = next[k] {
				done[k] = true
				loop = append(loop, halves[k].hop)
			}
			if loop == nil {
				continue
			}
			loops = append(loops, loop)
			paths := chain(ch, legsOf(a.elems, [][]hop{loop})[0])
			if gap := paths[len(paths)-1].at(paths[len(paths)-1].t1).X - paths[0].at(paths[0].t0).X; ch.period() > 0 && math.Abs(gap) > ch.period()/2 {
				open = append(open, loop)
			}
		}
		if open != nil {
			// Cut from the first loop that does not close to any other
			// loop, closed ones included: a cut may pass by a hole.
			others := [][]hop{open[0]}
			for _, loop := range loops {
				if &loop[0] != &open[0][0] {
					others = append(others, loop)
				}
			}
			if round > len(a.elems) || !a.cut(others, users, member, removed) {
				return sheet{}, false, fail(ErrValidation, "merging", "a face that winds around its axis has no cut")
			}
			continue
		}
		// The loop of positive area is the outer one.
		var holes [][]hop
		for _, loop := range loops {
			switch area := chartArea(ch, legsOf(a.elems, [][]hop{loop})); {
			case area > 0 && sh.loops == nil:
				sh.loops = [][]hop{loop}
			case area > 0:
				return sheet{}, false, fail(ErrValidation, "merging", "a merged face has two outer loops")
			default:
				holes = append(holes, loop)
			}
		}
		if sh.loops == nil {
			return sheet{}, false, fail(ErrValidation, "merging", "a merged face has no outer loop")
		}
		sh.loops = append(sh.loops, holes...)
		// Simple: every visit of a point is at its own place.
		type visit struct {
			point int
			at    geom.Vec2
		}
		var visits []visit
		for i, paths := range place(ch, legsOf(a.elems, sh.loops)) {
			for k, p := range paths {
				from := a.elems[sh.loops[i][k].elem].a
				if sh.loops[i][k].reversed {
					from = a.elems[sh.loops[i][k].elem].b
				}
				for _, v := range visits {
					if v.point == from && math.Hypot(v.at.X-p.at(p.t0).X, v.at.Y-p.at(p.t0).Y) <= a.points[from].radius {
						return sh, false, nil
					}
				}
				visits = append(visits, visit{from, p.at(p.t0)})
			}
		}
		return sh, true, nil
	}
}

// cut puts back removed elements of a face along a shortest chain from
// the first of the given loops to any other of them. It reports whether
// there was such a chain.
func (a *arrangement) cut(open [][]hop, users [][]int, member, removed []bool) bool {
	const none, source, target = 0, 1, 2
	mark := filled(a.points, none)
	for i, loop := range open {
		for _, h := range loop {
			for _, p := range [2]int{a.elems[h.elem].a, a.elems[h.elem].b} {
				if i == 0 {
					mark[p] = source
				} else if mark[p] == none {
					mark[p] = target
				}
			}
		}
	}
	// Breadth first over the removed elements of the face, in index
	// order, from the points of the first loop.
	via := filled(a.points, -1)
	var queue []int
	for p, m := range mark {
		if m == source {
			queue = append(queue, p)
		}
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for e, el := range a.elems {
			u := users[e]
			if !removed[e] || len(u) != 2 || !member[u[0]] || !member[u[1]] || el.a != p && el.b != p {
				continue
			}
			q := el.a
			if q == p {
				q = el.b
			}
			if mark[q] == source || via[q] >= 0 {
				continue
			}
			via[q] = e
			if mark[q] == target {
				// Walk the chain back and restore it.
				for mark[q] != source {
					e := via[q]
					removed[e] = false
					if a.elems[e].a == q {
						q = a.elems[e].b
					} else {
						q = a.elems[e].a
					}
				}
				return true
			}
			queue = append(queue, q)
		}
	}
	return false
}

// oneFace reports whether two kept pieces lie on one surface and face
// the same way, so that an edge between them is removed by M1.
func (a *arrangement) oneFace(pieces []piece, x, y kept) bool {
	p, q := pieces[x.piece], pieces[y.piece]
	fp, fq := a.ops[p.op].faces[p.face], a.ops[q.op].faces[q.face]
	switch {
	case p.op == q.op && p.face == q.face:
	case p.op == q.op:
		// Two faces of one operand: the carriers must be alike and the
		// corners of each must lie on the other, as in SP4.
		pairing, err := surfacePair(fp.s, fq.s, a.t)
		if err != nil || !pairing.alike {
			return false
		}
		if same, err := sameSurface([2]surface{fp.s, fq.s}, [2][]point{a.corners(p), a.corners(q)}); err != nil || !same {
			return false
		}
	default:
		same := false
		for _, pair := range a.pairs {
			same = same || pair.faces[p.op] == p.face && pair.faces[q.op] == q.face
		}
		if !same {
			return false
		}
	}
	at := a.points[a.elems[p.loops[0][0].elem].a].pos
	return (fp.s.normal(at).Dot(fq.s.normal(at)) < 0) == (a.against(pieces, x) != a.against(pieces, y))
}

// corners lists the points on the boundary of a piece.
func (a *arrangement) corners(p piece) []point {
	var out []point
	for _, loop := range p.loops {
		for _, h := range loop {
			out = append(out, a.points[a.elems[h.elem].a])
		}
	}
	return out
}

// chain applies M4 after M1: a point with exactly two remaining elements
// on one carrier is removed and the two become one edge. It returns the
// edges of the result and rewrites the loops of the sheets over them.
func (a *arrangement) chain(users [][]int, removed []bool, sheets []sheet) []element {
	alive := func(e int) bool { return users[e] != nil && !removed[e] }
	incident := table[[]int](a.points)
	for e, el := range a.elems {
		if alive(e) {
			incident[el.a] = append(incident[el.a], e)
			incident[el.b] = append(incident[el.b], e)
		}
	}
	gone := table[bool](a.points)
	for p, list := range incident {
		gone[p] = len(list) == 2 && list[0] != list[1] && a.oneCurve(list[0], list[1], p)
	}
	edgeOf, flip := filled(a.elems, -1), table[bool](a.elems)
	var edges []element
	next := 0 // the index of the edge being made
	for e, el := range a.elems {
		if !alive(e) || edgeOf[e] >= 0 {
			continue
		}
		// e is the lowest element of its chain, so the edge takes its
		// carrier and direction. Walk out from both of its ends through
		// the points that go.
		ends := [2]int{el.a, el.b}
		for side := range ends {
			for x := e; gone[ends[side]]; {
				p := ends[side]
				y := incident[p][0]
				if y == x {
					y = incident[p][1]
				}
				if y == e {
					gone[p] = false // a closed chain keeps one point
					break
				}
				q := a.elems[y].a
				if q == p {
					q = a.elems[y].b
				}
				// Along the edge, y runs from q to p before e and from p
				// to q after it.
				edgeOf[y], flip[y] = next, (a.elems[y].a == p) == (side == 0)
				ends[side], x = q, y
			}
		}
		edgeOf[e] = next
		next++
		// The interval of the edge on the carrier of e. On a closed
		// carrier it runs forward from its start, a whole period when
		// the chain is closed.
		t0, t1 := el.c.param(a.points[ends[0]].pos), el.c.param(a.points[ends[1]].pos)
		if period := el.c.period(); period > 0 {
			t0 += period * math.Floor((el.t0-t0)/period+1e-9)
			if t1 += period * math.Ceil((el.t1-t1)/period-1e-9); ends[0] == ends[1] {
				t1 = t0 + period
			}
		}
		edges = append(edges, element{el.c, t0, t1, ends[0], ends[1]})
	}
	for i := range sheets {
		for j, loop := range sheets[i].loops {
			var out []hop
			for _, h := range loop {
				from := a.elems[h.elem].a
				if h.reversed {
					from = a.elems[h.elem].b
				}
				if !gone[from] {
					out = append(out, hop{edgeOf[h.elem], h.reversed != flip[h.elem]})
				}
			}
			sheets[i].loops[j] = out
		}
	}
	return edges
}

// oneCurve reports whether two elements that meet at point p lie on one
// carrier: they leave p in parallel directions, and the far end and the
// middle of each lie on the carrier of the other.
func (a *arrangement) oneCurve(e1, e2, p int) bool {
	x, y := a.elems[e1], a.elems[e2]
	at := a.points[p].pos
	if !a.t.parallel(x.c.tangent(x.c.param(at)), y.c.tangent(y.c.param(at))) {
		return false
	}
	for _, pair := range [2][2]element{{x, y}, {y, x}} {
		el, other := pair[0], pair[1]
		q := a.points[el.a]
		if el.a == p {
			q = a.points[el.b]
		}
		if off(other.c, q.pos) > q.radius || off(other.c, el.c.at((el.t0+el.t1)/2)) > a.t.eps {
			return false
		}
	}
	return true
}
