package boolean

import (
	"math"
	"sort"

	"plainsolid/geom"
)

// half is one direction of an element in the chart of a face: a boundary
// element has the half of its loop direction, an element inside the face
// has both.
type half struct {
	hop
	from, to int       // points
	out      geom.Vec2 // chart direction in which it leaves from
	bend     float64   // its signed curvature there, positive to the left
	back     geom.Vec2 // chart direction in which it would leave to, walked backwards
	backBend float64
}

// split cuts every input face along the elements that lie in it and
// returns the pieces (section 7.6), in the order of operands and faces.
func (a *arrangement) split() ([]piece, error) {
	var out []piece
	for op := range a.ops {
		for f := range a.rims[op] {
			pieces, err := a.splitFace(op, f)
			if err != nil {
				return nil, failAt(err, "face %d of %s", f, operandName(op))
			}
			if out = append(out, pieces...); len(out) > a.limits.MaxPieces {
				return nil, fail(ErrBudget, "splitting", "more than MaxPieces %d pieces", a.limits.MaxPieces)
			}
		}
	}
	return out, nil
}

// splitFace applies F1 to F6 to one face.
func (a *arrangement) splitFace(op, f int) ([]piece, error) {
	ch := a.ops[op].faces[f].s.chart()
	// F1: the boundary, and each element inside the face once.
	inside := append([]int(nil), a.inner[op][f]...)
	sort.Ints(inside)
	kept := inside[:0]
	for i, e := range inside {
		if i == 0 || e != inside[i-1] {
			kept = append(kept, e)
		}
	}
	// F2: discard dangling chains. An element discarded must be a
	// touching contact or an edge of the other operand lying in the face.
	degree := filled(a.points, 0)
	for _, loop := range a.rims[op][f] {
		for _, h := range loop {
			degree[a.elems[h.elem].a]++
			degree[a.elems[h.elem].b]++
		}
	}
	for _, e := range kept {
		degree[a.elems[e].a]++
		degree[a.elems[e].b]++
	}
	for changed := true; changed; {
		changed = false
		for i, e := range kept {
			if e < 0 || degree[a.elems[e].a] > 1 && degree[a.elems[e].b] > 1 {
				continue
			}
			if !a.mayDangle(e) {
				return nil, fail(ErrInconsistent, "F2", "a trace ends inside the face")
			}
			degree[a.elems[e].a]--
			degree[a.elems[e].b]--
			kept[i], changed = -1, true
		}
	}
	// F3: trace the graph into loops that keep their region on the left.
	var halves []half
	add := func(h hop) {
		e := a.elems[h.elem]
		l := leg{e.c, e.t0, e.t1}
		x := half{hop: h, from: e.a, to: e.b}
		if h.reversed {
			l, x.from, x.to = l.back(), e.b, e.a
		}
		x.out, x.bend = ch.heading(l)
		x.back, x.backBend = ch.heading(l.back())
		halves = append(halves, x)
	}
	for _, loop := range a.rims[op][f] {
		for _, h := range loop {
			add(h)
		}
	}
	for _, e := range kept {
		if e >= 0 {
			add(hop{e, false})
			add(hop{e, true})
		}
	}
	next := filled(halves, -1)
	taken := filled(halves, false)
	for i, h := range halves {
		k, err := a.turn(halves, i)
		if err != nil {
			return nil, err
		}
		if taken[k] {
			return nil, fail(ErrInconsistent, "F3", "two walks leave point %d along one element", h.to)
		}
		next[i], taken[k] = k, true
	}
	var loops [][]hop
	done := filled(halves, false)
	for i := range halves {
		if done[i] {
			continue
		}
		var loop []hop
		for k := i; !done[k]; k = next[k] {
			done[k] = true
			loop = append(loop, halves[k].hop)
		}
		loops = append(loops, loop)
	}
	// F4: a loop of positive chart area bounds a piece; a loop of
	// negative area is a hole of the smallest piece that contains it.
	areas := table[float64](loops)
	var pieces []piece
	owner := filled(loops, -1)
	for i, loop := range loops {
		if areas[i] = polygonArea(ring(ch, legsOf(a.elems, [][]hop{loop})[0])); areas[i] > 0 {
			owner[i] = len(pieces)
			pieces = append(pieces, piece{op: op, face: f, loops: [][]hop{loop}, class: outside, mate: -1})
		}
	}
	for i, loop := range loops {
		if areas[i] > 0 {
			continue
		}
		best := -1
		for j := range loops {
			if areas[j] > -areas[i] && (best < 0 || areas[j] < areas[best]) && a.holds(ch, loops[j], loop) {
				best = j
			}
		}
		if best < 0 {
			return nil, fail(ErrInconsistent, "F4", "a hole lies in no piece")
		}
		pieces[owner[best]].loops = append(pieces[owner[best]].loops, loop)
	}
	// F6: every piece has an interior point farther than ε from its
	// boundary.
	for _, p := range pieces {
		if err := a.ctx.Err(); err != nil {
			return nil, err
		}
		if a.probes(p) == nil {
			return nil, fail(ErrTolerance, "F6", "a piece has no interior point farther than the tolerance from its boundary")
		}
	}
	return pieces, nil
}

