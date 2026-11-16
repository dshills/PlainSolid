package iges

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

type igesBoreRimCurve struct {
	ends          [2]igesPlanarVec
	center        igesPlanarVec
	normal        geom.Vec3
	radiusSquared *big.Rat
}

type igesBoreRimIncidence struct {
	graph           borefillet.Graph
	indices, curves []int
}

func (p *igesBoreRimSource) proveIncidence(ctx context.Context, s trimmed.Solid, faceSources []int, faces []igesBoreRimFace,
	o geom.SurfacePatchOptions) ([]int, []int, []int, error) {
	fail := func(err error) ([]int, []int, []int, error) { return nil, nil, nil, err }
	g, err := borefillet.Extract(ctx, s, o)
	if err != nil {
		return fail(err)
	}
	edges, err := p.topology(p.edges, 504)
	if err != nil || len(edges) < 2 || edges[1] < 8 || edges[1] > len(g.Edges) || len(edges) != 2+5*edges[1] {
		return fail(errIGESBoreRim)
	}
	p.vertices = edges[3]
	modelVertices, sourceVertices, err := p.readVertexIndices(g)
	if err != nil {
		return fail(err)
	}
	n := edges[1]
	directions := make([][]bool, n+1)
	for _, f := range faces {
		for _, loop := range f.loops {
			for _, u := range loop {
				if u.edge > n {
					return fail(errIGESBoreRim)
				}
				directions[u.edge] = append(directions[u.edge], u.forward)
			}
		}
	}
	inc := igesBoreRimIncidence{graph: g, indices: make([]int, len(g.Edges)), curves: make([]int, len(g.Edges))}
	pieces := make([][]igesBoreRimUse, n+1)
	usedVertices := make([]bool, len(modelVertices))
	for i := 1; i <= n; i++ {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		t := edges[2+5*(i-1):]
		if d := directions[i]; len(d) != 2 || d[0] == d[1] {
			return fail(errIGESBoreRim)
		}
		a, b := t[2], t[4]
		if t[1] != p.vertices || t[3] != p.vertices || a < 1 || b < 1 || a > len(modelVertices) || b > len(modelVertices) {
			return fail(errIGESBoreRim)
		}
		usedVertices[a-1], usedVertices[b-1] = true, true
		x, y := modelVertices[a-1], modelVertices[b-1]
		curve, err := p.readCurve(t[0])
		if err != nil || !curve.ends[0].equal(igesPlanarVec(g.Vertices[x])) || !curve.ends[1].equal(igesPlanarVec(g.Vertices[y])) {
			return fail(errIGESBoreRim)
		}
		pieces[i], err = inc.assignCurve(curve, i, t[0], x, y)
		if err != nil {
			return fail(err)
		}
	}
	for _, i := range inc.indices {
		if i == 0 {
			return fail(errIGESBoreRim)
		}
	}
	for _, used := range usedVertices {
		if !used {
			return fail(errIGESBoreRim)
		}
	}
	byID := make(map[int]igesBoreRimFace, len(faces))
	for _, f := range faces {
		byID[f.id] = f
	}
	for i, id := range faceSources {
		if !matchIGESBoreRimFace(byID[id], g.Faces[i], pieces) {
			return fail(errIGESBoreRim)
		}
	}
	return inc.curves, inc.indices, sourceVertices, nil
}

func (p *igesBoreRimSource) readVertexIndices(g borefillet.Graph) ([]int, []int, error) {
	r, err := p.entity(p.vertices, 502, true)
	if err != nil || len(r.values) < 2 {
		return nil, nil, errIGESBoreRim
	}
	n, err := intValue(r.values[1])
	if err != nil || n < 5 || n > len(g.Vertices) || len(r.values) != 2+3*n {
		return nil, nil, errIGESBoreRim
	}
	matched, sources := make([]int, n), make([]int, len(g.Vertices))
	for i := range n {
		q, err := igesPlanarCoordinates(r.values[2+3*i:])
		if err != nil {
			return nil, nil, err
		}
		j := -1
		for k, v := range g.Vertices {
			if q.equal(igesPlanarVec(v)) {
				if j >= 0 || sources[k] != 0 {
					return nil, nil, errIGESBoreRim
				}
				j = k
			}
		}
		if j < 0 {
			return nil, nil, errIGESBoreRim
		}
		matched[i], sources[j] = j, i+1
	}
	return matched, sources, nil
}

