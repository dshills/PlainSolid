package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func torusElbowBody(f *File, name string, faces []int64) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: unsupported bounded torus elbow graph")
	}
	if len(faces) != 3 {
		return fail()
	}
	var sideID, torusID int64
	var caps []int64
	var frame geom.Frame
	major, minor := 0., 0.
	for _, id := range faces {
		r, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(r.Args[2])
		if e != nil {
			return fail()
		}
		s, e := DecodeAnalyticSurface(f, sid)
		if e != nil {
			return fail()
		}
		switch s := s.(type) {
		case geom.TorusSurface:
			if sideID != 0 {
				return fail()
			}
			sideID, torusID = id, sid
			frame, major, minor = s.Parameters()
		case geom.PlaneSurface:
			caps = append(caps, id)
		default:
			return fail()
		}
	}
	if sideID == 0 || len(caps) != 2 {
		return fail()
	}
	side, uses, e := analyticFaceLoop(f, sideID, 4)
	if e != nil {
		return fail()
	}
	sense, e := boolean(side.Args[3])
	if e != nil || !sense {
		return fail()
	}
	found := false
	var seam int64
	var rims [2]int64
	for i := range uses {
		a, b, c, d := uses[i], uses[(i+1)%4], uses[(i+2)%4], uses[(i+3)%4]
		if a.id == c.id && a.id != b.id && a.id != d.id && b.id != d.id && !a.sense && !b.sense && c.sense && d.sense {
			seam = a.id
			rims = [2]int64{b.id, d.id}
			found = true
			break
		}
	}
	if !found {
		return fail()
	}
	start, end, seamGeometry, e := analyticEdge(f, seam)
	if e != nil || start == end {
		return fail()
	}
	tol := (major + minor) * 1e-10
	near := func(a, b geom.Vec3) bool { return a.Distance(b) <= tol }
	dir := func(a, b geom.Vec3) bool { return a.Distance(b) <= 1e-10 }
	circle := func(id int64) (geom.Frame, float64, error) {
		r, e := simple(f, id, "CIRCLE", 3)
		if e != nil {
			return geom.Frame{}, 0, e
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return geom.Frame{}, 0, e
		}
		pid, e := ref(r.Args[1])
		if e != nil {
			return geom.Frame{}, 0, e
		}
		fr, e := placement(f, pid)
		if e != nil {
			return fr, 0, e
		}
		radius, e := numeric(r.Args[2])
		return fr, radius, e
	}
	curve := func(id int64, kind string) (Record, error) {
		r, e := simple(f, id, kind, 4)
		if e != nil {
			return r, e
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return r, e
		}
		if r.Args[3].Kind != Enumeration || (r.Args[3].Text != "PCURVE_S1" && r.Args[3].Text != "PCURVE_S2") {
			return r, errors.New("step: unsupported elbow curve representation")
		}
		return r, nil
	}
	angles := [2]float64{}
	usedCaps := map[int64]bool{}
	for k, rim := range rims {
		vertex, other, g, e := analyticEdge(f, rim)
		if e != nil || vertex != other || (k == 0 && vertex != start) || (k == 1 && vertex != end) {
			return fail()
		}
		sc, e := curve(g, "SURFACE_CURVE")
		if e != nil {
			return fail()
		}
		cid, e := ref(sc.Args[1])
		if e != nil {
			return fail()
		}
		cf, r, e := circle(cid)
		if e != nil || math.Abs(r-minor) > tol {
			return fail()
		}
		pcs, e := listRefs(sc.Args[2], 2)
		if e != nil || len(pcs) != 2 {
			return fail()
		}
		var planeID, planePC int64
		torusFound := false
		for _, pcid := range pcs {
			pc, e := simple(f, pcid, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			support, e := ref(pc.Args[1])
			if e != nil {
				return fail()
			}
			if support == torusID {
				if torusFound {
					return fail()
				}
				torusFound = true
				angles[k], e = torusPCurve(f, pcid, torusID, 1)
				if e != nil {
					return fail()
				}
			} else {
				if planeID != 0 {
					return fail()
				}
				planeID, planePC = support, pcid
			}
		}
		if !torusFound || planeID == 0 {
			return fail()
		}
		u := angles[k]
		if (k == 0 && math.Abs(u) > 1e-10) || (k == 1 && (u <= 0 || u >= 2*math.Pi)) {
			return fail()
		}
		radial := frame.XAxis().Scale(math.Cos(u)).Add(frame.YAxis().Scale(math.Sin(u)))
		normal := radial.Cross(frame.ZAxis())
		center := frame.Origin().Add(radial.Scale(major))
		if !near(cf.Origin(), center) || !dir(cf.XAxis(), radial) || !dir(cf.ZAxis(), normal) {
			return fail()
		}
		point, e := analyticVertex(f, vertex)
		if e != nil || !near(point, center.Add(radial.Scale(minor))) {
			return fail()
		}
		pf, e := planeFrame(f, planeID)
		if e != nil || !dir(pf.XAxis(), radial) || !dir(pf.ZAxis(), normal) || math.Abs(pf.Origin().Sub(center).Dot(normal)) > tol {
			return fail()
		}
		local := pf.ToLocal(center)
		if !local.IsFinite() || !near(pf.ToWorld(local), center) {
			return fail()
		}
		if e = offsetCapPCurve(f, planePC, planeID, minor, geom.Vec2{X: local.X, Y: local.Y}); e != nil {
			return fail()
		}
		capFound := false
		for _, capID := range caps {
			r, u, e := analyticFaceLoop(f, capID, 1)
			if e != nil {
				return fail()
			}
			sid, e := ref(r.Args[2])
			if e != nil {
				return fail()
			}
			if sid != planeID {
				continue
			}
			if capFound || usedCaps[capID] || u[0].id != rim || u[0].sense != (k == 0) {
				return fail()
			}
			sense, e := boolean(r.Args[3])
			if e != nil || sense != (k == 0) {
				return fail()
			}
			capFound = true
			usedCaps[capID] = true
		}
		if !capFound {
			return fail()
		}
	}
	sc, e := curve(seamGeometry, "SEAM_CURVE")
	if e != nil {
		return fail()
	}
	cid, e := ref(sc.Args[1])
	if e != nil {
		return fail()
	}
	cf, r, e := circle(cid)
	if e != nil || math.Abs(r-major-minor) > tol || !near(cf.Origin(), frame.Origin()) || !dir(cf.XAxis(), frame.XAxis()) || !dir(cf.ZAxis(), frame.ZAxis()) {
		return fail()
	}
	pcs, e := listRefs(sc.Args[2], 2)
	if e != nil || len(pcs) != 2 {
		return fail()
	}
	a, e := torusPCurve(f, pcs[0], torusID, 0)
	if e != nil {
		return fail()
	}
	b, e := torusPCurve(f, pcs[1], torusID, 0)
	if e != nil || !((math.Abs(a) < 1e-10 && math.Abs(b-2*math.Pi) < 1e-10) || (math.Abs(b) < 1e-10 && math.Abs(a-2*math.Pi) < 1e-10)) {
		return fail()
	}
	solid, e := parametric.TorusElbow(frame, major, minor, angles[1])
	return AnalyticBody{name, solid}, e
}

