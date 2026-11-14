package iges

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// TetrahedronSolid preserves the original explicit IGES topology and its
// underlying affine spline charts and model/parameter curves. Solid is the
// independently proved equivalent convex planar native boundary.
type TetrahedronSolid struct {
	LengthUnit                                                         float64
	Solid                                                              brep.Solid
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	SourceFaceIDs                                                      [4]int
	Geometry                                                           map[int]Entity
	Metadata                                                           map[int]CylinderDatumMetadata
}

func tetraFailure() error         { return fmt.Errorf("iges: unsupported exact tetrahedron topology") }
func tetraRat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
func tetraDet(a, b, c, d geom.Vec3) int {
	v := func(p geom.Vec3) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Sub(tetraRat(p.X), tetraRat(a.X)), new(big.Rat).Sub(tetraRat(p.Y), tetraRat(a.Y)), new(big.Rat).Sub(tetraRat(p.Z), tetraRat(a.Z))}
	}
	x, y, z := v(b), v(c), v(d)
	sum := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		cross := new(big.Rat).Sub(new(big.Rat).Mul(y[j], z[k]), new(big.Rat).Mul(y[k], z[j]))
		sum.Add(sum, new(big.Rat).Mul(x[i], cross))
	}
	return sum.Sign()
}
func tetraLinear(en Entity) ([2]geom.Vec3, error) {
	var out [2]geom.Vec3
	if en.Spline == nil || en.Transform != nil || en.sourceWeightsRounded {
		return out, tetraFailure()
	}
	s := en.Spline
	g := s.Curve
	if g == nil || g.Degree() != 1 || len(g.Poles()) != 2 || s.Start != 0 || s.End != 1 || s.Closed || s.Periodic || !s.Polynomial || s.Planar || s.Normal != (geom.Vec3{}) {
		return out, tetraFailure()
	}
	k, w := g.Knots(), g.Weights()
	if len(k) != 4 || k[0] != 0 || k[1] != 0 || k[2] != 1 || k[3] != 1 || w[0] != 1 || w[1] != 1 {
		return out, tetraFailure()
	}
	copy(out[:], g.Poles())
	return out, nil
}

