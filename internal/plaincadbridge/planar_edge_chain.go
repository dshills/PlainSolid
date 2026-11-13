package plaincadbridge

import (
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

// planarEdgeChain resolves subdivisions of one complete cardinal sharp edge.
// Coplanar triangulation diagonals are ignored; every boundary interval must
// carry the same two orthogonal support normals and cover the authored segment.
func planarEdgeChain(s brep.Solid, start, end geom.Vec3) (geom.Vec3, geom.Vec3, bool) {
	fail := func() (geom.Vec3, geom.Vec3, bool) { return geom.Vec3{}, geom.Vec3{}, false }
	a, b := [3]float64{start.X, start.Y, start.Z}, [3]float64{end.X, end.Y, end.Z}
	axis := -1
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if axis >= 0 {
				return fail()
			}
			axis = i
		}
	}
	if axis < 0 {
		return fail()
	}
	lo, hi := a[axis], b[axis]
	if lo > hi {
		lo, hi = hi, lo
	}
	owners := make([][]geom.Vec3, len(s.Edges()))
	for i, f := range s.Faces() {
		p, e := s.FacePlane(i)
		if e != nil {
			return fail()
		}
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, u := range loop {
				owners[u.Edge] = append(owners[u.Edge], p.Normal())
			}
		}
	}
	type interval struct{ lo, hi float64 }
	var spans []interval
	var normals [2]geom.Vec3
	set := false
	v := s.Vertices()
	for id, ed := range s.Edges() {
		p, q := v[ed.Start], v[ed.End]
		pp, qq := [3]float64{p.X, p.Y, p.Z}, [3]float64{q.X, q.Y, q.Z}
		on := true
		for j := 0; j < 3; j++ {
			if j != axis && (pp[j] != a[j] || qq[j] != a[j]) {
				on = false
			}
		}
		if !on {
			continue
		}
		l, h := pp[axis], qq[axis]
		if l > h {
			l, h = h, l
		}
		if h < lo || l > hi || l == h {
			continue
		}
		ns := owners[id]
		if len(ns) != 2 {
			return fail()
		}
		if ns[0] == ns[1] {
			continue
		}
		if ns[0].Dot(ns[1]) != 0 {
			return fail()
		}
		if l < lo || h > hi {
			return fail()
		}
		if !set {
			normals = [2]geom.Vec3{ns[0], ns[1]}
			set = true
		} else if !(ns[0] == normals[0] && ns[1] == normals[1] || ns[0] == normals[1] && ns[1] == normals[0]) {
			return fail()
		}
		spans = append(spans, interval{l, h})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].lo < spans[j].lo })
	cursor := lo
	for _, s := range spans {
		if s.lo != cursor {
			return fail()
		}
		cursor = s.hi
	}
	if !set || cursor != hi {
		return fail()
	}
	return start, end, true
}
