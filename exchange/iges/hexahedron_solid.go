package iges

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"slices"
)

// HexahedronSolid retains a convex six-quad IGES186 solid and original exact
// affine charts, directed model edges and independent owner pcurves.
type HexahedronSolid struct {
	LengthUnit                                                         float64
	Solid                                                              brep.Solid
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	SourceFaceIDs                                                      [6]int
	Geometry                                                           map[int]Entity
	Metadata                                                           map[int]CylinderDatumMetadata
}

func hexaFailure() error { return fmt.Errorf("iges: unsupported exact convex hexahedron topology") }

// ReadHexahedronSolid accepts exactly one type186 solid, one outward type514
// shell and six quadrilateral type510 faces with affine degree-one type128 charts.
// Shared model edges and independent owner pcurves are polynomial degree-one
// type126 curves. Exact coefficient, endpoint, orientation and convexity proofs
// precede native construction. No tolerance, sewing repair or general solid
// inference is used.
func ReadHexahedronSolid(input io.Reader, l Limits) (HexahedronSolid, error) {
	fail := func() (HexahedronSolid, error) { return HexahedronSolid{}, hexaFailure() }
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxEntities = min(l.MaxEntities, 58)
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxControlPoints = min(l.MaxControlPoints, 96)
	d, e := readTopologySource(input, l, true)
	if e != nil {
		return HexahedronSolid{}, e
	}
	if len(d.entities) != 58 {
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
	m := HexahedronSolid{LengthUnit: d.unit, Geometry: map[int]Entity{}, Metadata: map[int]CylinderDatumMetadata{}}
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
	for kind, n := range map[int]int{186: 1, 514: 1, 510: 6, 508: 6, 504: 1, 502: 1, 128: 6, 126: 36} {
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
	v, e = topologyInts(r, 14)
	if e != nil || v[1] != 6 {
		return fail()
	}
	for i := 0; i < 6; i++ {
		if v[3+2*i] != 1 {
			return fail()
		}
		m.SourceFaceIDs[i] = v[2+2*i]
	}
	m.SourceVertexListID = kinds[502][0]
	r, _ = get(m.SourceVertexListID, 502)
	if len(r.values) < 26 {
		return fail()
	}
	kind, e := intValue(r.values[0])
	if e != nil || kind != 502 {
		return fail()
	}
	nv, e := intValue(r.values[1])
	if e != nil || nv != 8 || noPointers(r.values[26:]) != nil {
		return fail()
	}
	vertices := make([]geom.Vec3, 8)
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
	distinct := map[geom.Vec3]bool{}
	for _, v := range vertices {
		if distinct[v] {
			return fail()
		}
		distinct[v] = true
	}
	m.SourceEdgeListID = kinds[504][0]
	r, _ = get(m.SourceEdgeListID, 504)
	v, e = topologyInts(r, 62)
	if e != nil || v[1] != 12 {
		return fail()
	}
	edges := make([]brep.Edge, 12)
	pairs := map[[2]int]bool{}
	for i := range edges {
		a := v[2+5*i : 7+5*i]
		if a[1] != m.SourceVertexListID || a[3] != m.SourceVertexListID || a[2] < 1 || a[2] > 8 || a[4] < 1 || a[4] > 8 || a[2] == a[4] {
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
	faces := make([]brep.Face, 6)
	faceSets := map[[4]int]bool{}
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
		for j, p := range []geom.Vec3{sd.Poles[0], sd.Poles[2], sd.Poles[3], sd.Poles[1]} {
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
			if j == 1 || j == 2 {
				q.X = 1
			}
			if j == 2 || j == 3 {
				q.Y = 1
			}
			uv[found] = q
		}
		if len(uv) != 4 {
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
		lv, e := topologyInts(r, 30)
		if e != nil || lv[1] != 4 {
			return fail()
		}
		var tri [4]int
		for j := 0; j < 4; j++ {
			a := lv[2+7*j : 9+7*j]
			if a[0] != 0 || a[1] != m.SourceEdgeListID || a[2] < 1 || a[2] > 12 || (a[3] != 0 && a[3] != 1) || a[4] != 1 || a[5] != 0 {
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
			if end != tri[(j+1)%4] {
				return fail()
			}
		}
		area := 0.
		for j, x := range tri {
			a, b := uv[x], uv[tri[(j+1)%4]]
			area += a.X*b.Y - a.Y*b.X
		}
		if area != 2 {
			return fail()
		}
		key := tri
		slices.Sort(key[:])
		if faceSets[key] {
			return fail()
		}
		faceSets[key] = true
		for i := range vertices {
			if _, on := uv[i]; !on && tetraDet(vertices[tri[0]], vertices[tri[1]], vertices[tri[2]], vertices[i]) >= 0 {
				return fail()
			}
		}

	}
	if len(used) != len(records) {
		return fail()
	}
	degree := make([]int, 8)
	for _, e := range edges {
		degree[e.Start]++
		degree[e.End]++
	}
	for _, n := range degree {
		if n != 3 {
			return fail()
		}
	}
	reached := map[int]bool{0: true}
	for pass := 0; pass < 8; pass++ {
		for _, e := range edges {
			if reached[e.Start] || reached[e.End] {
				reached[e.Start] = true
				reached[e.End] = true
			}
		}
	}
	if len(reached) != 8 {
		return fail()
	}
	m.Solid, e = brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return fail()
	}
	return m, nil
}

// WriteHexahedronSolid emits a canonical exact parallelepiped. Vertex indexing
// is binary U,V,W: 0=origin,1=U,2=V,3=U+V,4=W,5=U+W,6=V+W,7=U+V+W.
// All eight authored positions and each affine face identity are re-proved;
// reflection is allowed. No point is inferred or snapped during emission.
func WriteHexahedronSolid(w io.Writer, vertices [8]geom.Vec3, unit float64) error {
	if w == nil {
		return hexaFailure()
	}
	for _, v := range vertices {
		if !v.IsFinite() {
			return hexaFailure()
		}
	}
	sign := tetraDet(vertices[0], vertices[1], vertices[2], vertices[4])
	if sign == 0 {
		return hexaFailure()
	}
	a := topologyAuthor{}
	vp := []string{"502", "8"}
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
	edges := [][2]int{}
	for i := 0; i < 8; i++ {
		for _, bit := range []int{1, 2, 4} {
			j := i ^ bit
			if j > i {
				edges = append(edges, [2]int{i, j})
			}
		}
	}
	ev := []int{504, 12}
	for _, e := range edges {
		ev = append(ev, curve(vertices[e[0]], vertices[e[1]]), vid, e[0]+1, vid, e[1]+1)
	}
	eid := a.add(504, 1, topologyNumbers(ev...))
	quads := [][4]int{{0, 2, 3, 1}, {4, 5, 7, 6}, {0, 1, 5, 4}, {2, 6, 7, 3}, {0, 4, 6, 2}, {1, 3, 7, 5}}
	fv := []int{514, 6}
	for _, q := range quads {
		if sign < 0 {
			q[1], q[3] = q[3], q[1]
		}
		g, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{vertices[q[0]], vertices[q[3]], vertices[q[1]], vertices[q[2]]}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			return e
		}
		sid := a.entity(Entity{Surface: &Surface{Geometry: g, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1, Polynomial: true}})
		uv := map[int]geom.Vec3{q[0]: {}, q[1]: {X: 1}, q[2]: {X: 1, Y: 1}, q[3]: {Y: 1}}
		lv := []int{508, 4}
		for j, start := range q {
			end := q[(j+1)%4]
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
	if _, e = ReadHexahedronSolid(bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
