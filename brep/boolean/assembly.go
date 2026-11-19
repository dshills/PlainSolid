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
	b := &body{tau: t.eps}
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
			shell, err := emit(ctx, points, edges, pick(in, members[j]), b.tau)
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
	out, err := emit(ctx, points, edges, sequence, b.tau)
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
func emit(ctx context.Context, points []point, edges []element, in []sheet, tau float64) (*body, error) {
	curved := false
	for _, sh := range in {
		for _, loop := range sh.loops {
			for _, h := range loop {
				curved = curved || edges[h.elem].c.period() > 0
			}
		}
	}
	var first error
	for round := 0; ; round++ {
		b, err := build(ctx, points, edges, in, tau, round)
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

// build is one attempt of emit, with every curved edge cut into arcs of
// at most a quarter turn halved round times. The points and edges the
// sheets use are numbered in index order, conditioning vertices last.
func build(ctx context.Context, points []point, edges []element, in []sheet, tau float64, round int) (*body, error) {
	b := &body{tau: tau, sheets: in}
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
		parts := conditioned(e, round)
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
			for j := range us {
				following := &us[(j+1)%len(us)]
				switch {
				case us[j].Curve != nil && following.Curve != nil:
					if us[j].UVEnd != following.UVStart {
						return nil, fail(ErrValidation, "assembly", "two arcs of different curves meet at a vertex of a face, and their parameter curves do not agree there to the last bit, which trimmed.NewSolid requires")
					}
				case us[j].Curve != nil:
					following.UVStart = us[j].UVEnd
				}
			}
			for j := range us {
				us[j].UVEnd = us[(j+1)%len(us)].UVStart
			}
			if i == 0 {
				face.Outer = us
			} else {
				face.Holes = append(face.Holes, us)
			}
		}
		b.faces = append(b.faces, face)
	}
	solid, err := trimmed.NewSolidContext(ctx, b.verts, b.edges, b.faces, tau)
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
