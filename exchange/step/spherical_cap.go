package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func sphericalCapBody(f *File, name string, faces []int64) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: unsupported spherical cap graph")
	}
	if len(faces) != 2 {
		return fail()
	}
	var sideID, capID, sphereID, planeID int64
	var frame geom.Frame
	radius := 0.
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
		case geom.SphereSurface:
			if sideID != 0 {
				return fail()
			}
			sideID, sphereID = id, sid
			frame, radius = s.Parameters()
		case geom.PlaneSurface:
			capID, planeID = id, sid
		default:
			return fail()
		}
	}
	if sideID == 0 || capID == 0 {
		return fail()
	}
	side, uses, e := analyticFaceLoop(f, sideID, 3)
	if e != nil {
		return fail()
	}
	sense, e := boolean(side.Args[3])
	if e != nil || !sense {
		return fail()
	}
	// A seam is used twice oppositely; the full latitude rim occurs once forward.
	found := false
	var seam, rim int64
	for i := range uses {
		a, b, c := uses[i], uses[(i+1)%3], uses[(i+2)%3]
		if a.id == c.id && a.id != b.id && !a.sense && b.sense && c.sense {
			seam, rim = a.id, b.id
			found = true
			break
		}
	}
	if !found {
		return fail()
	}
	vertex, v2, rg, e := analyticEdge(f, rim)
	if e != nil || vertex != v2 {
		return fail()
	}
	start, pole, sg, e := analyticEdge(f, seam)
	if e != nil || start != vertex || pole == vertex {
		return fail()
	}
	near := func(a, b geom.Vec3) bool { return a.Distance(b) <= radius*1e-10 }
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
		r0, e := numeric(r.Args[2])
		if e != nil || r0 <= 0 {
			return fr, 0, errors.New("step: invalid cap circle")
		}
		return fr, r0, nil
	}
	surfaceCurve, e := simple(f, rg, "SURFACE_CURVE", 4)
	if e != nil {
		return fail()
	}
	cid, e := ref(surfaceCurve.Args[1])
	if e != nil {
		return fail()
	}
	cf, rimRadius, e := circle(cid)
	if e != nil || !dir(cf.ZAxis(), frame.ZAxis()) || !dir(cf.XAxis(), frame.XAxis()) {
		return fail()
	}
	local := frame.ToLocal(cf.Origin())
	if math.Hypot(local.X, local.Y) > radius*1e-10 || math.Abs(local.Z) >= radius {
		return fail()
	}
	latitude := math.Asin(local.Z / radius)
	if math.Abs(rimRadius-radius*math.Cos(latitude)) > radius*1e-10 {
		return fail()
	}
	point, e := analyticVertex(f, vertex)
	if e != nil || !near(point, cf.Origin().Add(cf.XAxis().Scale(rimRadius))) {
		return fail()
	}
	point, e = analyticVertex(f, pole)
	if e != nil || !near(point, frame.Origin().Add(frame.ZAxis().Scale(radius))) {
		return fail()
	}
	plane, e := planeFrame(f, planeID)
	if e != nil || !near(plane.Origin(), cf.Origin()) || !dir(plane.XAxis(), frame.XAxis()) || !dir(plane.ZAxis(), frame.ZAxis()) {
		return fail()
	}
	pcs, e := listRefs(surfaceCurve.Args[2], 2)
	if e != nil || len(pcs) != 2 {
		return fail()
	}
	seen := map[int64]bool{}
	for _, id := range pcs {
		pc, e := simple(f, id, "PCURVE", 3)
		if e != nil {
			return fail()
		}
		support, e := ref(pc.Args[1])
		if e != nil || seen[support] {
			return fail()
		}
		seen[support] = true
		switch support {
		case sphereID:
			v, e := torusPCurve(f, id, sphereID, 0)
			if e != nil || math.Abs(v-latitude) > 1e-10 {
				return fail()
			}
		case planeID:
			if e := capPCurve(f, id, planeID, rimRadius); e != nil {
				return fail()
			}
		default:
			return fail()
		}
	}
	seamCurve, e := simple(f, sg, "SEAM_CURVE", 4)
	if e != nil {
		return fail()
	}
	mc, e := ref(seamCurve.Args[1])
	if e != nil {
		return fail()
	}
	mf, mr, e := circle(mc)
	if e != nil || math.Abs(mr-radius) > radius*1e-10 || !near(mf.Origin(), frame.Origin()) || !dir(mf.XAxis(), frame.XAxis()) || !dir(mf.ZAxis(), frame.YAxis().Scale(-1)) {
		return fail()
	}
	pcs, e = listRefs(seamCurve.Args[2], 2)
	if e != nil || len(pcs) != 2 {
		return fail()
	}
	a, e := sphereSeamPCurve(f, pcs[0], sphereID)
	if e != nil {
		return fail()
	}
	b, e := sphereSeamPCurve(f, pcs[1], sphereID)
	if e != nil || !((math.Abs(a) < 1e-10 && math.Abs(b-2*math.Pi) < 1e-10) || (math.Abs(b) < 1e-10 && math.Abs(a-2*math.Pi) < 1e-10)) {
		return fail()
	}
	for _, r := range []Record{surfaceCurve, seamCurve} {
		if _, e := textValue(r.Args[0]); e != nil {
			return fail()
		}
		if r.Args[3].Kind != Enumeration || (r.Args[3].Text != "PCURVE_S1" && r.Args[3].Text != "PCURVE_S2") {
			return fail()
		}
	}
	cap, cu, e := analyticFaceLoop(f, capID, 1)
	if e != nil || cu[0].id != rim || cu[0].sense {
		return fail()
	}
	sense, e = boolean(cap.Args[3])
	if e != nil || sense {
		return fail()
	}
	cut, e := geom.NewPlane(cf.Origin(), frame.ZAxis().Scale(-1))
	if e != nil {
		return fail()
	}
	solid, e := parametric.ClipSphere(parametric.Ball{Center: frame.Origin(), Radius: radius}, cut, radius*1e-10)
	return AnalyticBody{name, solid}, e
}

// A full-period meridian parameter shift preserves the exact model circle.
func sphereSeamPCurve(f *File, id, surface int64) (float64, error) {
	fail := func() (float64, error) { return 0, errors.New("step: unsupported sphere seam phase") }
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
	line, e := simple(f, ids[0], "LINE", 3)
	if e != nil {
		return fail()
	}
	pid, e := ref(line.Args[1])
	if e != nil {
		return fail()
	}
	point, e := coordinate(f, pid, "CARTESIAN_POINT", 2)
	if e != nil {
		return fail()
	}
	turns := math.Round(point.Y / (2 * math.Pi))
	if math.Abs(turns) > 1e6 || math.Abs(point.Y-turns*2*math.Pi) > 1e-10 {
		return fail()
	}
	local := *f
	local.Entities = make(map[int64]Entity, len(f.Entities))
	for key, ent := range f.Entities {
		local.Entities[key] = ent
	}
	ent := f.Entities[pid]
	record := ent.Records[0]
	record.Args = append([]Value(nil), record.Args...)
	record.Args[1].Items = append([]Value(nil), record.Args[1].Items...)
	record.Args[1].Items[1] = Value{Kind: Number, Text: "0."}
	ent.Records = []Record{record}
	local.Entities[pid] = ent
	return torusPCurve(&local, id, surface, 1)
}
