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

// emit builds a trimmed solid of the sheets, with the points and edges
// they use numbered in index order.
func emit(ctx context.Context, points []point, edges []element, in []sheet, tau float64) (*body, error) {
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
	vertexOf, edgeOf := filled(points, -1), filled(edges, -1)
	next := 0
	for i, used := range usedPoint {
		if used {
			vertexOf[i] = next
			b.verts = append(b.verts, points[i].pos)
			next++
		}
	}
	next = 0
	for i, used := range usedEdge {
		if !used {
			continue
		}
		e := edges[i]
		c, r, err := e.c.value(points[e.a].pos, points[e.b].pos)
		if err != nil {
			return nil, failWith(ErrValidation, "assembly", err)
		}
		edgeOf[i] = next
		next++
		b.edges = append(b.edges, parametric.Edge{Start: vertexOf[e.a], End: vertexOf[e.b], Curve: c, Range: r})
	}
	for _, sh := range in {
		value, err := sh.s.value()
		if err != nil {
			return nil, failWith(ErrValidation, "assembly", err)
		}
		face := trimmed.Face{Surface: value, Reversed: sh.reversed}
		ch := sh.s.chart()
		for i, loop := range sh.loops {
			uses := make([]trimmed.Use, len(loop))
			for j, h := range loop {
				from, to := edges[h.elem].a, edges[h.elem].b
				if h.reversed {
					from, to = to, from
				}
				uses[j] = trimmed.Use{Edge: edgeOf[h.elem], Reversed: h.reversed, UVStart: ch.locate(points[from].pos), UVEnd: ch.locate(points[to].pos)}
			}
			if i == 0 {
				face.Outer = uses
			} else {
				face.Holes = append(face.Holes, uses)
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
