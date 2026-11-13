package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type sectorUse struct {
	edge  int
	sense bool
}

var sectorEnds = [9][2]int{{1, 2}, {4, 5}, {1, 4}, {2, 5}, {0, 1}, {0, 2}, {3, 4}, {3, 5}, {0, 3}}
var sectorLoops = [][]sectorUse{
	{{1, false}, {2, false}, {0, true}, {3, true}},
	{{0, false}, {4, false}, {5, true}},
	{{1, true}, {7, false}, {6, true}},
	{{4, true}, {2, true}, {6, false}, {8, false}},
	{{7, true}, {3, false}, {5, false}, {8, true}},
}
var sectorSenses = [5]bool{true, false, true, true, false}

func sectorGeometry(frame geom.Frame, r, h, a float64) ([6]geom.Vec3, [5]geom.Frame) {
	return sectorFrustumGeometry(frame, r, r, h, a)
}
func sectorFrustumGeometry(frame geom.Frame, r, top, h, a float64) ([6]geom.Vec3, [5]geom.Frame) {
	radial := frame.XAxis().Scale(math.Cos(a)).Add(frame.YAxis().Scale(math.Sin(a)))
	p := [6]geom.Vec3{frame.Origin(), frame.ToWorld(geom.Vec3{X: r}), frame.Origin().Add(radial.Scale(r)), frame.ToWorld(geom.Vec3{Z: h}), frame.ToWorld(geom.Vec3{X: top, Z: h}), frame.ToWorld(geom.Vec3{Z: h}).Add(radial.Scale(top))}
	planes := [5]geom.Frame{frame, frame}
	planes[2], _ = geom.NewFrame(p[3], frame.XAxis(), frame.ZAxis())
	planes[3], _ = geom.NewFrame(p[0], frame.XAxis(), frame.YAxis().Scale(-1))
	planes[4], _ = geom.NewFrame(p[0], radial, radial.Cross(frame.ZAxis()))
	return p, planes
}
func sectorBody(f *File, name string, faces []int64) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: unsupported cylindrical/conical sector graph")
	}
	if len(faces) != 5 {
		return fail()
	}
	side := int64(0)
	var frame geom.Frame
	r, slope, sign := 0., 0., 1.
	conical := false
	for _, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		surface, e := DecodeAnalyticSurface(f, sid)
		if e != nil {
			return fail()
		}
		switch s := surface.(type) {
		case geom.CylinderSurface:
			if side != 0 {
				return fail()
			}
			side = id
			frame, r = s.Parameters()
		case ConicalSurface:
			if side != 0 {
				return fail()
			}
			side = id
			cf, rr, angle := s.Parameters()
			frame, e = geom.NewFrame(cf.Origin(), cf.XAxis(), cf.ZAxis().Scale(-1))
			if e != nil {
				return fail()
			}
			r = rr
			slope = math.Tan(angle)
			sign = -1
			conical = true
		case geom.PlaneSurface:
		default:
			return fail()
		}
	}
	if side == 0 {
		return fail()
	}
	_, uses, e := analyticFaceLoop(f, side, 4)
	if e != nil {
		return fail()
	}
	h, a := 0., 0.
	for _, u := range uses {
		start, end, _, e := analyticEdge(f, u.id)
		if e != nil {
			return fail()
		}
		for _, v := range []int64{start, end} {
			p, e := analyticVertex(f, v)
			if e != nil {
				return fail()
			}
			q := frame.ToLocal(p)
			if !q.IsFinite() {
				return fail()
			}
			h = math.Max(h, q.Z)
			angle := math.Atan2(q.Y, q.X)
			if angle < -1e-10 {
				angle += 2 * math.Pi
			}
			a = math.Max(a, angle)
		}
	}
	top := r - h*slope
	var solid parametric.Solid
	if conical {
		solid, e = parametric.ConicalSector(frame, r, top, h, a)
	} else {
		solid, e = parametric.CylinderSector(frame, r, h, a)
	}
	if e != nil {
		return fail()
	}
	points, planes := sectorFrustumGeometry(frame, r, top, h, a)
	tol := math.Max(r, h) * 1e-10
	near := func(x, y geom.Vec3) bool { return x.Distance(y) <= tol }
	vertexMap := map[int64]int{}
	vertexIDs := map[int]int64{}
	edgeIDs := map[int]int64{}
	edgeMap := map[int64]int{}
	owners := map[int64][]int64{}
	surfaceKinds := map[int64]int{}
	faceUses := make([][]analyticUse, 5)
	usedFaces := map[int]bool{}
	for _, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, _ := ref(face.Args[2])
		kind := 0
		if id != side {
			pf, e := planeFrame(f, sid)
			if e != nil {
				return fail()
			}
			kind = -1
			for j := 1; j < 5; j++ {
				if near(pf.Origin(), planes[j].Origin()) && pf.XAxis().Distance(planes[j].XAxis()) < 1e-10 && pf.ZAxis().Distance(planes[j].ZAxis()) < 1e-10 {
					if kind != -1 {
						return fail()
					}
					kind = j
				}
			}
			if kind < 0 {
				return fail()
			}
		}
		if usedFaces[kind] {
			return fail()
		}
		usedFaces[kind] = true
		surfaceKinds[sid] = kind
		sense, e := boolean(face.Args[3])
		if e != nil || sense != sectorSenses[kind] {
			return fail()
		}
		_, us, e := analyticFaceLoop(f, id, len(sectorLoops[kind]))
		if e != nil {
			return fail()
		}
		faceUses[kind] = us
		for _, u := range us {
			start, end, _, e := analyticEdge(f, u.id)
			if e != nil || start == end {
				return fail()
			}
			for _, v := range []int64{start, end} {
				if _, ok := vertexMap[v]; ok {
					continue
				}
				p, e := analyticVertex(f, v)
				if e != nil {
					return fail()
				}
				index := -1
				for j, q := range points {
					if near(p, q) {
						if index != -1 {
							return fail()
						}
						index = j
					}
				}
				if index < 0 {
					return fail()
				}
				if old, ok := vertexIDs[index]; ok && old != v {
					return fail()
				}
				vertexIDs[index] = v
				vertexMap[v] = index
			}
			index := -1
			for j, pair := range sectorEnds {
				if vertexMap[start] == pair[0] && vertexMap[end] == pair[1] {
					index = j
				}
			}
			if index < 0 {
				return fail()
			}
			if old, ok := edgeIDs[index]; ok && old != u.id {
				return fail()
			}
			edgeIDs[index] = u.id
			edgeMap[u.id] = index
			owners[u.id] = append(owners[u.id], sid)
		}
	}
	if len(vertexMap) != 6 || len(edgeIDs) != 9 || len(surfaceKinds) != 5 {
		return fail()
	}
	for kind, us := range faceUses {
		match := false
		for offset := range us {
			ok := true
			for j, want := range sectorLoops[kind] {
				got := us[(offset+j)%len(us)]
				if edgeMap[got.id] != want.edge || got.sense != want.sense {
					ok = false
				}
			}
			match = match || ok
		}
		if !match {
			return fail()
		}
	}
	for index, id := range edgeIDs {
		_, _, g, e := analyticEdge(f, id)
		if e != nil {
			return fail()
		}
		curve, e := simple(f, g, "SURFACE_CURVE", 4)
		if e != nil {
			return fail()
		}
		if _, e = textValue(curve.Args[0]); e != nil {
			return fail()
		}
		if curve.Args[3].Kind != Enumeration || (curve.Args[3].Text != "PCURVE_S1" && curve.Args[3].Text != "PCURVE_S2") {
			return fail()
		}
		geometry, e := ref(curve.Args[1])
		if e != nil {
			return fail()
		}
		var origin, direction geom.Vec3
		if index < 2 {
			circle, e := simple(f, geometry, "CIRCLE", 3)
			if e != nil {
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
			rr, e := numeric(circle.Args[2])
			level := index + 1
			expectedRadius := r
			if index == 1 {
				expectedRadius = top
			}
			if e != nil || math.Abs(rr-expectedRadius) > tol || !near(cf.Origin(), planes[level].Origin()) || cf.XAxis().Distance(frame.XAxis()) > 1e-10 || cf.ZAxis().Distance(frame.ZAxis()) > 1e-10 {
				return fail()
			}
		} else {
			origin, direction, e = sectorUnitLine(f, geometry, 3)
			if e != nil {
				return fail()
			}
			pair := sectorEnds[index]
			expected := points[pair[1]].Sub(points[pair[0]])
			expected = expected.Scale(1 / expected.Length())
			if !near(origin, points[pair[0]]) || direction.Distance(expected) > 1e-10 {
				return fail()
			}
		}
		pcs, e := listRefs(curve.Args[2], 2)
		if e != nil || len(pcs) != 2 || len(owners[id]) != 2 || owners[id][0] == owners[id][1] {
			return fail()
		}
		seen := map[int64]bool{}
		for _, pcid := range pcs {
			pc, e := simple(f, pcid, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			support, e := ref(pc.Args[1])
			if e != nil || seen[support] || (support != owners[id][0] && support != owners[id][1]) {
				return fail()
			}
			seen[support] = true
			kind := surfaceKinds[support]
			if kind == 0 {
				axis, expected := 0, 0.
				if index == 1 {
					expected = h
				}
				if index >= 2 {
					axis = 1
					if index == 3 {
						expected = a
					}
				}
				offset, e := signedPCurve(f, pcid, support, axis, sign)
				if e != nil || math.Abs(offset-expected) > 1e-10*math.Max(1, math.Abs(expected)) {
					return fail()
				}
			} else if index < 2 {
				rr := r
				if index == 1 {
					rr = top
				}
				if e := capPCurve(f, pcid, support, rr); e != nil {
					return fail()
				}
			} else {
				if e := sectorPlaneLinePC(f, pcid, support, planes[kind], origin, direction, tol); e != nil {
					return fail()
				}
			}
		}
	}
	return AnalyticBody{name, solid}, nil
}
func sectorPlaneLinePC(f *File, id, support int64, plane geom.Frame, origin, direction geom.Vec3, tol float64) error {
	fail := func() error { return errors.New("step: inconsistent sector planar pcurve") }
	pc, e := simple(f, id, "PCURVE", 3)
	if e != nil {
		return e
	}
	if _, e = textValue(pc.Args[0]); e != nil {
		return e
	}
	sid, e := ref(pc.Args[1])
	if e != nil || sid != support {
		return fail()
	}
	rid, e := ref(pc.Args[2])
	if e != nil {
		return e
	}
	rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil {
		return e
	}
	if _, e = textValue(rep.Args[0]); e != nil {
		return e
	}
	items, e := listRefs(rep.Args[1], 1)
	if e != nil || len(items) != 1 {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return e
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
	p, d, e := sectorUnitLine(f, items[0], 2)
	if e != nil {
		return e
	}
	mapped := plane.XAxis().Scale(d.X).Add(plane.YAxis().Scale(d.Y))
	if !p.IsFinite() || plane.ToWorld(p).Distance(origin) > tol || mapped.Distance(direction) > 1e-10 {
		return fail()
	}
	return nil
}

func sectorUnitLine(f *File, id int64, dimension int) (geom.Vec3, geom.Vec3, error) {
	p, d, e := lineGeometry(f, id, dimension)
	if e != nil {
		return p, d, e
	}
	r, _ := simple(f, id, "LINE", 3)
	vid, e := ref(r.Args[2])
	if e != nil {
		return p, d, e
	}
	v, e := simple(f, vid, "VECTOR", 3)
	if e != nil {
		return p, d, e
	}
	m, e := numeric(v.Args[2])
	if e != nil || math.Abs(m-1) > 1e-10 {
		return p, d, errors.New("step: nonunit sector line parameter scale")
	}
	return p, d, nil
}
