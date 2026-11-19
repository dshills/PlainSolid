package boolean

import "plainsolid/geom"

// against tells whether a kept piece faces against the normal of its
// carrier: its input face does, or the selection reversed it, not both.
func (a *arrangement) against(pieces []piece, k kept) bool {
	p := pieces[k.piece]
	return a.ops[p.op].faces[p.face].reversed != k.flip
}

// manifold is the test of section 7.9 on the kept pieces, before
// merging.
func (a *arrangement) manifold(pieces []piece, keep []kept) error {
	// N1: every element a kept piece uses has exactly two uses by kept
	// pieces, in opposite directions seen from outside.
	uses, forward := filled(a.elems, 0), filled(a.elems, 0)
	// N2: at every point, the kept pieces around it form one cycle. Each
	// end of an element is a slot; a corner of a piece joins the slot it
	// arrives by to the slot it leaves by, and the slots at a point must
	// then all be joined.
	slots := filled(a.elems, [2]int{-1, -1})
	next := 0
	for i := range slots {
		slots[i] = [2]int{next, next + 1}
		next += 2
	}
	group := make([]int, next)
	for i := range group {
		group[i] = i
	}
	find := func(i int) int {
		for group[i] != i {
			i = group[i]
		}
		return i
	}
	slot := func(h hop, leaving bool) int {
		if h.reversed != leaving {
			return slots[h.elem][0]
		}
		return slots[h.elem][1]
	}
	for _, k := range keep {
		for _, loop := range pieces[k.piece].loops {
			for i, h := range loop {
				uses[h.elem]++
				if h.reversed != a.against(pieces, k) {
					forward[h.elem]++
				}
				if x, y := find(slot(h, false)), find(slot(cyclic(loop, i+1), true)); x != y {
					group[max(x, y)] = min(x, y)
				}
			}
		}
	}
	at := filled(a.points, -1)
	for e, n := range uses {
		if n == 0 {
			continue
		}
		el := a.elems[e]
		if n != 2 || forward[e] != 1 {
			return fail(ErrNonManifold, "N1", "the edge from %v to %v has %d uses by kept faces, %d of them forward", a.points[el.a].pos, a.points[el.b].pos, n, forward[e])
		}
		for i, p := range [2]int{el.a, el.b} {
			if g := find(slots[e][i]); at[p] < 0 {
				at[p] = g
			} else if at[p] != g {
				return fail(ErrNonManifold, "N2", "the kept faces at %v do not form one cycle", a.points[p].pos)
			}
		}
	}
	// N3: at a touching contact, kept pieces of A and kept pieces of B
	// do not both contain the locus.
	for _, c := range a.contacts {
		locus, radius := a.locus(c)
		var holds [2]bool
		for _, k := range keep {
			p := pieces[k.piece]
			for _, f := range c.faces[p.op] {
				if f == p.face && locate(a.ops[p.op].faces[f].s.chart(), legsOf(a.elems, p.loops), locus, radius) != without {
					holds[p.op] = true
				}
			}
		}
		if holds[0] && holds[1] {
			return fail(ErrNonManifold, "N3", "the operands touch at %v without crossing", locus)
		}
	}
	return nil
}

// locus gives a point of a touching contact and the radius it is tested
// with: the point itself, or the middle of the element.
func (a *arrangement) locus(c contact) (geom.Vec3, float64) {
	if c.point >= 0 {
		return a.points[c.point].pos, a.points[c.point].radius
	}
	e := a.elems[c.elem]
	return e.c.at((e.t0 + e.t1) / 2), a.t.eps
}
