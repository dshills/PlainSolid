package iges

import (
	"fmt"
	"io"
	"plainsolid/geom"
	"reflect"
)

// CurvedOvalShell retains an explicit IGES type514 open shell, two type510
// faces, and shared quarter-edge incidence. Each original chart and every
// continuous pcurve/model relation is certified. Global intersection between
// face interiors is not certified, and no material closure is inferred.
// Use ReadCurvedOvalShell to obtain a source-backed value for lossless write-back.
type CurvedOvalShell struct {
	Sheet                             CurvedOvalSheet
	ShellID, EdgeListID, VertexListID int
	FaceIDs                           [2]int
	Edges                             []CurvedShellEdge
	Vertices                          []geom.Vec3
	Loops                             []CurvedShellLoop
	source                            topologySource
}
type CurvedShellEdge struct{ ModelCurveID, StartVertex, EndVertex int }
type CurvedShellUse struct {
	Edge, ParameterCurveID int
	Forward                bool
}
type CurvedShellLoop struct {
	SourceID, Face int
	Uses           []CurvedShellUse
}

func ReadCurvedOvalShell(r io.Reader, l Limits) (CurvedOvalShell, error) {
	d, e := readTopologySource(r, l)
	if e != nil {
		return CurvedOvalShell{}, e
	}
	return certifyCurvedShell(d, l)
}