// ReadTetrahedronSolid accepts exactly one type186 solid, one outward type514
// shell and four triangular type510 faces with affine degree-one type128 charts.
// Shared model edges and independent owner pcurves are polynomial degree-one
// type126 curves. Exact coefficient, endpoint, orientation and convexity proofs
// precede native construction. No tolerance, sewing repair or general solid
// inference is used.
func ReadTetrahedronSolid(input io.Reader, l Limits) (TetrahedronSolid, error) {
	fail := func() (TetrahedronSolid, error) { return TetrahedronSolid{}, tetraFailure() }
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxEntities = min(l.MaxEntities, 34)
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxControlPoints = min(l.MaxControlPoints, 52)
	d, e := readTopologySource(input, l, true)
	if e != nil {
		return TetrahedronSolid{}, e
	}
	if len(d.entities) != 34 {
		return fail()
	}
	// Appearance/level references are outside this material-only dialect.
	for _, line := range bytes.Split(d.bytes, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			seq, err := field(string(line[73:]))
			if err != nil {
				return fail()
			}
			indexes := []int{1, 2}
			if seq%2 == 1 {
				kind, err := field(string(line[:8]))
				if err != nil {
					return fail()
				}
				status, err := field(string(line[64:72]))
				if err != nil {
					return fail()
				}
				want := 10000
				if kind == 186 {
					want = 0
				}
				if kind == 504 {
					want = 10001
				}
				if status != want {
					return fail()
				}
				indexes = []int{3, 4}
			}
			for _, j := range indexes {
				v, err := field(string(line[j*8 : j*8+8]))
				if err != nil || v != 0 {
					return fail()
				}
			}
		}
	}
	m := TetrahedronSolid{LengthUnit: d.unit, Geometry: map[int]Entity{}, Metadata: map[int]CylinderDatumMetadata{}}
	records := map[int]topologyRecord{}
	kinds := map[int][]int{}
	used := map[int]bool{}
	controls := 0
	for _, r := range d.entities {
		records[r.id] = r
		kinds[r.kind] = append(kinds[r.kind], r.id)
		m.Metadata[r.id] = CylinderDatumMetadata{r.label, r.subscript}
		if e = compositeSourceNumbers(110, r.values); e != nil {
			return fail()
		}
		if r.kind == 126 || r.kind == 128 {
			en, e := decode(r.kind, r.values, l.MaxControlPoints-controls)
			if e != nil || en.sourceWeightsRounded {
				return fail()
			}
			if e = en.validate(); e != nil {
				return fail()
			}
			m.Geometry[r.id] = en
			if en.Spline != nil {
				controls += len(en.Spline.Curve.Poles())
			} else {
				controls += len(en.Surface.Geometry.Data().Poles)
			}
		}
	}
	for kind, n := range map[int]int{186: 1, 514: 1, 510: 4, 508: 4, 504: 1, 502: 1, 128: 4, 126: 18} {
		if len(kinds[kind]) != n {
			return fail()
		}
	}
	get := func(id, kind int) (topologyRecord, bool) {
		r, ok := records[id]
		if !ok || r.kind != kind || used[id] {
			return topologyRecord{}, false
		}
		used[id] = true
		return r, true
	}
	m.SourceSolidID = kinds[186][0]
	r, _ := get(m.SourceSolidID, 186)
	v, e := topologyInts(r, 4)
	if e != nil || v[2] != 1 || v[3] != 0 {
		return fail()
	}
	m.SourceShellID = v[1]
	r, ok := get(v[1], 514)
	if !ok {
		return fail()
	}
	v, e = topologyInts(r, 10)
	if e != nil || v[1] != 4 {
		return fail()
	}
	for i := 0; i < 4; i++ {
		if v[3+2*i] != 1 {
			return fail()
		}
		m.SourceFaceIDs[i] = v[2+2*i]
	}
	m.SourceVertexListID = kinds[502][0]
	r, _ = get(m.SourceVertexListID, 502)
	if len(r.values) < 14 {
		return fail()
	}
	kind, e := intValue(r.values[0])
	if e != nil || kind != 502 {
		return fail()
	}
	nv, e := intValue(r.values[1])
	if e != nil || nv != 4 || noPointers(r.values[14:]) != nil {
		return fail()
	}
	vertices := make([]geom.Vec3, 4)
	for i := range vertices {
		a := [3]float64{}
		for j := range a {
			a[j], e = realValue(r.values[2+3*i+j])
			if e != nil {
				return fail()
			}
		}
		vertices[i] = geom.Vec3{X: a[0], Y: a[1], Z: a[2]}
	}
	if tetraDet(vertices[0], vertices[1], vertices[2], vertices[3]) == 0 {
		return fail()
	}
	m.SourceEdgeListID = kinds[504][0]
	r, _ = get(m.SourceEdgeListID, 504)
	v, e = topologyInts(r, 32)
	if e != nil || v[1] != 6 {
		return fail()
	}
	edges := make([]brep.Edge, 6)
	pairs := map[[2]int]bool{}
	for i := range edges {
		a := v[2+5*i : 7+5*i]
		if a[1] != m.SourceVertexListID || a[3] != m.SourceVertexListID || a[2] < 1 || a[2] > 4 || a[4] < 1 || a[4] > 4 || a[2] == a[4] {
			return fail()
		}
		edge := brep.Edge{Start: a[2] - 1, End: a[4] - 1}
		key := [2]int{min(edge.Start, edge.End), max(edge.Start, edge.End)}
		if pairs[key] {
			return fail()
		}
		pairs[key] = true
		edges[i] = edge
		if _, ok = get(a[0], 126); !ok {
			return fail()
		}
		p, e := tetraLinear(m.Geometry[a[0]])
		if e != nil || p[0] != vertices[edge.Start] || p[1] != vertices[edge.End] {
			return fail()
		}
	}
	faces := make([]brep.Face, 4)
	opposites := map[int]bool{}
	for fi, fid := range m.SourceFaceIDs {
		r, ok = get(fid, 510)
		if !ok {
			return fail()
		}
		v, e = topologyInts(r, 5)
		if e != nil || v[2] != 1 || v[3] != 1 {
			return fail()
		}
		sid, lid := v[1], v[4]
		if _, ok = get(sid, 128); !ok {
			return fail()
		}
		s := m.Geometry[sid].Surface
		if s == nil || s.UStart != 0 || s.UEnd != 1 || s.VStart != 0 || s.VEnd != 1 || s.ClosedU || s.ClosedV || s.PeriodicU || s.PeriodicV || !s.Polynomial {
			return fail()
		}
		sd := s.Geometry.Data()
		if sd.DegreeU != 1 || sd.DegreeV != 1 || sd.CountU != 2 || sd.CountV != 2 {
			return fail()
		}
		for _, k := range [][]float64{sd.KnotsU, sd.KnotsV} {
			if len(k) != 4 || k[0] != 0 || k[1] != 0 || k[2] != 1 || k[3] != 1 {
				return fail()
			}
		}
		for _, w := range sd.Weights {
			if w != 1 {
				return fail()
			}
		}
		// In-memory indexing is u*CountV+v: p0,pV,pU,pUV.
		uv := map[int]geom.Vec3{}
		for j, p := range []geom.Vec3{sd.Poles[0], sd.Poles[2], sd.Poles[1]} {
			found := -1
			for i, q := range vertices {
				if p == q {
					found = i
				}
			}
			if found < 0 {
				return fail()
			}
			q := geom.Vec3{}
			if j == 1 {
				q.X = 1
			}
			if j == 2 {
				q.Y = 1
			}
			uv[found] = q
		}
		if len(uv) != 3 {
			return fail()
		}
		for axis := 0; axis < 3; axis++ {
			coord := func(p geom.Vec3) float64 { return [3]float64{p.X, p.Y, p.Z}[axis] }
			lhs := new(big.Rat).Add(tetraRat(coord(sd.Poles[0])), tetraRat(coord(sd.Poles[3])))
			rhs := new(big.Rat).Add(tetraRat(coord(sd.Poles[1])), tetraRat(coord(sd.Poles[2])))
			if lhs.Cmp(rhs) != 0 {
				return fail()
			}
		}
		r, ok = get(lid, 508)
		if !ok {
			return fail()
		}
		lv, e := topologyInts(r, 23)
		if e != nil || lv[1] != 3 {
			return fail()
		}
		var tri [3]int
		for j := 0; j < 3; j++ {
			a := lv[2+7*j : 9+7*j]
			if a[0] != 0 || a[1] != m.SourceEdgeListID || a[2] < 1 || a[2] > 6 || (a[3] != 0 && a[3] != 1) || a[4] != 1 || a[5] != 0 {
				return fail()
			}
			edge := edges[a[2]-1]
			p0, o0 := uv[edge.Start]
			p1, o1 := uv[edge.End]
			if !o0 || !o1 {
				return fail()
			}
			if _, ok = get(a[6], 126); !ok {
				return fail()
			}
			pc, e := tetraLinear(m.Geometry[a[6]])
			if e != nil || pc[0] != p0 || pc[1] != p1 {
				return fail()
			}
			rev := a[3] == 0
			faces[fi].Boundary = append(faces[fi].Boundary, brep.EdgeUse{Edge: a[2] - 1, Reversed: rev})
			tri[j] = edge.Start
			if rev {
				tri[j] = edge.End
			}
		}
		for j, u := range faces[fi].Boundary {
			edge := edges[u.Edge]
			end := edge.End
			if u.Reversed {
				end = edge.Start
			}
			if end != tri[(j+1)%3] {
				return fail()
			}
		}
		a, b, c := uv[tri[0]], uv[tri[1]], uv[tri[2]]
		if (b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X) != 1 {
			return fail()
		}
		opposite := -1
		for i := 0; i < 4; i++ {
			if i != tri[0] && i != tri[1] && i != tri[2] {
				opposite = i
			}
		}
		if opposite < 0 || opposites[opposite] || tetraDet(vertices[tri[0]], vertices[tri[1]], vertices[tri[2]], vertices[opposite]) >= 0 {
			return fail()
		}
		opposites[opposite] = true
	}
	if len(used) != len(records) {
		return fail()
	}
	m.Solid, e = brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return fail()
	}
	return m, nil
}