func (p *igesBoreRimSource) readCurve(id int) (igesBoreRimCurve, error) {
	fail := func(err error) (igesBoreRimCurve, error) { return igesBoreRimCurve{}, err }
	r, err := p.entity(id, p.records[id].kind, true)
	if err != nil || len(r.values) != 7 && len(r.values) != 8 {
		return fail(errIGESBoreRim)
	}
	var c igesBoreRimCurve
	switch r.kind {
	case 110:
		if len(r.values) != 7 || r.transform != 0 {
			return fail(errIGESBoreRim)
		}
		for i := range c.ends {
			c.ends[i], err = igesPlanarCoordinates(r.values[1+3*i:])
			if err != nil {
				return fail(err)
			}
		}
		return c, nil
	case 100:
		if len(r.values) != 8 {
			return fail(errIGESBoreRim)
		}
	default:
		return fail(errIGESBoreRim)
	}
	x := make([]*big.Rat, 7)
	for i := range x {
		x[i], err = igesPlanarExact(r.values[i+1])
		if err != nil {
			return fail(err)
		}
	}
	center := igesPlanarVec{x[1], x[2], x[0]}
	ends := [2]igesPlanarVec{{x[3], x[4], x[0]}, {x[5], x[6], x[0]}}
	d := ends[0].sub(center)
	c.radiusSquared = d.dot(d)
	d = ends[1].sub(center)
	if c.radiusSquared.Sign() <= 0 || c.radiusSquared.Cmp(d.dot(d)) != 0 {
		return fail(errIGESBoreRim)
	}
	if r.transform == 0 {
		c.center, c.ends, c.normal = center, ends, geom.Vec3{Z: 1}
		return c, nil
	}
	frame, axis, err := p.arcFrame(r.transform)
	if err != nil {
		return fail(err)
	}
	c.center = frame(center)
	c.ends = [2]igesPlanarVec{frame(ends[0]), frame(ends[1])}
	c.normal = axis
	return c, nil
}

// All admitted type 124 arcs use an exact signed-coordinate frame with +X
// fixed. A local +Y axis chooses the horizontal rim or a vertical meridian.
func (p *igesBoreRimSource) arcFrame(id int) (func(igesPlanarVec) igesPlanarVec, geom.Vec3, error) {
	r, err := p.entity(id, 124, false)
	if err != nil || len(r.values) != 13 {
		return nil, geom.Vec3{}, errIGESBoreRim
	}
	var rows [3][4]*big.Rat
	for i := range rows {
		for j := range rows[i] {
			rows[i][j], err = igesPlanarExact(r.values[1+4*i+j])
			if err != nil {
				return nil, geom.Vec3{}, err
			}
		}
	}
	column := func(j int) (geom.Vec3, error) {
		return (igesPlanarVec{rows[0][j], rows[1][j], rows[2][j]}).represented()
	}
	x, err := column(0)
	if err != nil || x != (geom.Vec3{X: 1}) {
		return nil, geom.Vec3{}, errIGESBoreRim
	}
	y, err := column(1)
	if err != nil || y.X != 0 || !(y.Y == 0 && (y.Z == 1 || y.Z == -1) || y.Z == 0 && (y.Y == 1 || y.Y == -1)) {
		return nil, geom.Vec3{}, errIGESBoreRim
	}
	z, err := column(2)
	if err != nil || z != x.Cross(y) {
		return nil, geom.Vec3{}, errIGESBoreRim
	}
	mapPoint := func(v igesPlanarVec) (q igesPlanarVec) {
		for i, row := range rows {
			q[i] = new(big.Rat).Add(row[3], igesPlanarVec{row[0], row[1], row[2]}.dot(v))
		}
		return q
	}
	return mapPoint, z, nil
}

