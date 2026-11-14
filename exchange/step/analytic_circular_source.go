package step

import (
	"fmt"
	"math"
	"plainsolid/geom"
)

type circularSourceFace struct {
	id, surface int64
	kind        string
	frame       geom.Frame
	radius      float64
	sense       bool
	bounds      []int64
	loops       [][]analyticUse
}
type circularSourceEdge struct {
	id, a, b, geometry int64
	kind, wrapper      string
	frame              geom.Frame
	radius             float64
	origin, direction  geom.Vec3
	pcs                []int64
}

func circularSourceFailure() error {
	return fmt.Errorf("step: unsupported or inconsistent analytic circular extrusion")
}
func readCircularSourceFace(f *File, id int64) (circularSourceFace, error) {
	return readCircularSourceFaceBounded(f, id, 17, 64)
}
func readCircularSourceFaceBounded(f *File, id int64, maxBounds, maxUses int) (circularSourceFace, error) {
	out := circularSourceFace{id: id}
	r, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return out, e
	}
	out.surface, e = ref(r.Args[2])
	if e != nil {
		return out, e
	}
	out.sense, e = boolean(r.Args[3])
	if e != nil {
		return out, e
	}
	surface, e := DecodeAnalyticSurface(f, out.surface)
	if e != nil {
		return out, e
	}
	switch s := surface.(type) {
	case geom.PlaneSurface:
		out.kind = "PLANE"
		out.frame = s.Frame()
	case geom.CylinderSurface:
		out.kind = "CYLINDER"
		out.frame, out.radius = s.Parameters()
	default:
		return out, circularSourceFailure()
	}
	out.bounds, e = listRefs(r.Args[1], 1)
	if e != nil || len(out.bounds) > maxBounds {
		return out, circularSourceFailure()
	}
	for _, bid := range out.bounds {
		en := f.Entities[bid]
		if len(en.Records) != 1 || (en.Records[0].Type != "FACE_BOUND" && en.Records[0].Type != "FACE_OUTER_BOUND") {
			return out, circularSourceFailure()
		}
		b, e := simple(f, bid, en.Records[0].Type, 3)
		if e != nil {
			return out, e
		}
		forward, e := boolean(b.Args[2])
		if e != nil {
			return out, e
		}
		lid, e := ref(b.Args[1])
		if e != nil {
			return out, e
		}
		loop, e := simple(f, lid, "EDGE_LOOP", 2)
		if e != nil {
			return out, e
		}
		ids, e := listRefs(loop.Args[1], 1)
		if e != nil || len(ids) > maxUses {
			return out, circularSourceFailure()
		}
		uses := make([]analyticUse, len(ids))
		for i, uid := range ids {
			u, e := simple(f, uid, "ORIENTED_EDGE", 5)
			if e != nil || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived {
				return out, circularSourceFailure()
			}
			uses[i].id, e = ref(u.Args[3])
			if e != nil {
				return out, e
			}
			uses[i].sense, e = boolean(u.Args[4])
			if e != nil {
				return out, e
			}
		}
		if !forward {
			for i, j := 0, len(uses)-1; i < j; i, j = i+1, j-1 {
				uses[i], uses[j] = uses[j], uses[i]
			}
			for i := range uses {
				uses[i].sense = !uses[i].sense
			}
		}
		out.loops = append(out.loops, uses)
	}
	return out, nil
}
func readCircularSourceEdge(f *File, id int64) (circularSourceEdge, error) {
	return readCircularEdge(f, id, false)
}

