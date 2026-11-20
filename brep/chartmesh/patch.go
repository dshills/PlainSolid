package chartmesh

import (
	"errors"
	"math"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// patch is the mesh of one face in its chart: the boundary points of its
// loops, the outer one first and counterclockwise, the holes clockwise,
// then any inner points, each with its vertex of the solid's mesh.
type patch struct {
	at        []geom.Vec2
	ids       []int
	rings     [][]int
	triangles [][3]int
	fixed     map[[2]int]bool // the sides on the boundary
	// squash is what the second chart coordinate is multiplied by when
	// relax judges which of two pairs of triangles is the better.
	squash float64
}

var errNoEar = errors.New("the boundary does not triangulate")

func side(a, b int) [2]int {
	if a > b {
		a, b = b, a
	}
	return [2]int{a, b}
}

// turn is the sign of the area of the chart triangle a, b, c: positive
// when it is counterclockwise. A value the rounding of the products could
// have decided is decided exactly.
func (m *patch) turn(a, b, c int) int {
	p, q, r := m.at[a], m.at[b], m.at[c]
	// Points of one rim of a cylinder have one height to the last bit,
	// and points of one ruling one place round the axis: no arithmetic
	// is needed to see that three of them are in line.
	if p.Y == q.Y && q.Y == r.Y || p.X == q.X && q.X == r.X {
		return 0
	}
	x, y := (q.X-p.X)*(r.Y-p.Y), (q.Y-p.Y)*(r.X-p.X)
	if d := x - y; math.Abs(d) > 8e-16*(math.Abs(x)+math.Abs(y)) {
		if d > 0 {
			return 1
		}
		return -1
	}
	s, _ := geom.Orient2D(p, q, r)
	return s
}

// within reports whether point q, which lies on the line through a and b,
// lies between them.
func (m *patch) within(a, b, q int) bool {
	p, r, x := m.at[a], m.at[b], m.at[q]
	return x.X >= math.Min(p.X, r.X) && x.X <= math.Max(p.X, r.X) && x.Y >= math.Min(p.Y, r.Y) && x.Y <= math.Max(p.Y, r.Y)
}

// inside reports whether q is in the corner of the region at v, between
// the boundary arriving from before and leaving to after.
func (m *patch) inside(before, v, after, q int) bool {
	a, b := m.turn(before, v, q), m.turn(v, after, q)
	if m.turn(before, v, after) > 0 {
		return a > 0 && b > 0
	}
	return a > 0 || b > 0
}

// blocked reports whether the segment from p to q meets a boundary side
// anywhere but at its own ends.
func (m *patch) blocked(p, q int, rings ...[]int) bool {
	for _, ring := range rings {
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			switch {
			case a == p || a == q:
				if m.turn(p, q, b) == 0 && b != p && b != q && m.within(p, q, b) {
					return true
				}
			case b == p || b == q:
				if m.turn(p, q, a) == 0 && m.within(p, q, a) {
					return true
				}
			default:
				s1, s2, s3, s4 := m.turn(p, q, a), m.turn(p, q, b), m.turn(a, b, p), m.turn(a, b, q)
				if s1*s2 < 0 && s3*s4 < 0 ||
					s1 == 0 && m.within(p, q, a) || s2 == 0 && m.within(p, q, b) ||
					s3 == 0 && m.within(a, b, p) || s4 == 0 && m.within(a, b, q) {
					return true
				}
			}
		}
	}
	return false
}