func (p *igesBoreRimIncidence) assignCurve(c igesBoreRimCurve, edge, curve, a, b int) ([]igesBoreRimUse, error) {
	if a == b {
		return p.assignFullCircle(c, edge, curve, a)
	}
	for i, e := range p.graph.Edges {
		forward := a == e.Start && b == e.End
		if !forward && (a != e.End || b != e.Start) {
			continue
		}
		if p.indices[i] != 0 || (e.Circle.Radius == nil) != (c.radiusSquared == nil) {
			return nil, errIGESBoreRim
		}
		if c.radiusSquared != nil {
			normal := geom.Vec3{Z: 1}
			if e.Circle.Meridian {
				normal = geom.Vec3{Y: -1}
			}
			if !c.sameCircle(e.Circle) || c.normal != normal && c.normal != normal.Scale(-1) ||
				(forward == e.Forward) != (c.normal == normal) {
				return nil, errIGESBoreRim
			}
		}
		p.indices[i], p.curves[i] = edge, curve
		return []igesBoreRimUse{{edge: i, forward: forward}}, nil
	}
	return nil, errIGESBoreRim
}

func (p *igesBoreRimIncidence) assignFullCircle(c igesBoreRimCurve, edge, curve, start int) ([]igesBoreRimUse, error) {
	if c.radiusSquared == nil || c.normal != (geom.Vec3{Z: 1}) && c.normal != (geom.Vec3{Z: -1}) {
		return nil, errIGESBoreRim
	}
	var out []igesBoreRimUse
	v := start
	for range 4 {
		next := -1
		for i, e := range p.graph.Edges {
			if !c.sameCircle(e.Circle) || e.Circle.Meridian {
				continue
			}
			a, b := e.Start, e.End
			forward := e.Forward == (c.normal.Z > 0)
			if !forward {
				a, b = b, a
			}
			if a != v {
				continue
			}
			if next >= 0 || p.indices[i] != 0 {
				return nil, errIGESBoreRim
			}
			next = b
			out = append(out, igesBoreRimUse{edge: i, forward: forward})
			p.indices[i], p.curves[i] = edge, curve
		}
		if next < 0 {
			return nil, errIGESBoreRim
		}
		v = next
	}
	if v != start {
		return nil, errIGESBoreRim
	}
	return out, nil
}

func (c igesBoreRimCurve) sameCircle(want borefillet.Circle) bool {
	return want.Radius != nil && c.radiusSquared.Cmp(new(big.Rat).Mul(want.Radius, want.Radius)) == 0 &&
		c.center.equal(igesPlanarVec(want.Center))
}

func matchIGESBoreRimFace(f igesBoreRimFace, native borefillet.Face, pieces [][]igesBoreRimUse) bool {
	wires := append([][]trimmed.Use{native.Outer}, native.Holes...)
	if len(wires) != len(f.loops) {
		return false
	}
	for i, loop := range f.loops {
		var path []igesBoreRimUse
		for _, u := range loop {
			parts := pieces[u.edge]
			if u.forward {
				path = append(path, parts...)
			} else {
				for j := len(parts) - 1; j >= 0; j-- {
					path = append(path, igesBoreRimUse{edge: parts[j].edge, forward: !parts[j].forward})
				}
			}
		}
		if !sameIGESBoreRimWire(path, wires[i], native.Reversed) {
			return false
		}
	}
	return true
}

func sameIGESBoreRimWire(path []igesBoreRimUse, want []trimmed.Use, reversed bool) bool {
	if len(path) != len(want) {
		return false
	}
	for start := range want {
		match := true
		for i, u := range path {
			j := (start + i) % len(want)
			if reversed {
				j = (start + len(want) - i) % len(want)
			}
			v := want[j]
			if u.edge != v.Edge || u.forward != (v.Reversed == reversed) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