func readCircularEdge(f *File, id int64, allowReversed bool) (circularSourceEdge, error) {
	out := circularSourceEdge{id: id}
	var e error
	out.a, out.b, out.geometry, e = analyticEdgeGeometry(f, id, allowReversed)
	if e != nil {
		return out, e
	}
	en := f.Entities[out.geometry]
	if len(en.Records) != 1 {
		return out, circularSourceFailure()
	}
	out.wrapper = en.Records[0].Type
	if out.wrapper != "SURFACE_CURVE" && out.wrapper != "SEAM_CURVE" {
		return out, circularSourceFailure()
	}
	r, e := simple(f, out.geometry, out.wrapper, 4)
	if e != nil {
		return out, e
	}
	if r.Args[3].Kind != Enumeration || (r.Args[3].Text != "CURVE_3D" && r.Args[3].Text != "PCURVE_S1" && r.Args[3].Text != "PCURVE_S2") {
		return out, circularSourceFailure()
	}
	out.pcs, e = listRefs(r.Args[2], 2)
	if e != nil || len(out.pcs) != 2 {
		return out, circularSourceFailure()
	}
	gid, e := ref(r.Args[1])
	if e != nil {
		return out, e
	}
	g := f.Entities[gid]
	if len(g.Records) != 1 {
		return out, circularSourceFailure()
	}
	out.kind = g.Records[0].Type
	switch out.kind {
	case "LINE":
		out.origin, out.direction, e = lineGeometry(f, gid, 3)
	case "CIRCLE":
		c, err := simple(f, gid, "CIRCLE", 3)
		if err != nil {
			return out, err
		}
		pid, err := ref(c.Args[1])
		if err != nil {
			return out, err
		}
		out.frame, e = placement(f, pid)
		if e == nil {
			out.radius, e = numeric(c.Args[2])
		}
		if out.radius <= 0 {
			return out, circularSourceFailure()
		}
	default:
		return out, circularSourceFailure()
	}
	return out, e
}
func circularPCurveGeometry(f *File, id int64) (int64, int64, error) {
	p, e := simple(f, id, "PCURVE", 3)
	if e != nil {
		return 0, 0, e
	}
	owner, e := ref(p.Args[1])
	if e != nil {
		return 0, 0, e
	}
	rid, e := ref(p.Args[2])
	if e != nil {
		return 0, 0, e
	}
	r, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil {
		return 0, 0, e
	}
	ids, e := listRefs(r.Args[1], 1)
	if e != nil || len(ids) != 1 {
		return 0, 0, circularSourceFailure()
	}
	ctx, e := ref(r.Args[2])
	if e != nil {
		return 0, 0, e
	}
	seen := map[string]bool{}
	for _, c := range f.Entities[ctx].Records {
		if seen[c.Type] {
			return 0, 0, circularSourceFailure()
		}
		seen[c.Type] = true
		switch c.Type {
		case "GEOMETRIC_REPRESENTATION_CONTEXT":
			if len(c.Args) != 1 {
				return 0, 0, circularSourceFailure()
			}
			n, e := integer(c.Args[0])
			if e != nil || n != 2 {
				return 0, 0, circularSourceFailure()
			}
		case "PARAMETRIC_REPRESENTATION_CONTEXT":
			if len(c.Args) != 0 {
				return 0, 0, circularSourceFailure()
			}
		case "REPRESENTATION_CONTEXT":
			if len(c.Args) != 2 || c.Args[0].Kind != String || c.Args[1].Kind != String {
				return 0, 0, circularSourceFailure()
			}
		default:
			return 0, 0, circularSourceFailure()
		}
	}
	if len(seen) != 3 {
		return 0, 0, circularSourceFailure()
	}
	return owner, ids[0], nil
}
func verifyCircularPCurve(f *File, pcid int64, face circularSourceFace, edge circularSourceEdge, tol float64) (float64, error) {
	fail := func() (float64, error) { return 0, circularSourceFailure() }
	owner, g, e := circularPCurveGeometry(f, pcid)
	if e != nil || owner != face.surface {
		return fail()
	}
	near := func(a, b geom.Vec3) bool { return a.Sub(b).Length() <= tol }
	if edge.kind == "LINE" {
		p, d, e := lineGeometry(f, g, 2)
		if e != nil {
			return fail()
		}
		var origin, direction geom.Vec3
		if face.kind == "PLANE" {
			origin = face.frame.ToWorld(p)
			direction = face.frame.XAxis().Scale(d.X).Add(face.frame.YAxis().Scale(d.Y))
		} else {
			if math.Abs(d.X) > 1e-10 || math.Abs(d.Y) != 1 {
				return fail()
			}
			origin = face.frame.ToWorld(geom.Vec3{X: face.radius * math.Cos(p.X), Y: face.radius * math.Sin(p.X), Z: p.Y})
			direction = face.frame.ZAxis().Scale(d.Y)
		}
		if !near(origin, edge.origin) || direction.Sub(edge.direction).Length() > 1e-9 {
			return fail()
		}
		return p.X, nil
	}
	var center, cosine, sine geom.Vec3
	if face.kind == "PLANE" {
		c, e := simple(f, g, "CIRCLE", 3)
		if e != nil {
			return fail()
		}
		r, e := numeric(c.Args[2])
		if e != nil || r <= 0 {
			return fail()
		}
		pid, e := ref(c.Args[1])
		if e != nil {
			return fail()
		}
		a, e := simple(f, pid, "AXIS2_PLACEMENT_2D", 3)
		if e != nil {
			return fail()
		}
		oid, e := ref(a.Args[1])
		if e != nil {
			return fail()
		}
		p, e := coordinate(f, oid, "CARTESIAN_POINT", 2)
		if e != nil {
			return fail()
		}
		did, e := ref(a.Args[2])
		if e != nil {
			return fail()
		}
		d, e := coordinate(f, did, "DIRECTION", 2)
		if e != nil {
			return fail()
		}
		d, e = d.Unit()
		if e != nil {
			return fail()
		}
		center = face.frame.ToWorld(p)
		cosine = face.frame.XAxis().Scale(r * d.X).Add(face.frame.YAxis().Scale(r * d.Y))
		sine = face.frame.XAxis().Scale(-r * d.Y).Add(face.frame.YAxis().Scale(r * d.X))
	} else {
		p, d, e := circularUnitLineGeometry(f, g, 2)
		if e != nil || math.Abs(d.Y) > 1e-10 || math.Abs(d.X) != 1 {
			return fail()
		}
		center = face.frame.ToWorld(geom.Vec3{Z: p.Y})
		cosine = face.frame.XAxis().Scale(face.radius * math.Cos(p.X)).Add(face.frame.YAxis().Scale(face.radius * math.Sin(p.X)))
		sine = face.frame.XAxis().Scale(-face.radius * d.X * math.Sin(p.X)).Add(face.frame.YAxis().Scale(face.radius * d.X * math.Cos(p.X)))
	}
	if !near(center, edge.frame.Origin()) || !near(cosine, edge.frame.XAxis().Scale(edge.radius)) || !near(sine, edge.frame.YAxis().Scale(edge.radius)) {
		return fail()
	}
	return 0, nil
}

