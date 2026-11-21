package boolean

import (
	"context"
	"math"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// sheet is one face of the result under construction: a region of one
// surface carrier. Its loops are hops over the edge list handed to
// assemble, keep the region on their left in the carrier's chart, and
// start with the outer loop.
type sheet struct {
	s        surface
	reversed bool // the outward normal is opposite to the carrier normal
	loops    [][]hop
	sources  [][2]int
}

// body is the assembled result (section 7.11).
type body struct {
	solid  trimmed.Solid
	tau    float64 // the tolerance the solid records (T6)
	slack  float64 // the allowance for rounding in a comparison with tau
	verts  []geom.Vec3
	edges  []parametric.Edge
	faces  []trimmed.Face
	sheets []sheet // in the order of faces
	shells [][]int
	voids  []bool
	owners []int
	solids int
}

// assemble groups the sheets into shells and solids (B1) and builds the
// output solid (B3 to B6). No geometry is welded here: sheets share edges
// by identity.
func assemble(ctx context.Context, points []point, edges []element, in []sheet, t tolerances) (*body, error) {
	b := &body{tau: t.eps, slack: t.slack()}
	if len(in) == 0 {
		return b, nil // B5
	}
	// Shells: sheets joined through shared edges, in order of first sheet.
	shellOf, first := filled(in, -1), filled(edges, -1)
	find := func(i int) int {
		for shellOf[i] >= 0 {
			i = shellOf[i]
		}
		return i
	}
	for i, sh := range in {
		for _, loop := range sh.loops {
			for _, h := range loop {
				if first[h.elem] < 0 {
					first[h.elem] = i
				} else if x, y := find(first[h.elem]), find(i); x != y {
					shellOf[max(x, y)] = min(x, y)
				}
			}
		}
	}
	var members [][]int
	index := filled(in, -1)
	for i := range in {
		root := find(i)
		if index[root] < 0 {
			index[root] = len(members)
			members = append(members, nil)
		}
		members[index[root]] = append(members[index[root]], i)
	}
	// Enclosed volume and smallest box corner of each shell (B1, V2), and
	// the largest radius of a node the result uses (T6).
	volumes, corners := table[float64](members), table[geom.Vec3](members)
	for k, list := range members {
		first := in[list[0]].loops[0][0]
		box := around(points[edges[first.elem].a].pos)
		for _, i := range list {
			v := in[i].s.chart().flux(legsOf(edges, in[i].loops))
			if in[i].reversed {
				v = -v
			}
			volumes[k] += v
			for _, loop := range in[i].loops {
				for _, h := range loop {
					for _, p := range [2]point{points[edges[h.elem].a], points[edges[h.elem].b]} {
						box = box.with(around(p.pos))
						b.tau = math.Max(b.tau, p.radius)
					}
				}
			}
		}
		corners[k] = box.lo
		if !(math.Abs(volumes[k]) > 0) || math.IsInf(volumes[k], 0) {
			return nil, fail(ErrValidation, "V2", "a shell encloses no volume")
		}
	}
	// Each void belongs to the smallest outer shell that contains it.
	parent := filled(members, -1)
	for k := range members {
		if volumes[k] > 0 {
			continue
		}
		probe := points[edges[in[members[k][0]].loops[0][0].elem].a].pos
		for j := range members {
			if volumes[j] < -volumes[k] || parent[k] >= 0 && volumes[j] >= volumes[parent[k]] {
				continue
			}
			shell, err := emit(ctx, points, edges, pick(in, members[j]), b.tau, b.slack)
			if err != nil {
				return nil, err
			}
			where, err := shell.solid.Classify(probe, b.tau)
			if err != nil || where == parametric.Boundary {
				return nil, fail(ErrValidation, "V2", "a void cannot be placed in an outer shell: %v", err)
			}
			if where == parametric.Inside {
				parent[k] = j
			}
		}
		if parent[k] < 0 {
			return nil, fail(ErrValidation, "V2", "a void lies in no outer shell")
		}
	}
	// Solids in the order of their smallest box corner; each outer shell
	// is followed by its voids.
	before := func(i, j int) bool {
		p, q := corners[i], corners[j]
		if p.X != q.X {
			return p.X < q.X
		}
		if p.Y != q.Y {
			return p.Y < q.Y
		}
		if p.Z != q.Z {
			return p.Z < q.Z
		}
		return i < j
	}
	var order []int
	for k := range members {
		order = append(order, k)
	}
	sort.SliceStable(order, func(i, j int) bool { return before(order[i], order[j]) })
	var sequence []sheet
	place := func(k int) {
		var list []int
		for _, i := range members[k] {
			list = append(list, len(sequence))
			sequence = append(sequence, in[i])
		}
		b.shells = append(b.shells, list)
		b.voids = append(b.voids, volumes[k] < 0)
		b.owners = append(b.owners, b.solids)
	}
	for _, k := range order {
		if volumes[k] < 0 {
			continue
		}
		place(k)
		for _, v := range order {
			if parent[v] == k {
				place(v)
			}
		}
		b.solids++
	}
	out, err := emit(ctx, points, edges, sequence, b.tau, b.slack)
	if err != nil {
		return nil, err
	}
	out.shells, out.voids, out.owners, out.solids = b.shells, b.voids, b.owners, b.solids
	return out, nil
}

// pick gives the listed entries of a slice.
func pick[T any](from []T, list []int) []T {
	var out []T
	for _, i := range list {
		out = append(out, from[i])
	}
	return out
}

// conditioningRounds bounds how often the arcs of the result are halved
// when its solid is not accepted (B2).
const conditioningRounds = 6

// emit builds a trimmed solid of the sheets (B2 to B6). Every curved edge
// is first cut into arcs of at most a quarter turn; while the solid is
// not accepted the arcs are halved, at most six times. Conditioning adds
// vertices and never moves geometry.
func emit(ctx context.Context, points []point, edges []element, in []sheet, tau, slack float64) (*body, error) {
	curved := false
	for _, sh := range in {
		for _, loop := range sh.loops {
			for _, h := range loop {
				curved = curved || edges[h.elem].c.period() > 0
			}
		}
	}
	var first error
	crowd := crowded(edges, in)
	for round := 0; ; round++ {
		b, err := build(ctx, points, edges, in, tau, slack, round, crowd)
		if err == nil || ctx.Err() != nil {
			return b, err
		}
		if first == nil {
			first = err
		}
		if !curved || round == conditioningRounds {
			return nil, first // what the unhalved arcs were refused for
		}
	}
}

// crowdedRounds bounds how often crowded halves the arcs of one edge.
const crowdedRounds = 8

// crowded gives, for each edge, how often its arcs are halved before the
// first attempt of emit, so that no vertex of a face lies between an arc
// of that face and the chord of the arc. trimmed.NewSolid reads the
// domain of a face as the polygon of its vertices in the chart (B2). A
// hole that comes nearer to the rim of its face than the rim's arcs
// stand off their chords has a vertex outside that polygon, and the
// face is refused though it is sound. Halving every arc of the solid
// until the polygon holds it, as emit then does, reaches the limits of
// a face before it reaches such a vertex; halving the arc it lies under
// does not. An edge is halved at most crowdedRounds times.
func crowded(edges []element, in []sheet) []int {
	extra := table[int](edges)
	for again := true; again; {
		again = false
		for _, sh := range in {
			paths := place(sh.s.chart(), legsOf(edges, sh.loops))
			// The vertices of the face in its chart, as its arcs stand.
			var spots []geom.Vec2
			for i, loop := range sh.loops {
				for k, h := range loop {
					for _, part := range conditioned(edges[h.elem], extra[h.elem]) {
						spots = append(spots, paths[i][k].at(part.t0))
					}
					spots = append(spots, paths[i][k].at(edges[h.elem].t1))
				}
			}
			for i, loop := range sh.loops {
				for k, h := range loop {
					if !paths[i][k].curved() || extra[h.elem] >= crowdedRounds {
						continue
					}
					for _, part := range conditioned(edges[h.elem], extra[h.elem]) {
						if under(paths[i][k], part.t0, part.t1, spots) {
							extra[h.elem]++
							again = true
							break
						}
					}
				}
			}
		}
	}
	return extra
}

// under reports whether one of the spots lies strictly between the part
// of a chart path from t0 to t1 and the chord of that part. The ends of
// the part are not counted.
func under(p path, t0, t1 float64, spots []geom.Vec2) bool {
	const samples = 16
	var ring [samples + 1]geom.Vec2
	for i := range ring {
		ring[i] = p.at(t0 + (t1-t0)*float64(i)/samples)
	}
	a, b := ring[0], ring[samples]
	span := math.Hypot(b.X-a.X, b.Y-a.Y)
	for _, s := range spots {
		if math.Hypot(s.X-a.X, s.Y-a.Y) <= 1e-9*span || math.Hypot(s.X-b.X, s.Y-b.Y) <= 1e-9*span {
			continue
		}
		inside := false
		for i, q := range ring {
			r := ring[(i+1)%len(ring)]
			if (q.Y > s.Y) != (r.Y > s.Y) && s.X < q.X+(r.X-q.X)*(s.Y-q.Y)/(r.Y-q.Y) {
				inside = !inside
			}
		}
		if inside {
			return true
		}
	}
	return false
}

// build is one attempt of emit, with every curved edge cut into arcs of
// at most a quarter turn, halved round times and as often again as
// crowded says of it. The points and edges the sheets use are numbered
// in index order, conditioning vertices last.
func build(ctx context.Context, points []point, edges []element, in []sheet, tau, slack float64, round int, crowd []int) (*body, error) {
	b := &body{tau: tau, slack: slack, sheets: in}
	usedEdge, usedPoint := table[bool](edges), table[bool](points)
	for _, sh := range in {
		for _, loop := range sh.loops {
			for _, h := range loop {
				usedEdge[h.elem] = true
				usedPoint[edges[h.elem].a], usedPoint[edges[h.elem].b] = true, true
			}
		}
	}
	vertexOf := filled(points, -1)
	next := 0
	for i, used := range usedPoint {
		if used {
			vertexOf[i] = next
			b.verts = append(b.verts, points[i].pos)
			next++
		}
	}
	// arcs lists, for each edge, the parts it is cut into: each a leg of
	// its carrier with the index of its edge in the solid.
	type arc struct {
		leg
		index    int
		from, to int // vertices of the solid at the ends of the leg
	}
	arcs := table[[]arc](edges)
	stored := 0 // the index the next part gets in the solid
	for i, used := range usedEdge {
		if !used {
			continue
		}
		e := edges[i]
		from := vertexOf[e.a]
		parts := conditioned(e, round+crowd[i])
		for k, part := range parts {
			to := vertexOf[e.b]
			if k+1 < len(parts) {
				to = next
				b.verts = append(b.verts, e.c.at(part.t1))
				next++
			}
			c, r, err := e.c.value(b.verts[from], b.verts[to], part.t0, part.t1)
			if err != nil {
				return nil, failWith(ErrValidation, "assembly", err)
			}
			arcs[i] = append(arcs[i], arc{part, stored, from, to})
			stored++
			b.edges = append(b.edges, parametric.Edge{Start: from, End: to, Curve: c, Range: r})
			from = to
		}
	}
	for _, sh := range in {
		value, err := sh.s.value()
		if err != nil {
			return nil, failWith(ErrValidation, "assembly", err)
		}
		face := trimmed.Face{Surface: value, Reversed: sh.reversed}
		// The loops as the arcs their hops are cut into, in loop order.
		var uses [][]trimmed.Use
		var legs [][]leg
		var starts [][]int // the vertex each use leaves
		for _, loop := range sh.loops {
			var us []trimmed.Use
			var ls []leg
			var vs []int
			for _, h := range loop {
				for k := range arcs[h.elem] {
					x := arcs[h.elem][k]
					if h.reversed {
						x = cyclic(arcs[h.elem], -1-k)
						x.leg, x.from = x.back(), x.to
					}
					us = append(us, trimmed.Use{Edge: x.index, Reversed: h.reversed})
					ls, vs = append(ls, x.leg), append(vs, x.from)
				}
			}
			uses, legs, starts = append(uses, us), append(legs, ls), append(starts, vs)
		}
		// The parameters of the surface along each use (B3): straight
		// for a straight path of the chart, an angular arc for any
		// other. An arc fixes the place of its ends; a straight use
		// takes the place of its neighbour.
		ch := sh.s.chart()
		for i, paths := range place(ch, legs) {
			us := uses[i]
			for j, p := range paths {
				// A straight use starts at the place of its vertex, which
				// is the same for every use that leaves the vertex in the
				// same turn of the chart.
				at := ch.locate(b.verts[starts[i][j]])
				if ch.period() > 0 {
					at.X += ch.period() * math.Round((p.at(p.t0).X-at.X)/ch.period())
				}
				us[j].UVStart = sh.s.stored(at)
				if !p.curved() {
					continue
				}
				if p.free != nil {
					// B1 of specs/general-boolean-quartic: the image by
					// evaluation. Its ends are set below, once those of
					// the use are.
					us[j].Curve = trim{p: p, s: sh.s}
					us[j].UVStart, us[j].UVEnd = sh.s.stored(p.at(p.t0)), sh.s.stored(p.at(p.t1))
					continue
				}
				// A path that was moved by whole periods of the chart is
				// written as the unmoved arc at angles a whole number of
				// turns on, so that it ends exactly where its neighbour
				// on the same carrier starts.
				raw, turns := ch.image(legs[i][j]), 0.
				if ch.period() > 0 && raw.lin.X != 0 {
					turns = 2 * math.Pi * math.Round((p.base.X-raw.base.X)/ch.period()) * raw.lin.X / math.Abs(raw.lin.X)
				}
				curve := trimmed.EllipseUV{Center: sh.s.stored(raw.base), Cos: sh.s.stored(p.cos), Sin: sh.s.stored(p.sin), Linear: sh.s.stored(p.lin), From: p.t0 + turns, To: p.t1 + turns}
				start, err := curve.EvalUV(0)
				if err != nil {
					return nil, failWith(ErrValidation, "assembly", err)
				}
				end, err := curve.EvalUV(1)
				if err != nil {
					return nil, failWith(ErrValidation, "assembly", err)
				}
				us[j].Curve, us[j].UVStart, us[j].UVEnd = curve, start, end
			}
			// The use after an arc starts where the arc ends. Where that
			// use is an arc of another curve, its own evaluation of the
			// shared point differs in the last bits; NewSolid takes an
			// arc whose ends are within the tolerance of the declared
			// ones and keeps it with its ends at them.
			for j := range us {
				if us[j].Curve != nil {
					us[(j+1)%len(us)].UVStart = us[j].UVEnd
				}
			}
			// A use along a section circle of a cylinder is level in the
			// chart, with both ends at the height of the circle, and a
			// use along a ruling is upright, with both ends at the place
			// of the ruling round the axis. The place of a vertex, and
			// the end of an arc, carry the rounding of their own
			// computation in that coordinate, and on a cylinder off the
			// coordinate axes it differs from one vertex of a rim, or one
			// end of a seam, to the next. Corners that are in line but
			// for that rounding leave NewSolid a triangle of no area
			// between them, in one release of Go and not in another.
			if ch.period() > 0 {
				for j, p := range paths {
					next := &us[(j+1)%len(us)]
					switch at := sh.s.stored(p.base); {
					case p.curved():
					case p.lin.Y == 0 && p.lin.X != 0:
						us[j].UVStart.Y, next.UVStart.Y = at.Y, at.Y
					case p.lin.X == 0 && p.lin.Y != 0:
						us[j].UVStart.X, next.UVStart.X = at.X, at.X
					}
				}
			}
			for j := range us {
				us[j].UVEnd = us[(j+1)%len(us)].UVStart
				if c, ok := us[j].Curve.(trim); ok {
					c.start, c.end, c.turns = us[j].UVStart, us[j].UVEnd, c.p.turns()
					us[j].Curve = c
				}
			}
			if i == 0 {
				face.Outer = us
			} else {
				face.Holes = append(face.Holes, us)
			}
		}
		b.faces = append(b.faces, face)
	}
	solid, err := trimmed.NewSolidContext(ctx, b.verts, b.edges, b.faces, tau+slack)
	if err != nil {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		return nil, failWith(ErrValidation, "V1", err) // B4
	}
	b.solid = solid
	return b, nil
}

// conditioned cuts an edge into the arcs its solid stores (B2): a
// straight edge is one part; a curved one is cut where its parameter
// passes a whole turn, and each part into arcs of at most a quarter turn,
// halved round times. Parameters are counted from the last whole turn,
// so that no arc spans one and a closed curve in a plane ends, to the
// last bit, where it starts. An end within 1e-9 of a whole turn is taken
// as that turn.
func conditioned(e element, round int) []leg {
	period := e.c.period()
	if period == 0 {
		return []leg{{e.c, e.t0, e.t1}}
	}
	const snap = 1e-9
	start := e.t0 - period*math.Floor(e.t0/period)
	if start < snap || period-start < snap {
		start = 0
	}
	left := e.t1 - e.t0
	var out []leg
	for from := start; left > snap; from = 0 {
		to := math.Min(from+left, period)
		if period-to < snap {
			to = period
		}
		left -= to - from
		if left <= snap && e.a == e.b && start > 0 {
			to = start // a closed edge ends exactly where it started
		}
		parts := int(math.Ceil((to-from)/(period/4)-snap)) << round
		for k := 0; k < parts; k++ {
			t0, t1 := from+(to-from)*float64(k)/float64(parts), from+(to-from)*float64(k+1)/float64(parts)
			if k+1 == parts {
				t1 = to
			}
			out = append(out, leg{e.c, t0, t1})
		}
	}
	return out
}

// trim is the p-curve of a use along a leg whose chart image is a free
// path: the image by evaluation, in the parameters of the stored
// surface. It gives the declared ends of its use at 0 and at 1, to the
// last bit, and lists where either coordinate turns back, which is what
// trimmed.Solid.Classify needs of it (trimmed.MonotoneUV).
type trim struct {
	p          path
	s          surface
	start, end geom.Vec2
	turns      []float64
}

var _ trimmed.MonotoneUV = trim{}

func (c trim) EvalUV(u float64) (geom.Vec2, error) {
	switch u {
	case 0:
		return c.start, nil
	case 1:
		return c.end, nil
	}
	return c.s.stored(c.p.at(c.p.t0 + (c.p.t1-c.p.t0)*u)), nil
}

func (c trim) TurnsUV() []float64 { return c.turns }

// DerivativeUV is the derivative of the curve by its own parameter, which
// trimmed.Solid.MassProperties integrates along. The map to the stored
// surface is linear, so it takes the velocity of the path as it takes
// its point.
func (c trim) DerivativeUV(u float64) (geom.Vec2, error) {
	v := c.s.stored(c.p.speed(c.p.t0 + (c.p.t1-c.p.t0)*u))
	return geom.Vec2{X: v.X * (c.p.t1 - c.p.t0), Y: v.Y * (c.p.t1 - c.p.t0)}, nil
}

// BoundsUV holds the curve: between two of its turns it is monotone in
// both coordinates, so it reaches no farther than its values at them and
// at its ends. brep/trimmed asks this of a p-curve it does not know.
func (c trim) BoundsUV() (geom.UVBox, error) {
	box := geom.UVBox{Min: c.start, Max: c.start}
	for _, u := range append(append([]float64{}, c.turns...), 1) {
		q, _ := c.EvalUV(u)
		box.Min = geom.Vec2{X: math.Min(box.Min.X, q.X), Y: math.Min(box.Min.Y, q.Y)}
		box.Max = geom.Vec2{X: math.Max(box.Max.X, q.X), Y: math.Max(box.Max.Y, q.Y)}
	}
	return box, nil
}

// turns lists the places, as fractions of the path strictly between its
// ends, at which either chart coordinate turns back: where a component
// of the velocity changes sign between two of 16 samples, placed by
// bisection. A change within a billionth of an end is the rounding of a
// velocity that is zero there.
func (p path) turns() []float64 {
	const parts, edge = 16, 1e-9
	var out []float64
	for axis := range 2 {
		speed := func(u float64) float64 {
			v := p.speed(p.t0 + (p.t1-p.t0)*u)
			return [2]float64{v.X, v.Y}[axis]
		}
		from, before := 0., speed(0)
		for k := 1; k <= parts; k++ {
			to := float64(k) / parts
			after := speed(to)
			if before != 0 && after != 0 && (before > 0) != (after > 0) {
				sign := -1
				if before > 0 {
					sign = 1
				}
				if u := zeroOf(speed, from, to, sign); u > edge && u < 1-edge {
					out = append(out, u)
				}
			}
			if after != 0 {
				from, before = to, after
			}
		}
	}
	sort.Float64s(out)
	return out
}
