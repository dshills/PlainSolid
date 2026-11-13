package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type analyticUse struct {
	id    int64
	sense bool
}

func analyticFaceLoop(f *File, id int64, n int) (Record, []analyticUse, error) {
	fail := func() (Record, []analyticUse, error) {
		return Record{}, nil, errors.New("step: unsupported analytic face loop")
	}
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return fail()
	}
	if _, e = textValue(face.Args[0]); e != nil {
		return fail()
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return fail()
	}
	ent := f.Entities[bounds[0]]
	if len(ent.Records) != 1 || (ent.Records[0].Type != "FACE_BOUND" && ent.Records[0].Type != "FACE_OUTER_BOUND") {
		return fail()
	}
	b, e := simple(f, bounds[0], ent.Records[0].Type, 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(b.Args[0]); e != nil {
		return fail()
	}
	sense, e := boolean(b.Args[2])
	if e != nil || !sense {
		return fail()
	}
	lid, e := ref(b.Args[1])
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
	ids, e := listRefs(loop.Args[1], n)
	if e != nil || len(ids) != n {
		return fail()
	}
	out := make([]analyticUse, n)
	for i, id := range ids {
		u, e := simple(f, id, "ORIENTED_EDGE", 5)
		if e != nil || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived {
			return fail()
		}
		if _, e = textValue(u.Args[0]); e != nil {
			return fail()
		}
		eid, e := ref(u.Args[3])
		if e != nil {
			return fail()
		}
		sense, e := boolean(u.Args[4])
		if e != nil {
			return fail()
		}
		out[i] = analyticUse{eid, sense}
	}
	return face, out, nil
}
func analyticEdge(f *File, id int64) (start, end, geometry int64, err error) {
	r, e := simple(f, id, "EDGE_CURVE", 5)
	if e != nil {
		return 0, 0, 0, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return 0, 0, 0, e
	}
	a, e := ref(r.Args[1])
	if e != nil {
		return 0, 0, 0, e
	}
	b, e := ref(r.Args[2])
	if e != nil {
		return 0, 0, 0, e
	}
	g, e := ref(r.Args[3])
	if e != nil {
		return 0, 0, 0, e
	}
	sense, e := boolean(r.Args[4])
	if e != nil || !sense {
		return 0, 0, 0, errors.New("step: reversed analytic edge geometry unsupported")
	}
	return a, b, g, nil
}
func analyticVertex(f *File, id int64) (geom.Vec3, error) {
	v, e := simple(f, id, "VERTEX_POINT", 2)
	if e != nil {
		return geom.Vec3{}, e
	}
	if _, e = textValue(v.Args[0]); e != nil {
		return geom.Vec3{}, e
	}
	p, e := ref(v.Args[1])
	if e != nil {
		return geom.Vec3{}, e
	}
	return triple(f, p, "CARTESIAN_POINT")
}
func capPCurve(f *File, id, surface int64, radius float64) error {
	fail := func() error { return errors.New("step: unsupported cylinder cap pcurve") }
	p, e := simple(f, id, "PCURVE", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(p.Args[0]); e != nil {
		return fail()
	}
	sid, e := ref(p.Args[1])
	if e != nil || sid != surface {
		return fail()
	}
	rid, e := ref(p.Args[2])
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
	if len(seen) != 3 {
		return fail()
	}
	circle, e := simple(f, ids[0], "CIRCLE", 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(circle.Args[0]); e != nil {
		return fail()
	}
	rad, e := numeric(circle.Args[2])
	if e != nil || math.Abs(rad-radius)/radius > 1e-10 {
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
	if _, e = textValue(axis.Args[0]); e != nil {
		return fail()
	}
	oid, e := ref(axis.Args[1])
	if e != nil {
		return fail()
	}
	origin, e := coordinate(f, oid, "CARTESIAN_POINT", 2)
	if e != nil || origin.Scale(1/radius).Length() > 1e-10 {
		return fail()
	}
	did, e := ref(axis.Args[2])
	if e != nil {
		return fail()
	}
	d, e := coordinate(f, did, "DIRECTION", 2)
	if e != nil {
		return fail()
	}
	d, e = d.Unit()
	if e != nil || d.Distance(geom.Vec3{X: 1}) > 1e-10 {
		return fail()
	}
	return nil
}
func cylinderBody(f *File, name string, faces []int64) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: only full canonical finite cylinder topology supported")
	}
	if len(faces) != 3 && len(faces) != 2 {
		return fail()
	}
	var side Record
	var uses []analyticUse
	var caps []int64
	var sid int64
	var frame geom.Frame
	var radius, slope float64
	sign := 1.0
	conical := false
	for _, id := range faces {
		r, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		s, e := ref(r.Args[2])
		if e != nil {
			return fail()
		}
		surface, e := DecodeAnalyticSurface(f, s)
		if e != nil {
			return fail()
		}
		switch v := surface.(type) {
		case geom.CylinderSurface, ConicalSurface:
			if sid != 0 {
				return fail()
			}
			sid = s
			switch s := v.(type) {
			case geom.CylinderSurface:
				frame, radius = s.Parameters()
			case ConicalSurface:
				cf, r, a := s.Parameters()
				frame, e = geom.NewFrame(cf.Origin(), cf.XAxis(), cf.ZAxis().Scale(-1))
				if e != nil {
					return fail()
				}
				radius = r
				slope = math.Tan(a)
				sign = -1
				conical = true
			}
			side, uses, e = analyticFaceLoop(f, id, len(faces)+1)
			if e != nil {
				return fail()
			}
			sense, e := boolean(side.Args[3])
			if e != nil || !sense {
				return fail()
			}
		case geom.PlaneSurface:
			caps = append(caps, id)
		default:
			return fail()
		}
	}
	if sid == 0 || len(caps) != len(faces)-1 || (!conical && len(faces) != 3) {
		return fail()
	}
	near := func(a, b geom.Vec3, scale float64) bool { return a.Sub(b).Scale(1/scale).Length() <= 1e-10 }
	type rim struct {
		edge, vertex, plane int64
		z                   float64
	}
	rims := []rim{}
	seam := int64(0)
	for _, u := range uses {
		start, end, g, e := analyticEdge(f, u.id)
		if e != nil {
			return fail()
		}
		ent := f.Entities[g]
		if len(ent.Records) != 1 {
			return fail()
		}
		kind := ent.Records[0].Type
		if kind == "SEAM_CURVE" {
			if seam != 0 && seam != u.id {
				return fail()
			}
			seam = u.id
			continue
		}
		if kind != "SURFACE_CURVE" || start != end {
			return fail()
		}
		curve, e := simple(f, g, kind, 4)
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
		rad, e := numeric(circle.Args[2])
		if e != nil || rad <= 0 {
			return fail()
		}
		pid, e := ref(circle.Args[1])
		if e != nil {
			return fail()
		}
		cf, e := placement(f, pid)
		if e != nil || cf.ZAxis().Distance(frame.ZAxis()) > 1e-10 || cf.XAxis().Distance(frame.XAxis()) > 1e-10 {
			return fail()
		}
		local := frame.ToLocal(cf.Origin())
		if !local.IsFinite() || math.Hypot(local.X, local.Y)/radius > 1e-10 || local.Z < 0 {
			return fail()
		}
		z := local.Z
		if math.Abs(rad-(radius-z*slope))/radius > 1e-10 {
			return fail()
		}
		pcurves, e := listRefs(curve.Args[2], 2)
		if e != nil || len(pcurves) != 2 {
			return fail()
		}
		plane := int64(0)
		sideFound := false
		for _, p := range pcurves {
			pr, e := simple(f, p, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			ps, e := ref(pr.Args[1])
			if e != nil {
				return fail()
			}
			if ps == sid {
				if sideFound {
					return fail()
				}
				sideFound = true
				offset, e := signedPCurve(f, p, sid, 0, sign)
				if e != nil || math.Abs(offset-z)/math.Max(radius, math.Abs(z)) > 1e-10 {
					return fail()
				}
			} else {
				if plane != 0 {
					return fail()
				}
				plane = ps
				pf, e := planeFrame(f, ps)
				if e != nil || !near(pf.Origin(), cf.Origin(), radius) || pf.ZAxis().Distance(frame.ZAxis()) > 1e-10 || pf.XAxis().Distance(frame.XAxis()) > 1e-10 {
					return fail()
				}
				if e = capPCurve(f, p, ps, rad); e != nil {
					return fail()
				}
			}
		}
		if !sideFound || plane == 0 {
			return fail()
		}
		point, e := analyticVertex(f, start)
		want := cf.Origin().Add(frame.XAxis().Scale(rad))
		if e != nil || !near(point, want, math.Max(radius, math.Abs(z))) {
			return fail()
		}
		rims = append(rims, rim{u.id, start, plane, z})
	}
	if len(rims) != len(faces)-1 || seam == 0 {
		return fail()
	}
	if len(rims) == 2 && rims[0].z > rims[1].z {
		rims[0], rims[1] = rims[1], rims[0]
	}
	bottom := rims[0]
	var top rim
	if len(rims) == 2 {
		top = rims[1]
	} else {
		start, end, _, e := analyticEdge(f, seam)
		if e != nil || start != bottom.vertex {
			return fail()
		}
		p, e := analyticVertex(f, end)
		if e != nil {
			return fail()
		}
		local := frame.ToLocal(p)
		if !local.IsFinite() || math.Hypot(local.X, local.Y)/radius > 1e-10 || math.Abs(radius-local.Z*slope)/radius > 1e-10 {
			return fail()
		}
		top = rim{vertex: end, z: local.Z}
	}
	height := top.z
	if bottom.z != 0 || height <= 0 || !finiteNumber(height) || bottom.vertex == top.vertex || (len(rims) == 2 && (bottom.plane == top.plane || bottom.edge == top.edge)) {
		return fail()
	}
	pattern := false
	for i := range uses {
		if len(uses) == 4 {
			a, b, c, d := uses[i], uses[(i+1)%4], uses[(i+2)%4], uses[(i+3)%4]
			if a.id == top.edge && !a.sense && b.id == seam && !b.sense && c.id == bottom.edge && c.sense && d.id == seam && d.sense {
				pattern = true
			}
		} else {
			a, b, c := uses[i], uses[(i+1)%3], uses[(i+2)%3]
			if a.id == seam && !a.sense && b.id == bottom.edge && b.sense && c.id == seam && c.sense {
				pattern = true
			}
		}
	}
	if !pattern {
		return fail()
	}
	start, end, g, e := analyticEdge(f, seam)
	if e != nil || start != bottom.vertex || end != top.vertex {
		return fail()
	}
	curve, e := simple(f, g, "SEAM_CURVE", 4)
	if e != nil {
		return fail()
	}
	if _, e = textValue(curve.Args[0]); e != nil {
		return fail()
	}
	if curve.Args[3].Kind != Enumeration || (curve.Args[3].Text != "PCURVE_S1" && curve.Args[3].Text != "PCURVE_S2") {
		return fail()
	}
	lineID, e := ref(curve.Args[1])
	if e != nil {
		return fail()
	}
	lineRecord, er := simple(f, lineID, "LINE", 3)
	if er != nil {
		return fail()
	}
	vectorID, er := ref(lineRecord.Args[2])
	if er != nil {
		return fail()
	}
	vector, er := simple(f, vectorID, "VECTOR", 3)
	if er != nil {
		return fail()
	}
	magnitude, er := numeric(vector.Args[2])
	if er != nil || math.Abs(magnitude-1) > 1e-12 {
		return fail()
	}
	expectedDirection, er := frame.ZAxis().Sub(frame.XAxis().Scale(slope)).Unit()
	if er != nil {
		return fail()
	}
	origin, direction, e := lineGeometry(f, lineID, 3)
	point, e2 := analyticVertex(f, bottom.vertex)
	if e != nil || e2 != nil || !near(origin, point, math.Max(radius, height)) || direction.Distance(expectedDirection) > 1e-10 {
		return fail()
	}
	pcs, e := listRefs(curve.Args[2], 2)
	if e != nil || len(pcs) != 2 {
		return fail()
	}
	a, e := signedPCurve(f, pcs[0], sid, 1, sign)
	if e != nil {
		return fail()
	}
	b, e := signedPCurve(f, pcs[1], sid, 1, sign)
	if e != nil || !((math.Abs(a) < 1e-10 && math.Abs(b-2*math.Pi) < 1e-10) || (math.Abs(b) < 1e-10 && math.Abs(a-2*math.Pi) < 1e-10)) {
		return fail()
	}
	matched := map[int64]bool{}
	for _, cap := range caps {
		face, u, e := analyticFaceLoop(f, cap, 1)
		if e != nil {
			return fail()
		}
		plane, e := ref(face.Args[2])
		if e != nil || matched[plane] {
			return fail()
		}
		matched[plane] = true
		sense, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		if plane == top.plane {
			if !sense || u[0].id != top.edge || !u[0].sense {
				return fail()
			}
		} else if plane == bottom.plane {
			if sense || u[0].id != bottom.edge || u[0].sense {
				return fail()
			}
		} else {
			return fail()
		}
	}
	var solid parametric.Solid
	if conical {
		topRadius := 0.0
		if len(rims) == 2 {
			topRadius = radius - height*slope
			if topRadius <= 0 {
				return fail()
			}
		}
		profile := []geom.Vec2{{}, {X: radius}, {X: topRadius, Y: height}}
		if topRadius > 0 {
			profile = append(profile, geom.Vec2{Y: height})
		}
		solid, e = parametric.Revolve(frame, profile)
	} else {
		solid, e = parametric.Cylinder(frame, radius, height)
	}
	if e != nil {
		return AnalyticBody{}, e
	}
	return AnalyticBody{name, solid}, nil
}
