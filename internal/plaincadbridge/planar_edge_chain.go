package plaincadbridge

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// supportingEdgeChain proves a complete cardinal sharp edge, permitting only
// subdivisions with the same two globally supporting outward face planes.
func supportingEdgeChain(s brep.Solid, a, b geom.Vec3) ([2]geom.Vec3, bool) {
	var normals [2]geom.Vec3
	verts, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if !a.IsFinite() || !b.IsFinite() || len(edges) > 50000 || len(faces) > 10000 {
		return normals, false
	}
	av, bv := coordinates(a), coordinates(b)
	axis := -1
	for k := 0; k < 3; k++ {
		if av[k] != bv[k] {
			if axis >= 0 {
				return normals, false
			}
			axis = k
		}
	}
	if axis < 0 {
		return normals, false
	}
	lo, hi := av[axis], bv[axis]
	if lo > hi {
		lo, hi = hi, lo
	}
	incidence := make([][]int, len(edges))
	for fi, f := range faces {
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, use := range loop {
				incidence[use.Edge] = append(incidence[use.Edge], fi)
			}
		}
	}
	spans := [][2]float64{}
	for id, e := range edges {
		p, q := coordinates(verts[e.Start]), coordinates(verts[e.End])
		same := true
		for k := 0; k < 3; k++ {
			if k != axis && (p[k] != av[k] || q[k] != av[k]) {
				same = false
			}
		}
		if !same || p[axis] == q[axis] {
			continue
		}
		x, y := p[axis], q[axis]
		if x > y {
			x, y = y, x
		}
		if y < lo || x > hi {
			continue
		}
		ns := []geom.Vec3{}
		for _, fi := range incidence[id] {
			plane, err := s.FacePlane(fi)
			if err != nil {
				return normals, false
			}
			n := plane.Normal()
			count := 0
			for _, v := range coordinates(n) {
				if v != 0 {
					if v != 1 && v != -1 {
						return normals, false
					}
					count++
				}
			}
			if count != 1 {
				return normals, false
			}
			ns = append(ns, n)
		}
		if len(ns) != 2 {
			return normals, false
		}
		if ns[0] == ns[1] {
			continue
		}
		if ns[0].Dot(ns[1]) != 0 {
			return normals, false
		}
		if x < lo || y > hi {
			return normals, false
		}
		if len(spans) == 0 {
			normals = [2]geom.Vec3{ns[0], ns[1]}
		} else if !((normals[0] == ns[0] && normals[1] == ns[1]) || (normals[0] == ns[1] && normals[1] == ns[0])) {
			return normals, false
		}
		spans = append(spans, [2]float64{x, y})
	}
	if len(spans) == 0 {
		return normals, false
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
	end := lo
	for _, v := range spans {
		if v[0] != end {
			return normals, false
		}
		end = v[1]
	}
	if end != hi {
		return normals, false
	}
	for _, p := range verts {
		pv := coordinates(p)
		for _, n := range normals {
			nv := coordinates(n)
			for k := 0; k < 3; k++ {
				if nv[k] > 0 && pv[k] > av[k] || nv[k] < 0 && pv[k] < av[k] {
					return normals, false
				}
			}
		}
	}
	return normals, true
}

func exactShift(p, n geom.Vec3, d float64) (geom.Vec3, bool) {
	v, w := coordinates(p), coordinates(n)
	for k := 0; k < 3; k++ {
		r := new(big.Rat).Sub(new(big.Rat).SetFloat64(v[k]), new(big.Rat).Mul(new(big.Rat).SetFloat64(w[k]), new(big.Rat).SetFloat64(d)))
		f, _ := r.Float64()
		q := new(big.Rat).SetFloat64(f)
		if q == nil || q.Cmp(r) != 0 {
			return geom.Vec3{}, false
		}
		v[k] = f
	}
	return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, true
}

// chamferSupportingChain requires a fully occupied rectangular collar. Exact
// difference emptiness proves occupancy, preventing a bevel across voids.
func chamferSupportingChain(s brep.Solid, a, b, normal geom.Vec3, d float64) (brep.Solid, error) {
	ns, ok := supportingEdgeChain(s, a, b)
	if !ok || (normal != ns[0] && normal != ns[1] && normal != ns[0].Scale(-1) && normal != ns[1].Scale(-1)) {
		return brep.Solid{}, unsupportedEdge("unproved supporting edge chain")
	}
	av, bv := coordinates(a), coordinates(b)
	axis := 0
	for av[axis] == bv[axis] {
		axis++
	}
	lo, hi := av[axis], bv[axis]
	if lo > hi {
		lo, hi = hi, lo
	}
	for _, v := range s.Vertices() {
		x := coordinates(v)[axis]
		if x < lo || x > hi {
			return brep.Solid{}, unsupportedEdge("stock extends beyond authored bevel span")
		}
	}
	p, ok := exactShift(a, ns[0], d)
	if !ok {
		return brep.Solid{}, geom.ErrDomain
	}
	q, ok := exactShift(a, ns[1], d)
	if !ok {
		return brep.Solid{}, geom.ErrDomain
	}
	r, ok := exactShift(p, ns[1], d)
	if !ok {
		return brep.Solid{}, geom.ErrDomain
	}
	offset := b.Sub(a)
	oc := coordinates(offset)
	ac, bc := coordinates(a), coordinates(b)
	for k := 0; k < 3; k++ {
		r := new(big.Rat).Sub(new(big.Rat).SetFloat64(bc[k]), new(big.Rat).SetFloat64(ac[k]))
		v := new(big.Rat).SetFloat64(oc[k])
		if v == nil || v.Cmp(r) != 0 {
			return brep.Solid{}, geom.ErrDomain
		}
	}
	collar, e := brep.Prism([]geom.Vec3{a, p, r, q}, offset)
	if e != nil {
		return brep.Solid{}, e
	}
	opts := poly.Options{RequireExactConversion: true}
	outside, handled, e := poly.CombineAxisAligned(context.Background(), collar, s, poly.Difference, opts)
	if e != nil {
		return brep.Solid{}, e
	}
	if !handled || len(outside.Faces()) != 0 {
		return brep.Solid{}, unsupportedEdge("chamfer collar is not fully occupied")
	}
	out, e := clipCardinalBoundary(s, p, ns[0].Add(ns[1]))
	if e != nil {
		return brep.Solid{}, e
	}

	return out, nil
}
