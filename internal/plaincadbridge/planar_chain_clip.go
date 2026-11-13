package plaincadbridge

import (
	"fmt"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// clipCardinalBoundary first removes coplanar internal edges using their exact
// incidence, then clips original planar polygons. It never rounds intersections.
func clipCardinalBoundary(s brep.Solid, origin, normal geom.Vec3) (brep.Solid, error) {
	fail := func() (brep.Solid, error) {
		return brep.Solid{}, unsupportedEdge("unsupported exact planar bevel boundary")
	}
	vs, es := s.Vertices(), s.Edges()
	groups := map[string]map[[2]int]bool{}
	for fi, f := range s.Faces() {
		if len(f.Holes) > 0 {
			return fail()
		}
		p, e := s.FacePlane(fi)
		if e != nil {
			return fail()
		}
		n := coordinates(p.Normal())
		axis := -1
		for k, v := range n {
			if v != 0 {
				if axis >= 0 || (v != 1 && v != -1) {
					return fail()
				}
				axis = k
			}
		}
		if axis < 0 {
			return fail()
		}
		v := coordinates(vs[es[f.Boundary[0].Edge].Start])
		for _, u := range f.Boundary {
			ed := es[u.Edge]
			if coordinates(vs[ed.Start])[axis] != v[axis] || coordinates(vs[ed.End])[axis] != v[axis] {
				return fail()
			}
		}
		key := fmt.Sprintf("%d:%g:%g", axis, n[axis], v[axis])
		g := groups[key]
		if g == nil {
			g = map[[2]int]bool{}
			groups[key] = g
		}
		for _, u := range f.Boundary {
			ed := es[u.Edge]
			a, b := ed.Start, ed.End
			if u.Reversed {
				a, b = b, a
			}
			rev := [2]int{b, a}
			if g[rev] {
				delete(g, rev)
			} else {
				key := [2]int{a, b}
				if g[key] {
					return fail()
				}
				g[key] = true
			}
		}
	}
	loops := [][]geom.Vec3{}
	for _, g := range groups {
		for len(g) > 0 {
			next := map[int]int{}
			for e := range g {
				if _, ok := next[e[0]]; ok {
					return fail()
				}
				next[e[0]] = e[1]
			}
			var first int
			for e := range g {
				first = e[0]
				break
			}
			cur := first
			loop := []geom.Vec3{}
			for {
				j, ok := next[cur]
				if !ok {
					return fail()
				}
				loop = append(loop, vs[cur])
				delete(g, [2]int{cur, j})
				cur = j
				if cur == first {
					break
				}
				if len(loop) > len(es) {
					return fail()
				}
			}
			loops = append(loops, removeCardinalCollinear(loop))
		}
	}
	n, o := coordinates(normal), coordinates(origin)
	distance := func(p geom.Vec3) *big.Rat {
		q := coordinates(p)
		d := new(big.Rat)
		for k := 0; k < 3; k++ {
			d.Add(d, new(big.Rat).Mul(new(big.Rat).SetFloat64(n[k]), new(big.Rat).Sub(new(big.Rat).SetFloat64(q[k]), new(big.Rat).SetFloat64(o[k]))))
		}
		return d
	}
	clipped := [][]geom.Vec3{}
	cutEdges := map[geom.Vec3]geom.Vec3{}
	for _, loop := range loops {
		out := []geom.Vec3{}
		for i, a := range loop {
			b := loop[(i+1)%len(loop)]
			da, db := distance(a), distance(b)
			if da.Sign() <= 0 {
				out = append(out, a)
			}
			if da.Sign()*db.Sign() < 0 {
				t := new(big.Rat).Quo(da, new(big.Rat).Sub(da, db))
				aa, bb := coordinates(a), coordinates(b)
				var cc [3]float64
				for k := 0; k < 3; k++ {
					r := new(big.Rat).Add(new(big.Rat).SetFloat64(aa[k]), new(big.Rat).Mul(t, new(big.Rat).Sub(new(big.Rat).SetFloat64(bb[k]), new(big.Rat).SetFloat64(aa[k]))))
					v, _ := r.Float64()
					check := new(big.Rat).SetFloat64(v)
					if check == nil || check.Cmp(r) != 0 {
						return fail()
					}
					cc[k] = v
				}
				out = append(out, geom.Vec3{X: cc[0], Y: cc[1], Z: cc[2]})
			}
		}
		if len(out) < 3 {
			continue
		}
		for i, a := range out {
			b := out[(i+1)%len(out)]
			if distance(a).Sign() == 0 && distance(b).Sign() == 0 {
				if _, ok := cutEdges[b]; ok {
					return fail()
				}
				cutEdges[b] = a
			}
		}
		clipped = append(clipped, out)
	}
	if len(cutEdges) < 3 {
		return fail()
	}
	var first geom.Vec3
	for a := range cutEdges {
		first = a
		break
	}
	cap := []geom.Vec3{}
	cur := first
	for {
		next, ok := cutEdges[cur]
		if !ok {
			return fail()
		}
		cap = append(cap, cur)
		delete(cutEdges, cur)
		cur = next
		if cur == first {
			break
		}
	}
	if len(cutEdges) != 0 {
		return fail()
	}
	clipped = append(clipped, cap)
	vertices := []geom.Vec3{}
	ids := map[geom.Vec3]int{}
	edges := []brep.Edge{}
	edgeIDs := map[[2]int]int{}
	faces := []brep.Face{}
	intern := func(p geom.Vec3) int {
		if i, ok := ids[p]; ok {
			return i
		}
		i := len(vertices)
		ids[p] = i
		vertices = append(vertices, p)
		return i
	}
	for _, loop := range clipped {
		face := brep.Face{}
		for i, a := range loop {
			u, v := intern(a), intern(loop[(i+1)%len(loop)])
			rev := u > v
			if rev {
				u, v = v, u
			}
			key := [2]int{u, v}
			id, ok := edgeIDs[key]
			if !ok {
				id = len(edges)
				edgeIDs[key] = id
				edges = append(edges, brep.Edge{Start: u, End: v})
			}
			face.Boundary = append(face.Boundary, brep.EdgeUse{Edge: id, Reversed: rev})
		}
		faces = append(faces, face)
	}
	return brep.NewSolid(vertices, edges, faces)
}
func removeCardinalCollinear(p []geom.Vec3) []geom.Vec3 {
	for changed := true; changed && len(p) > 3; {
		changed = false
		for i := range p {
			a, b, c := coordinates(p[(i+len(p)-1)%len(p)]), coordinates(p[i]), coordinates(p[(i+1)%len(p)])
			fixed := 0
			for k := 0; k < 3; k++ {
				if a[k] == b[k] && b[k] == c[k] {
					fixed++
				}
			}
			if fixed >= 2 {
				p = append(p[:i], p[i+1:]...)
				changed = true
				break
			}
		}
	}
	return p
}