// triangulate fills the region the loops bound with triangles on the
// boundary points alone. Each hole is first joined to the outline by a
// side walked both ways, the hole that reaches farthest along the first
// coordinate first, since nothing then stands between it and the outline
// that way. The one outline that results is cut ear by ear: a corner
// that turns to the left with no other point in or on its triangle.
func (m *patch) triangulate() error {
	m.fixed = map[[2]int]bool{}
	for _, ring := range m.rings {
		if len(ring) < 3 {
			return errNoEar
		}
		for i, a := range ring {
			m.fixed[side(a, ring[(i+1)%len(ring)])] = true
		}
	}
	outline := append([]int{}, m.rings[0]...)
	holes := append([][]int{}, m.rings[1:]...)
	for len(holes) > 0 {
		pick, corner := 0, 0
		for h, hole := range holes {
			for j, p := range hole {
				if m.at[p].X > m.at[holes[pick][corner]].X {
					pick, corner = h, j
				}
			}
		}
		hole := holes[pick]
		from := hole[corner]
		order := make([]int, len(outline))
		for i := range order {
			order[i] = i
		}
		far := func(i int) float64 {
			p := m.at[outline[i]]
			return math.Hypot(p.X-m.at[from].X, p.Y-m.at[from].Y)
		}
		sort.SliceStable(order, func(i, j int) bool { return far(order[i]) < far(order[j]) })
		found := -1
		n := len(outline)
		for _, i := range order {
			to := outline[i]
			if m.inside(outline[(i+n-1)%n], to, outline[(i+1)%n], from) &&
				m.inside(hole[(corner+len(hole)-1)%len(hole)], from, hole[(corner+1)%len(hole)], to) &&
				!m.blocked(from, to, append([][]int{outline}, holes...)...) {
				found = i
				break
			}
		}
		if found < 0 {
			return errNoEar
		}
		joined := append([]int{}, outline[:found+1]...)
		joined = append(append(joined, hole[corner:]...), hole[:corner]...)
		joined = append(append(joined, from, outline[found]), outline[found+1:]...)
		outline = joined
		holes = append(holes[:pick], holes[pick+1:]...)
	}
	n := len(outline)
	before, after := make([]int, n), make([]int, n)
	for i := range outline {
		before[i], after[i] = (i+n-1)%n, (i+1)%n
	}
	ear := func(i int) bool {
		a, b, c := outline[before[i]], outline[i], outline[after[i]]
		if m.turn(a, b, c) <= 0 {
			return false
		}
		lo := geom.Vec2{X: math.Min(m.at[a].X, math.Min(m.at[b].X, m.at[c].X)), Y: math.Min(m.at[a].Y, math.Min(m.at[b].Y, m.at[c].Y))}
		hi := geom.Vec2{X: math.Max(m.at[a].X, math.Max(m.at[b].X, m.at[c].X)), Y: math.Max(m.at[a].Y, math.Max(m.at[b].Y, m.at[c].Y))}
		for j := after[after[i]]; j != before[i]; j = after[j] {
			q := outline[j]
			if p := m.at[q]; q == a || q == b || q == c || p.X < lo.X || p.X > hi.X || p.Y < lo.Y || p.Y > hi.Y {
				continue
			}
			if m.turn(a, b, q) >= 0 && m.turn(b, c, q) >= 0 && m.turn(c, a, q) >= 0 {
				return false
			}
		}
		return true
	}
	left, missed := n, 0
	for i := 0; left > 3; {
		if !ear(i) {
			if i, missed = after[i], missed+1; missed > left {
				return errNoEar
			}
			continue
		}
		m.triangles = append(m.triangles, [3]int{outline[before[i]], outline[i], outline[after[i]]})
		after[before[i]], before[after[i]] = after[i], before[i]
		left, missed, i = left-1, 0, after[i]
		// i is now the corner after the one cut off; the next ear is
		// looked for one further on, which keeps the triangles from
		// fanning about one point.
		i = after[i]
	}
	// The three corners left. The loop leaves i at one of them only when
	// it cut an ear; any corner of the outline before that will do.
	last := 0
	for k := range outline {
		if after[before[k]] == k && before[after[k]] == k {
			last = k
			break
		}
	}
	a, b, c := outline[before[last]], outline[last], outline[after[last]]
	if m.turn(a, b, c) <= 0 {
		return errNoEar
	}
	m.triangles = append(m.triangles, [3]int{a, b, c})
	return nil
}