// WriteTetrahedronSolid authors a canonical exact type186 graph for four affine-
// independent represented vertices. It stages the full typed readback before I/O.
func WriteTetrahedronSolid(w io.Writer, vertices [4]geom.Vec3, unit float64) error {
	if w == nil {
		return tetraFailure()
	}
	for _, v := range vertices {
		if !v.IsFinite() {
			return tetraFailure()
		}
	}
	sign := tetraDet(vertices[0], vertices[1], vertices[2], vertices[3])
	if sign == 0 {
		return tetraFailure()
	}
	a := topologyAuthor{}
	vp := []string{"502", "4"}
	for _, p := range vertices {
		vp = append(vp, number(p.X), number(p.Y), number(p.Z))
	}
	vid := a.add(502, 1, joinTopologyValues(vp))
	curve := func(p, q geom.Vec3) int {
		g, e := geom.NewBSpline(1, []geom.Vec3{p, q}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			return 0
		}
		return a.entity(Entity{Spline: &Spline{Curve: g, Start: 0, End: 1, Polynomial: true}})
	}
	edges := [][2]int{{0, 1}, {0, 2}, {0, 3}, {1, 2}, {1, 3}, {2, 3}}
	ev := []int{504, 6}
	for _, e := range edges {
		ev = append(ev, curve(vertices[e[0]], vertices[e[1]]), vid, e[0]+1, vid, e[1]+1)
	}
	eid := a.add(504, 1, topologyNumbers(ev...))
	tris := [][3]int{{0, 2, 1}, {0, 1, 3}, {0, 3, 2}, {1, 2, 3}}
	fv := []int{514, 4}
	for _, tri := range tris {
		if sign < 0 {
			tri[1], tri[2] = tri[2], tri[1]
		}
		p, u, v := vertices[tri[0]], vertices[tri[1]], vertices[tri[2]]
		var xyz [3]float64
		for j := range xyz {
			coords := func(q geom.Vec3) float64 { return [3]float64{q.X, q.Y, q.Z}[j] }
			r := new(big.Rat).Sub(new(big.Rat).Add(tetraRat(coords(u)), tetraRat(coords(v))), tetraRat(coords(p)))
			value, exact := r.Float64()
			if !exact {
				return geom.ErrUnrepresentable
			}
			xyz[j] = value
		}
		fourth := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		g, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{p, v, u, fourth}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			return e
		}
		sid := a.entity(Entity{Surface: &Surface{Geometry: g, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1, Polynomial: true}})
		uv := map[int]geom.Vec3{tri[0]: {}, tri[1]: {X: 1}, tri[2]: {Y: 1}}
		lv := []int{508, 3}
		for j, start := range tri {
			end := tri[(j+1)%3]
			for i, edge := range edges {
				if min(start, end) == edge[0] && max(start, end) == edge[1] {
					sense := 0
					if start == edge[0] {
						sense = 1
					}
					lv = append(lv, 0, eid, i+1, sense, 1, 0, curve(uv[edge[0]], uv[edge[1]]))
				}
			}
		}
		loop := a.add(508, 1, topologyNumbers(lv...))
		face := a.add(510, 1, topologyNumbers(510, sid, 1, 1, loop))
		fv = append(fv, face, 1)
	}
	shell := a.add(514, 1, topologyNumbers(fv...))
	a.add(186, 0, topologyNumbers(186, shell, 1, 0))
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadTetrahedronSolid(bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