// mayDangle reports whether F2 may discard an element: it is a touching
// contact, or it is a span and not a trace.
func (a *arrangement) mayDangle(e int) bool {
	for _, c := range a.contacts {
		if c.elem == e {
			return true
		}
	}
	for _, tr := range a.traces {
		if tr.elem == e {
			return false
		}
	}
	return true
}

// turn gives the half that follows half i at its end point (F3): the
// first other half met when turning clockwise from the direction of
// arrival reversed. Halves that leave in one direction within the
// angular tolerance are ordered by curvature, the one curving most to
// the left first; two that agree in curvature as well are one carrier
// and should have been one bundle.
func (a *arrangement) turn(halves []half, i int) (int, error) {
	arrived := halves[i]
	// angle is the clockwise angle from the reversed arrival direction,
	// in (0, 2π]; a direction that agrees with it is placed by curvature.
	angle := func(h half) (float64, error) {
		cross := arrived.back.X*h.out.Y - arrived.back.Y*h.out.X
		dot := arrived.back.X*h.out.X + arrived.back.Y*h.out.Y
		if math.Abs(cross) > a.t.angular || dot < 0 {
			return math.Mod(2*math.Pi-math.Atan2(cross, dot), 2*math.Pi), nil
		}
		switch d := (h.bend - arrived.backBend) * a.t.size; {
		case d > a.t.angular:
			return 2 * math.Pi, nil
		case d < -a.t.angular:
			return 0, nil
		}
		return 0, fail(ErrInconsistent, "F3", "two elements leave point %d along one carrier and are not one bundle", arrived.to)
	}
	best, least := -1, 0.
	for k, h := range halves {
		if h.from != arrived.to || h.elem == arrived.elem && h.reversed != arrived.reversed {
			continue // not at this point, or the way back along the element of arrival
		}
		at, err := angle(h)
		if err != nil {
			return -1, err
		}
		if best >= 0 {
			cross := halves[best].out.X*h.out.Y - halves[best].out.Y*h.out.X
			dot := halves[best].out.X*h.out.X + halves[best].out.Y*h.out.Y
			if math.Abs(cross) <= a.t.angular && dot > 0 {
				// One direction: the one curving most to the left first.
				d := (h.bend - halves[best].bend) * a.t.size
				if math.Abs(d) <= a.t.angular {
					return -1, fail(ErrInconsistent, "F3", "two elements leave point %d along one carrier and are not one bundle", arrived.to)
				}
				if d > 0 {
					best, least = k, math.Min(at, least)
				}
				continue
			}
		}
		if best < 0 || at < least {
			best, least = k, at
		}
	}
	if best < 0 {
		return -1, fail(ErrInconsistent, "F3", "a walk cannot leave point %d", arrived.to)
	}
	return best, nil
}

// holds reports whether the region inside an outer loop contains a hole
// loop: no point of the hole is outside it and one is strictly inside.
func (a *arrangement) holds(ch chart, outer, hole []hop) bool {
	border := legsOf(a.elems, [][]hop{outer})
	inside := false
	for _, l := range legsOf(a.elems, [][]hop{hole})[0] {
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

// probes gives up to five interior points of a piece that are farther
// than ε from its boundary, the one with the most clearance first, or
// nil when there is none (F6). Candidates are the middles of the chords
// that chart lines through the piece cut out of it, across both chart
// directions, between every two consecutive corners.
func (a *arrangement) probes(p piece) []geom.Vec3 {
	ch := a.ops[p.op].faces[p.face].s.chart()
	border := legsOf(a.elems, p.loops)
	var rings [][]geom.Vec2
	for _, loop := range border {
		rings = append(rings, ring(ch, loop))
	}
	type candidate struct {
		at    geom.Vec3
		clear float64
	}
	var found []candidate
	for axis := 0; axis < 2; axis++ {
		pick := func(q geom.Vec2) (float64, float64) {
			if axis == 0 {
				return q.X, q.Y
			}
			return q.Y, q.X
		}
		var levels []float64
		for _, r := range rings {
			for _, q := range r {
				_, v := pick(q)
				levels = append(levels, v)
			}
		}
		sort.Float64s(levels)
		for i := 0; i+1 < len(levels); i++ {
			if levels[i] == levels[i+1] {
				continue
			}
			level := (levels[i] + levels[i+1]) / 2
			var cuts []float64
			for _, r := range rings {
				for k, q := range r {
					u0, v0 := pick(q)
					u1, v1 := pick(cyclic(r, k+1))
					if (v0 > level) != (v1 > level) {
						cuts = append(cuts, u0+(level-v0)/(v1-v0)*(u1-u0))
					}
				}
			}
			sort.Float64s(cuts)
			for k := 0; k+1 < len(cuts); k += 2 {
				q := geom.Vec2{X: (cuts[k] + cuts[k+1]) / 2, Y: level}
				if axis == 1 {
					q = geom.Vec2{X: level, Y: (cuts[k] + cuts[k+1]) / 2}
				}
				at := ch.lift(q)
				clear := math.Inf(1)
				for _, loop := range border {
					for _, l := range loop {
						clear = math.Min(clear, at.Distance(l.nearest(at)))
					}
				}
				if clear > a.t.eps {
					found = append(found, candidate{at, clear})
				}
			}
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].clear > found[j].clear })
	var out []geom.Vec3
	for i, c := range found {
		if i < 5 {
			out = append(out, c.at)
		}
	}
	return out
}
