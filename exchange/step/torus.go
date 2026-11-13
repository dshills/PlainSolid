package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func torusPCurve(f *File, id, surface int64, axis int) (float64, error) {
	return signedPCurve(f, id, surface, axis, 1)
}
func signedPCurve(f *File, id, surface int64, axis int, sign float64) (float64, error) {
	fail := func() (float64, error) { return 0, errors.New("step: unsupported torus seam pcurve") }
	r, e := simple(f, id, "PCURVE", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return fail()
	}
	sid, e := ref(r.Args[1])
	if e != nil || sid != surface {
		return fail()
	}
	rid, e := ref(r.Args[2])
	if e != nil {
		return fail()
	}
	rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(rep.Args[0]); e != nil {
		return fail()
	}
	ids, e := listRefs(rep.Args[1], 1)
	if e != nil || len(ids) != 1 {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	seen := map[string]bool{}
	for _, r := range f.Entities[ctx].Records {
		if seen[r.Type] {
			return fail()
		}
		seen[r.Type] = true
		switch r.Type {
		case "GEOMETRIC_REPRESENTATION_CONTEXT":
			if len(r.Args) != 1 {
				return fail()
			}
			n, e := integer(r.Args[0])
			if e != nil || n != 2 {
				return fail()
			}
		case "PARAMETRIC_REPRESENTATION_CONTEXT":
			if len(r.Args) != 0 {
				return fail()
			}
		case "REPRESENTATION_CONTEXT":
			if len(r.Args) != 2 || r.Args[0].Kind != String || r.Args[1].Kind != String {
				return fail()
			}
		default:
			return fail()
		}
	}
	if !seen["GEOMETRIC_REPRESENTATION_CONTEXT"] || !seen["PARAMETRIC_REPRESENTATION_CONTEXT"] || !seen["REPRESENTATION_CONTEXT"] {
		return fail()
	}
	p, d, e := lineGeometry(f, ids[0], 2)
	if e != nil {
		return fail()
	}
	line, _ := simple(f, ids[0], "LINE", 3)
	vectorID, _ := ref(line.Args[2])
	vector, e := simple(f, vectorID, "VECTOR", 3)
	if e != nil {
		return fail()
	}
	m, e := numeric(vector.Args[2])
	if e != nil || math.Abs(m-1) > 1e-12 {
		return fail()
	}
	p = p.Scale(sign)
	d = d.Scale(sign)
	if axis == 0 {
		if math.Abs(p.X) > 1e-10 || d.Distance(geom.Vec3{X: 1}) > 1e-10 {
			return fail()
		}
		return p.Y, nil
	}
	if math.Abs(p.Y) > 1e-10 || d.Distance(geom.Vec3{Y: 1}) > 1e-10 {
		return fail()
	}
	return p.X, nil
}
func torusSeam(f *File, id, sid int64, surface geom.TorusSurface) (axis int, vertex int64, err error) {
	fail := func() (int, int64, error) { return 0, 0, errors.New("step: unsupported torus seam edge") }
	edge, e := simple(f, id, "EDGE_CURVE", 5)
	if e != nil {
		return fail()
	}
	if _, e = textValue(edge.Args[0]); e != nil {
		return fail()
	}
	v, e := ref(edge.Args[1])
	if e != nil {
		return fail()
	}
	end, e := ref(edge.Args[2])
	if e != nil || end != v {
		return fail()
	}
	sense, e := boolean(edge.Args[4])
	if e != nil || !sense {
		return fail()
	}
	curveID, e := ref(edge.Args[3])
	if e != nil {
		return fail()
	}
	curve, e := simple(f, curveID, "SEAM_CURVE", 4)
	if e != nil {
		return fail()
	}
	if _, e = textValue(curve.Args[0]); e != nil {
		return fail()
	}
	if curve.Args[3].Kind != Enumeration || (curve.Args[3].Text != "PCURVE_S1" && curve.Args[3].Text != "PCURVE_S2") {
		return fail()
	}
	cid, e := ref(curve.Args[1])
	if e != nil {
		return fail()
	}
	circle, e := simple(f, cid, "CIRCLE", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(circle.Args[0]); e != nil {
		return fail()
	}
	pid, e := ref(circle.Args[1])
	if e != nil {
		return fail()
	}
	cf, e := placement(f, pid)
	if e != nil {
		return fail()
	}
	radius, e := numeric(circle.Args[2])
	if e != nil {
		return fail()
	}
	frame, R, r := surface.Parameters()
	scale := R + r
	near := func(a, b geom.Vec3) bool { return a.Sub(b).Scale(1/scale).Length() <= 1e-10 }
	axis = 0
	center, normal, rad := frame.Origin(), frame.ZAxis(), scale
	if math.Abs(radius-r)/scale <= 1e-10 {
		axis = 1
		center = frame.Origin().Add(frame.XAxis().Scale(R))
		normal = frame.YAxis().Scale(-1)
		rad = r
	}
	if math.Abs(radius-rad)/scale > 1e-10 || !near(cf.Origin(), center) || cf.ZAxis().Distance(normal) > 1e-10 || cf.XAxis().Distance(frame.XAxis()) > 1e-10 {
		return fail()
	}
	pcurves, e := listRefs(curve.Args[2], 2)
	if e != nil || len(pcurves) != 2 {
		return fail()
	}
	a, e := torusPCurve(f, pcurves[0], sid, axis)
	if e != nil {
		return fail()
	}
	b, e := torusPCurve(f, pcurves[1], sid, axis)
	if e != nil {
		return fail()
	}
	if !((math.Abs(a) < 1e-10 && math.Abs(b-2*math.Pi) < 1e-10) || (math.Abs(b) < 1e-10 && math.Abs(a-2*math.Pi) < 1e-10)) {
		return fail()
	}
	vp, e := simple(f, v, "VERTEX_POINT", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(vp.Args[0]); e != nil {
		return fail()
	}
	pointID, e := ref(vp.Args[1])
	if e != nil {
		return fail()
	}
	point, e := triple(f, pointID, "CARTESIAN_POINT")
	if e != nil || !near(point, frame.Origin().Add(frame.XAxis().Scale(scale))) {
		return fail()
	}
	return axis, v, nil
}
func torusBody(f *File, name string, face Record, sid int64, surface geom.TorusSurface) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: only complete canonical double-seam ring tori supported")
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return fail()
	}
	ent := f.Entities[bounds[0]]
	if len(ent.Records) != 1 || (ent.Records[0].Type != "FACE_BOUND" && ent.Records[0].Type != "FACE_OUTER_BOUND") {
		return fail()
	}
	bound, e := simple(f, bounds[0], ent.Records[0].Type, 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(bound.Args[0]); e != nil {
		return fail()
	}
	sense, e := boolean(bound.Args[2])
	if e != nil || !sense {
		return fail()
	}
	lid, e := ref(bound.Args[1])
	if e != nil {
		return fail()
	}
	loop, e := simple(f, lid, "EDGE_LOOP", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(loop.Args[0]); e != nil {
		return fail()
	}
	uses, e := listRefs(loop.Args[1], 4)
	if e != nil || len(uses) != 4 {
		return fail()
	}
	type use struct {
		id    int64
		axis  int
		sense bool
	}
	var got [4]use
	vertex := int64(0)
	edges := map[int64]int{}
	for i, id := range uses {
		r, e := simple(f, id, "ORIENTED_EDGE", 5)
		if e != nil {
			return fail()
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return fail()
		}
		if r.Args[1].Kind != Derived || r.Args[2].Kind != Derived {
			return fail()
		}
		eid, e := ref(r.Args[3])
		if e != nil {
			return fail()
		}
		sense, e := boolean(r.Args[4])
		if e != nil {
			return fail()
		}
		axis, v, e := torusSeam(f, eid, sid, surface)
		if e != nil {
			return AnalyticBody{}, e
		}
		if vertex != 0 && v != vertex {
			return fail()
		}
		vertex = v
		got[i] = use{eid, axis, sense}
		edges[eid]++
	}
	if len(edges) != 2 {
		return fail()
	}
	for _, n := range edges {
		if n != 2 {
			return fail()
		}
	}
	valid := false
	for shift := 0; shift < 4; shift++ {
		a, b, c, d := got[shift], got[(shift+1)%4], got[(shift+2)%4], got[(shift+3)%4]
		if a.axis == 0 && !a.sense && b.axis == 1 && !b.sense && c.id == a.id && c.sense && d.id == b.id && d.sense {
			valid = true
		}
	}
	if !valid {
		return fail()
	}
	frame, R, r := surface.Parameters()
	solid, e := parametric.Torus(frame, R, r)
	if e != nil {
		return AnalyticBody{}, e
	}
	return AnalyticBody{name, solid}, nil
}