// relax flips an inner side shared by two triangles when the corner
// opposite it in one lies inside the circle through the corners of the
// other, and goes on with the four sides around each flip until none is
// left to flip. A side on the boundary is never flipped.
//
// The circle is drawn in the chart with its second coordinate squashed.
// How far a facet of a cylinder leaves the surface depends on how far it
// goes round the axis and not on how far along it, so the triangles
// wanted are those that are narrow round the axis, however long. In the
// chart as it is, a point of a hole half way up a tall wall is nearer
// the points of both rims for a long way round than those are to the
// other rim, and takes them all into one fan. Squashed, the two rims and
// the hole lie a few steps apart, and each point joins those beside it
// round the axis.
func (m *patch) relax() {
	// The two triangles on each side, or -1.
	seats := make(map[[2]int][2]int, 3*len(m.triangles)/2)
	seat := func(key [2]int, from, to int) {
		pair, ok := seats[key]
		if !ok {
			pair = [2]int{-1, -1}
		}
		for i := range pair {
			if pair[i] == from {
				pair[i] = to
				break
			}
		}
		seats[key] = pair
	}
	var queue [][2]int
	for t, tri := range m.triangles {
		for k := range 3 {
			key := side(tri[k], tri[(k+1)%3])
			seat(key, -1, t)
			queue = append(queue, key)
		}
	}
	for flips := 0; len(queue) > 0 && flips < 64*len(m.triangles)+1024; {
		key := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		pair, ok := seats[key]
		if !ok || m.fixed[key] || pair[0] < 0 || pair[1] < 0 {
			continue
		}
		// The first triangle as a, b, c with the side from a to b, and
		// the corner d of the second opposite it.
		tri, other := m.triangles[pair[0]], m.triangles[pair[1]]
		k := 0
		for side(tri[k], tri[(k+1)%3]) != key {
			k++
		}
		a, b, c := tri[k], tri[(k+1)%3], tri[(k+2)%3]
		d := other[0]
		for _, q := range other {
			if q != a && q != b {
				d = q
			}
		}
		if m.turn(c, a, d) <= 0 || m.turn(d, b, c) <= 0 || !m.encircled(a, b, c, d) {
			continue
		}
		m.triangles[pair[0]], m.triangles[pair[1]] = [3]int{c, a, d}, [3]int{d, b, c}
		delete(seats, key)
		seats[side(c, d)] = pair
		seat(side(a, d), pair[1], pair[0])
		seat(side(b, c), pair[0], pair[1])
		queue = append(queue, side(c, a), side(a, d), side(d, b), side(b, c))
		flips++
	}
}

// encircled reports whether d lies inside the circle through the corners
// of the counterclockwise triangle a, b, c, by more than rounding.
func (m *patch) encircled(a, b, c, d int) bool {
	p := m.at[d]
	ax, ay := m.at[a].X-p.X, (m.at[a].Y-p.Y)*m.squash
	bx, by := m.at[b].X-p.X, (m.at[b].Y-p.Y)*m.squash
	cx, cy := m.at[c].X-p.X, (m.at[c].Y-p.Y)*m.squash
	a2, b2, c2 := ax*ax+ay*ay, bx*bx+by*by, cx*cx+cy*cy
	det := a2*(bx*cy-by*cx) - b2*(ax*cy-ay*cx) + c2*(ax*by-ay*bx)
	size := a2*(math.Abs(bx*cy)+math.Abs(by*cx)) + b2*(math.Abs(ax*cy)+math.Abs(ay*cx)) + c2*(math.Abs(ax*by)+math.Abs(ay*bx))
	return det > 1e-10*size
}

