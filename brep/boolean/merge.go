package boolean

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
	// start is the point a use leaves when its piece is seen from
	// outside, where every kept piece keeps its region on the left.
	start := func(k kept, h hop) int {
		if h.reversed != a.against(pieces, k) {
			return a.elems[h.elem].b
		}
		return a.elems[h.elem].a
	}
	leaving := filled(a.points, 0)
	// pinched reports whether the face made of groups x and y, without
	// the elements between them, would visit a point twice (M3).
	pinched := func(x, y int) bool {
		twice := false
		for _, add := range [2]int{1, -1} {
			for _, g := range [2]int{x, y} {
				for _, i := range members[g] {
					for _, loop := range pieces[keep[i].piece].loops {
						for _, h := range loop {
							u := users[h.elem]
							if removed[h.elem] || removable[h.elem] && group[u[0]] != group[u[1]] && (group[u[0]] == x || group[u[0]] == y) && (group[u[1]] == x || group[u[1]] == y) {
								continue
							}
							p := start(keep[i], h)
							if leaving[p] += add; leaving[p] > 1 {
								twice = true
							}
						}
					}
				}
			}
		}
		return twice
	}
	for e, u := range users {
		if !removable[e] || removed[e] {
			continue
		}
		x, y := group[u[0]], group[u[1]]
		if x != y && pinched(x, y) {
			continue // M3: this merge is not performed
		}
		if y < x {
			x, y = y, x
		}
		for f, v := range users {
			if removable[f] && (group[v[0]] == x || group[v[0]] == y) && (group[v[1]] == x || group[v[1]] == y) {
				removed[f] = true
			}
		}
		if x != y {
			for _, i := range members[y] {
				group[i] = x
			}
			members[x], members[y] = append(members[x], members[y]...), nil
		}
	}
	// One sheet for every group, in the order of its first piece, with
	// the carrier of that piece (M1) and the sources of all (P5).
	var sheets []sheet
	leave := filled(a.points, -1)
	for g, list := range members {
		if list == nil {
			continue
		}
		lead := pieces[keep[g].piece]
		sh := sheet{s: a.ops[lead.op].faces[lead.face].s, reversed: a.against(pieces, keep[g])}
		var from []piece
		var steps []hop // seen from outside: reversed means from b to a
		for _, i := range list {
			from = append(from, pieces[keep[i].piece])
			for _, loop := range pieces[keep[i].piece].loops {
				for _, h := range loop {
					if !removed[h.elem] {
						leave[start(keep[i], h)] = len(steps)
						steps = append(steps, hop{h.elem, h.reversed != a.against(pieces, keep[i])})
					}
				}
			}
		}
		sh.sources = sourcesOf(from)
		done := table[bool](steps)
		var holes [][]hop
		for i := range steps {
			var loop []hop
			for k := i; !done[k]; {
				done[k] = true
				// Into the chart of the sheet: a sheet that faces
				// against its carrier is walked the other way round.
				h, end := steps[k], a.elems[steps[k].elem].b
				if h.reversed {
					end = a.elems[h.elem].a
				}
				if sh.reversed {
					loop = append([]hop{{h.elem, !h.reversed}}, loop...)
				} else {
					loop = append(loop, h)
				}
				k = leave[end]
			}
			switch area := chartArea(sh.s.chart(), legsOf(a.elems, [][]hop{loop})); {
			case loop == nil:
			case area > 0 && sh.loops == nil:
				sh.loops = [][]hop{loop}
			case area > 0:
				return nil, nil, fail(ErrValidation, "merging", "a merged face has two outer loops")
			default:
				holes = append(holes, loop)
			}
		}
		if sh.loops == nil {
			return nil, nil, fail(ErrValidation, "merging", "a merged face has no outer loop")
		}
		sh.loops = append(sh.loops, holes...)
		sheets = append(sheets, sh)
	}
	edges := a.chain(users, removed, sheets)
	return edges, sheets, nil
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
		edges = append(edges, element{el.c, el.c.param(a.points[ends[0]].pos), el.c.param(a.points[ends[1]].pos), ends[0], ends[1]})
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