// circularSourceGraph retains actual STEP identity, independently of coordinates.
type circularSourceGraph struct {
	faces  []circularSourceFace
	edges  map[int64]circularSourceEdge
	owners map[int64][]int
}

func readCircularSourceGraph(f *File, body int64) (circularSourceGraph, error) {
	return readCircularGraph(f, body, false)
}

func readCircularGraph(f *File, body int64, allowReversed bool) (circularSourceGraph, error) {
	return readCircularGraphBounded(f, body, allowReversed, 98, 17, 64)
}
func readCircularGraphBounded(f *File, body int64, allowReversed bool, maxFaces, maxBounds, maxUses int) (circularSourceGraph, error) {
	out := circularSourceGraph{edges: map[int64]circularSourceEdge{}, owners: map[int64][]int{}}
	b, e := simple(f, body, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return out, e
	}
	sid, e := ref(b.Args[1])
	if e != nil {
		return out, e
	}
	s, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return out, e
	}
	ids, e := listRefs(s.Args[1], 3)
	if e != nil || len(ids) > maxFaces {
		return out, circularSourceFailure()
	}
	owned := map[int64]bool{}
	surfaces := map[int64]int{}
	uses := map[int64][]bool{}
	for _, id := range ids {
		face, e := readCircularSourceFaceBounded(f, id, maxBounds, maxUses)
		if e != nil {
			return out, e
		}
		if _, ok := surfaces[face.surface]; ok {
			return out, circularSourceFailure()
		}
		surfaces[face.surface] = len(out.faces)
		for j, bid := range face.bounds {
			br := f.Entities[bid].Records[0]
			lid, e := ref(br.Args[1])
			if e != nil {
				return out, e
			}
			if owned[bid] || owned[lid] {
				return out, circularSourceFailure()
			}
			owned[bid], owned[lid] = true, true
			loop := face.loops[j]
			for k, u := range loop {
				edge, ok := out.edges[u.id]
				if !ok {
					edge, e = readCircularEdge(f, u.id, allowReversed)
					if e != nil {
						return out, e
					}
					out.edges[u.id] = edge
				}
				next, ok := out.edges[loop[(k+1)%len(loop)].id]
				if !ok {
					next, e = readCircularEdge(f, loop[(k+1)%len(loop)].id, allowReversed)
					if e != nil {
						return out, e
					}
					out.edges[next.id] = next
				}
				end := edge.b
				if !u.sense {
					end = edge.a
				}
				start := next.a
				if !loop[(k+1)%len(loop)].sense {
					start = next.b
				}
				if start != end {
					return out, circularSourceFailure()
				}
				out.owners[u.id] = append(out.owners[u.id], len(out.faces))
				uses[u.id] = append(uses[u.id], u.sense)
			}
		}
		out.faces = append(out.faces, face)
	}
	pcs := map[int64]bool{}
	for id, edge := range out.edges {
		owners := out.owners[id]
		if len(owners) != 2 || len(uses[id]) != 2 || uses[id][0] == uses[id][1] {
			return out, circularSourceFailure()
		}
		expected := map[int64]int{}
		for _, i := range owners {
			expected[out.faces[i].surface]++
		}
		for _, pc := range edge.pcs {
			if pcs[pc] {
				return out, circularSourceFailure()
			}
			pcs[pc] = true
			owner, _, e := circularPCurveGeometry(f, pc)
			if e != nil || expected[owner] == 0 {
				return out, circularSourceFailure()
			}
			expected[owner]--
		}
		if (owners[0] == owners[1]) != (edge.wrapper == "SEAM_CURVE") {
			return out, circularSourceFailure()
		}
	}
	return out, nil
}

// A cylindrical circle pcurve must preserve the angular parameter scale.
// Non-unit VECTOR magnitudes need explicit reparameterization and are rejected.
func circularUnitLineGeometry(f *File, id int64, dimension int) (geom.Vec3, geom.Vec3, error) {
	r, e := simple(f, id, "LINE", 3)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	vid, e := ref(r.Args[2])
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	v, e := simple(f, vid, "VECTOR", 3)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	n, e := numeric(v.Args[2])
	if e != nil || n != 1 {
		return geom.Vec3{}, geom.Vec3{}, circularSourceFailure()
	}
	return lineGeometry(f, id, dimension)
}