// WriteCurvedOvalShell rechecks original records, exact geometry, incidence and
// all exposed ancestry. Edited or fabricated cached results are rejected.
func WriteCurvedOvalShell(w io.Writer, s CurvedOvalShell) error {
	q, e := certifyCurvedShell(s.source, Limits{})
	if e != nil {
		return e
	}
	a, b := s, q
	a.source = topologySource{}
	b.source = topologySource{}
	if !reflect.DeepEqual(a, b) {
		return geom.ErrInvalid
	}
	if w == nil {
		return geom.ErrInvalid
	}
	n, e := w.Write(s.source.bytes)
	if e == nil && n != len(s.source.bytes) {
		return io.ErrShortWrite
	}
	return e
}
func topologyInts(r topologyRecord, n int) ([]int, error) {
	if n < 1 || len(r.values) < n {
		return nil, geom.ErrInvalid
	}
	v := make([]int, n)
	for i := range v {
		x, e := intValue(r.values[i])
		if e != nil {
			return nil, e
		}
		v[i] = x
	}
	if v[0] != r.kind {
		return nil, geom.ErrInvalid
	}
	if e := noPointers(r.values[n:]); e != nil {
		return nil, e
	}
	return v, nil
}
func joinShellCurves(parts []*geom.ExactSplineCurve) (*geom.BSpline, error) {
	var poles []geom.Vec3
	var weights, knots []float64
	degree := -1
	for i, part := range parts {
		q, e := part.Reparameterize(float64(i), float64(i+1), false, curvedOvalOptions())
		if e != nil {
			return nil, e
		}
		s, e := q.ToBSpline()
		if e != nil {
			return nil, e
		}
		p, w := s.Poles(), s.Weights()
		if degree < 0 {
			degree = s.Degree()
			for j := 0; j <= degree; j++ {
				knots = append(knots, 0)
			}
		}
		if s.Degree() != degree || len(p) != degree+1 {
			return nil, geom.ErrInvalid
		}
		if i > 0 {
			if p[0] != poles[len(poles)-1] || w[0] != weights[len(weights)-1] {
				return nil, geom.ErrUnrepresentable
			}
			p, w = p[1:], w[1:]
			for j := 0; j < degree; j++ {
				knots = append(knots, float64(i))
			}
		}
		poles = append(poles, p...)
		weights = append(weights, w...)
	}
	for j := 0; j <= degree; j++ {
		knots = append(knots, float64(len(parts)))
	}
	return geom.NewBSpline(degree, poles, knots, weights)
}
func certifyCurvedShell(d topologySource, l Limits) (CurvedOvalShell, error) {
	fail := func() (CurvedOvalShell, error) {
		return CurvedOvalShell{}, fmt.Errorf("iges: unsupported or inconsistent curved open shell")
	}
	if len(d.bytes) == 0 || len(d.entities) != 30 {
		return fail()
	}
	limits, e := l.defaults()
	if e != nil {
		return CurvedOvalShell{}, e
	}
	q := CurvedOvalShell{source: d}
	records := map[int]topologyRecord{}
	geometry := map[int]Entity{}
	byType := map[int][]int{}
	used := map[int]bool{}
	count := 0
	for _, r := range d.entities {
		records[r.id] = r
		byType[r.kind] = append(byType[r.kind], r.id)
		if r.kind < 500 {
			en, e := decode(r.kind, r.values, limits.MaxControlPoints-count)
			if e != nil {
				return CurvedOvalShell{}, fmt.Errorf("record %d: %w", r.id, e)
			}
			en.Label = r.label
			en.Subscript = r.subscript
			if en.sourceWeightsRounded {
				return CurvedOvalShell{}, geom.ErrUnrepresentable
			}
			if e = en.validate(); e != nil {
				return CurvedOvalShell{}, fmt.Errorf("record %d: %w", r.id, e)
			}
			if en.Surface != nil {
				count += len(en.Surface.Geometry.Data().Poles)
			}
			if en.Spline != nil {
				count += len(en.Spline.Curve.Poles())
			}
			geometry[r.id] = en
		}
	}
	if len(byType[514]) != 1 || len(byType[510]) != 2 || len(byType[508]) != 3 || len(byType[504]) != 1 || len(byType[502]) != 1 || len(byType[128]) != 2 || len(byType[126]) != 20 {
		return fail()
	}
	get := func(id, kind int) (topologyRecord, bool) {
		r, ok := records[id]
		if !ok || r.kind != kind {
			return r, false
		}
		used[id] = true
		return r, true
	}
	q.ShellID = byType[514][0]
	r, _ := get(q.ShellID, 514)
	v, e := topologyInts(r, 6)
	if e != nil || v[1] != 2 || v[3] != 1 || v[5] != 1 || v[2] == v[4] {
		return fail()
	}
	q.FaceIDs = [2]int{v[2], v[4]}
	q.EdgeListID = byType[504][0]
	q.VertexListID = byType[502][0]
	r, _ = get(q.VertexListID, 502)
	if len(r.values) < 26 {
		return fail()
	}
	nv, e := intValue(r.values[1])
	if e != nil || nv != 8 {
		return fail()
	}
	if x, e := intValue(r.values[0]); e != nil || x != 502 {
		return fail()
	}
	if e = noPointers(r.values[26:]); e != nil {
		return fail()
	}
	unique := map[geom.Vec3]bool{}
	for i := 0; i < 8; i++ {
		var c [3]float64
		for j := 0; j < 3; j++ {
			c[j], e = realValue(r.values[2+3*i+j])
			if e != nil {
				return fail()
			}
		}
		p := geom.Vec3{X: c[0], Y: c[1], Z: c[2]}
		if unique[p] {
			return fail()
		}
		unique[p] = true
		q.Vertices = append(q.Vertices, p)
	}
	r, _ = get(q.EdgeListID, 504)
	v, e = topologyInts(r, 42)
	if e != nil || v[1] != 8 {
		return fail()
	}
	modelOwners := map[int]bool{}
	for i := 0; i < 8; i++ {
		a := v[2+5*i : 7+5*i]
		if a[1] != q.VertexListID || a[3] != q.VertexListID || a[2] < 1 || a[2] > 8 || a[4] < 1 || a[4] > 8 || a[2] == a[4] || modelOwners[a[0]] {
			return fail()
		}
		modelOwners[a[0]] = true
		if _, ok := get(a[0], 126); !ok {
			return fail()
		}
		g := geometry[a[0]]
		if g.Spline == nil {
			return fail()
		}
		x, e := geom.NewExactSplineCurve(g.Spline.Curve, curvedOvalOptions())
		if e != nil {
			return fail()
		}
		start, e := x.ExactPoint(g.Spline.Start)
		if e != nil {
			return fail()
		}
		end, e := x.ExactPoint(g.Spline.End)
		if e != nil || start != q.Vertices[a[2]-1] || end != q.Vertices[a[4]-1] {
			return fail()
		}
		q.Edges = append(q.Edges, CurvedShellEdge{a[0], a[2] - 1, a[4] - 1})
	}
	m := Model{LengthUnit: d.unit}
	loopSeen := map[int]bool{}
	uvSeen := map[int]bool{}
	surfaceSeen := map[int]bool{}
	incidence := make([][]int, 8)
	for f, fid := range q.FaceIDs {
		r, ok := get(fid, 510)
		if !ok || len(r.values) < 4 {
			return fail()
		}
		n, e := intValue(r.values[2])
		if e != nil || n < 1 || n > 2 {
			return fail()
		}
		v, e := topologyInts(r, 4+n)
		if e != nil || v[3] != 1 || surfaceSeen[v[1]] {
			return fail()
		}
		surfaceSeen[v[1]] = true
		if _, ok = get(v[1], 128); !ok {
			return fail()
		}
		support := len(m.Entities)
		m.Entities = append(m.Entities, geometry[v[1]])
		trim := TrimmedSurface{Surface: support, Label: r.label, Subscript: r.subscript}
		for j, lid := range v[4:] {
			if loopSeen[lid] {
				return fail()
			}
			loopSeen[lid] = true
			r, ok = get(lid, 508)
			if !ok {
				return fail()
			}
			lv, e := topologyInts(r, 30)
			if e != nil || lv[1] != 4 {
				return fail()
			}
			loop := CurvedShellLoop{SourceID: lid, Face: f}
			var up, mp []*geom.ExactSplineCurve
			first, last := -1, -1
			for k := 0; k < 4; k++ {
				a := lv[2+7*k : 9+7*k]
				if a[0] != 0 || a[1] != q.EdgeListID || a[2] < 1 || a[2] > 8 || (a[3] != 0 && a[3] != 1) || a[4] != 1 || a[5] != 0 || uvSeen[a[6]] || modelOwners[a[6]] {
					return fail()
				}
				uvSeen[a[6]] = true
				if _, ok = get(a[6], 126); !ok {
					return fail()
				}
				edge := a[2] - 1
				forward := a[3] == 1
				ed := q.Edges[edge]
				start, end := ed.StartVertex, ed.EndVertex
				if !forward {
					start, end = end, start
				}
				if k == 0 {
					first = start
				} else if start != last {
					return fail()
				}
				last = end
				use := CurvedShellUse{Edge: edge, ParameterCurveID: a[6], Forward: forward}
				loop.Uses = append(loop.Uses, use)
				sign := 1
				if !forward {
					sign = -1
				}
				incidence[edge] = append(incidence[edge], sign)
				uv, md := geometry[a[6]].Spline, geometry[ed.ModelCurveID].Spline
				if uv == nil || md == nil {
					return fail()
				}
				ux, e := geom.NewExactSplineCurve(uv.Curve, curvedOvalOptions())
				if e != nil {
					return fail()
				}
				ux, e = ux.Restrict(uv.Start, uv.End, curvedOvalOptions())
				if e != nil {
					return fail()
				}
				mx, e := geom.NewExactSplineCurve(md.Curve, curvedOvalOptions())
				if e != nil {
					return fail()
				}
				mx, e = mx.Restrict(md.Start, md.End, curvedOvalOptions())
				if e != nil {
					return fail()
				}
				mx, e = mx.Reparameterize(0, 1, !forward, curvedOvalOptions())
				if e != nil {
					return fail()
				}
				up = append(up, ux)
				mp = append(mp, mx)
			}
			if first != last {
				return fail()
			}
			uc, e := joinShellCurves(up)
			if e != nil {
				return CurvedOvalShell{}, fmt.Errorf("UV join: %w", e)
			}
			mc, e := joinShellCurves(mp)
			if e != nil {
				return CurvedOvalShell{}, fmt.Errorf("model join: %w", e)
			}
			idx := len(m.Entities)
			m.Entities = append(m.Entities, Entity{Spline: &Spline{Curve: uc, Start: 0, End: 4, Planar: true, Closed: true, Normal: geom.Vec3{Z: 1}}}, Entity{Spline: &Spline{Curve: mc, Start: 0, End: 4, Closed: true}})
			ci := len(m.CurvesOnSurfaces)
			m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, CurveOnSurface{Surface: support, ParameterCurve: idx, ModelCurve: idx + 1, Preference: 3})
			if j == 0 {
				trim.Outer = ci
			} else {
				trim.Inner = append(trim.Inner, ci)
			}
			q.Loops = append(q.Loops, loop)
		}
		m.TrimmedSurfaces = append(m.TrimmedSurfaces, trim)
	}
	if len(used) != len(records) {
		return fail()
	}
	once, twice := 0, 0
	for _, uses := range incidence {
		if len(uses) == 1 {
			once++
		} else if len(uses) == 2 && uses[0] == -uses[1] {
			twice++
		} else {
			return fail()
		}
	}
	if once != 4 || twice != 4 {
		return fail()
	}
	q.Sheet, e = CertifyCurvedOvalSheet(m)
	if e != nil {
		return CurvedOvalShell{}, fmt.Errorf("sheet: %w", e)
	}
	// Shared incidence must be exactly the hole/fill loops, not an outer seam.
	firstLoop := map[int]bool{}
	for _, loop := range q.Loops {
		shared := loop.Face == q.Sheet.Disk || firstLoop[loop.Face]
		firstLoop[loop.Face] = true
		for _, use := range loop.Uses {
			if (len(incidence[use.Edge]) == 2) != shared {
				return fail()
			}
		}
	}
	return q, nil
}