// refine halves inner sides of a cylindrical face until no triangle spans
// more of the turn about the axis than the face allows and, where a facet
// angle is asked for, none has a normal farther than that from the
// surface normal at a corner. Of a triangle that fails, the inner side
// that goes farthest round the axis is halved, since that is what both
// bounds grow with. A triangle whose inner sides all go less than a
// sixteenth of the span is left as it is: it lies along the boundary,
// whose points are not this face's to add to.
func (m *patch) refine(c face, o parametric.TessellationOptions, out *mesh.Mesh) error {
	normals := map[int]geom.Vec3{}
	normal := func(i int) (geom.Vec3, error) {
		if n, ok := normals[i]; ok {
			return n, nil
		}
		du, dv, err := c.surface.Partials(m.at[i].X/c.radius, m.at[i].Y)
		if err != nil {
			return geom.Vec3{}, err
		}
		n, err := du.Cross(dv).Unit()
		normals[i] = n
		return n, err
	}
	poor := func(t [3]int) (bool, error) {
		lo := math.Min(m.at[t[0]].X, math.Min(m.at[t[1]].X, m.at[t[2]].X))
		hi := math.Max(m.at[t[0]].X, math.Max(m.at[t[1]].X, m.at[t[2]].X))
		if hi-lo > c.limit*(1+1e-9) {
			return true, nil
		}
		if o.Angle == 0 {
			return false, nil
		}
		p := [3]geom.Vec3{out.Vertices[m.ids[t[0]]], out.Vertices[m.ids[t[1]]], out.Vertices[m.ids[t[2]]]}
		flat, err := p[1].Sub(p[0]).Cross(p[2].Sub(p[0])).Unit()
		if err != nil {
			return true, nil
		}
		for _, i := range t {
			n, err := normal(i)
			if err != nil {
				return false, err
			}
			if math.Atan2(n.Cross(flat).Length(), n.Dot(flat)) > o.Angle {
				return true, nil
			}
		}
		return false, nil
	}
	// The face squashed is four spans tall.
	lo, hi := m.at[0].Y, m.at[0].Y
	for _, p := range m.at {
		lo, hi = math.Min(lo, p.Y), math.Max(hi, p.Y)
	}
	m.squash = math.Min(1, 4*c.limit/(hi-lo))
	m.relax()
	for range refinements {
		halved := map[[2]int]int{}
		var keys [][2]int
		for _, t := range m.triangles {
			bad, err := poor(t)
			if err != nil {
				return err
			}
			if !bad {
				continue
			}
			best, span := -1, c.limit/16
			for k := range 3 {
				a, b := t[k], t[(k+1)%3]
				if s := math.Abs(m.at[a].X - m.at[b].X); !m.fixed[side(a, b)] && s > span {
					best, span = k, s
				}
			}
			if key := side(t[max(best, 0)], t[(max(best, 0)+1)%3]); best >= 0 && halved[key] == 0 {
				halved[key] = -1
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			return nil
		}
		if len(m.triangles)+2*len(keys) > o.MaxTriangles {
			return parametric.ErrBudget
		}
		for _, key := range keys {
			p, q := m.at[key[0]], m.at[key[1]]
			middle := geom.Vec2{X: (p.X + q.X) / 2, Y: (p.Y + q.Y) / 2}
			x, err := c.surface.Eval(middle.X/c.radius, middle.Y)
			if err != nil {
				return err
			}
			halved[key] = len(m.at)
			m.at, m.ids = append(m.at, middle), append(m.ids, len(out.Vertices))
			out.Vertices = append(out.Vertices, x)
		}
		var next [][3]int
		for _, t := range m.triangles {
			// The middles of the three sides, or -1, and how many.
			var mid [3]int
			count, unmarked, marked := 0, 0, 0
			for k := range 3 {
				mid[k] = -1
				if at, ok := halved[side(t[k], t[(k+1)%3])]; ok {
					mid[k], count, marked = at, count+1, k
				} else {
					unmarked = k
				}
			}
			switch count {
			case 0:
				next = append(next, t)
			case 1:
				a, b, c := t[marked], t[(marked+1)%3], t[(marked+2)%3]
				next = append(next, [3]int{a, mid[marked], c}, [3]int{mid[marked], b, c})
			case 2:
				// Turned so that the whole side runs from c back to a.
				k := (unmarked + 1) % 3
				a, b, c := t[k], t[(k+1)%3], t[(k+2)%3]
				ab, bc := mid[k], mid[(k+1)%3]
				next = append(next, [3]int{ab, b, bc}, [3]int{a, ab, bc}, [3]int{a, bc, c})
			default:
				next = append(next, [3]int{t[0], mid[0], mid[2]}, [3]int{mid[0], t[1], mid[1]}, [3]int{mid[2], mid[1], t[2]}, [3]int{mid[0], mid[1], mid[2]})
			}
		}
		m.triangles = next
		m.relax()
	}
	return nil
}