// Preserve the exact plane-to-circle offset while reusing strict cap pcurve
// context, axis, radius and representation validation.
func offsetCapPCurve(f *File, id, surface int64, radius float64, center geom.Vec2) error {
	fail := func() error { return errors.New("step: invalid offset cap pcurve") }
	pc, e := simple(f, id, "PCURVE", 3)
	if e != nil {
		return fail()
	}
	rid, e := ref(pc.Args[2])
	if e != nil {
		return fail()
	}
	rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	ids, e := listRefs(rep.Args[1], 1)
	if e != nil || len(ids) != 1 {
		return fail()
	}
	circle, e := simple(f, ids[0], "CIRCLE", 3)
	if e != nil {
		return fail()
	}
	aid, e := ref(circle.Args[1])
	if e != nil {
		return fail()
	}
	axis, e := simple(f, aid, "AXIS2_PLACEMENT_2D", 3)
	if e != nil {
		return fail()
	}
	pid, e := ref(axis.Args[1])
	if e != nil {
		return fail()
	}
	p, e := coordinate(f, pid, "CARTESIAN_POINT", 2)
	if e != nil || math.Hypot(p.X-center.X, p.Y-center.Y) > 1e-10*radius {
		return fail()
	}
	local := *f
	local.Entities = make(map[int64]Entity, len(f.Entities))
	for key, ent := range f.Entities {
		local.Entities[key] = ent
	}
	record := f.Entities[pid].Records[0]
	record.Args = append([]Value(nil), record.Args...)
	record.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}
	local.Entities[pid] = Entity{ID: pid, Records: []Record{record}}
	return capPCurve(&local, id, surface, radius)
}
