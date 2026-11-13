package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func sphericalZoneBody(f *File, name string, faces []int64) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: unsupported spherical zone graph")
	}
	if len(faces) != 3 {
		return fail()
	}
	var sideID int64
	var caps []int64
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
			sideID = id
			frame, radius = s.Parameters()
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
	var seam int64
	var rims [2]int64
	found := false
	for i := range uses {
		a, b, c, d := uses[i], uses[(i+1)%4], uses[(i+2)%4], uses[(i+3)%4]
		if b.id == d.id && a.id != c.id && a.id != b.id && c.id != b.id && !a.sense && !b.sense && c.sense && d.sense {
			seam = b.id
			rims = [2]int64{c.id, a.id}
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
	var latitudes [2]float64
	usedCaps := map[int64]bool{}
	for k, rim := range rims {
		vertex, other, geometry, e := analyticEdge(f, rim)
		if e != nil || vertex != other || (k == 0 && vertex != start) || (k == 1 && vertex != end) {
			return fail()
		}
		var cap Record
		capFound := false
		for _, id := range caps {
			r, u, e := analyticFaceLoop(f, id, 1)
			if e != nil {
				return fail()
			}
			if u[0].id != rim {
				continue
			}
			if capFound || usedCaps[id] || u[0].sense != (k == 1) {
				return fail()
			}
			sense, e := boolean(r.Args[3])
			if e != nil || sense != (k == 1) {
				return fail()
			}
			cap = r
			capFound = true
			usedCaps[id] = true
		}
		if !capFound {
			return fail()
		}
		sc, e := simple(f, geometry, "SURFACE_CURVE", 4)
		if e != nil {
			return fail()
		}
		cid, e := ref(sc.Args[1])
		if e != nil {
			return fail()
		}
		circle, e := simple(f, cid, "CIRCLE", 3)
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
		z := frame.ToLocal(cf.Origin()).Z / radius
		if math.Abs(z) >= 1 {
			return fail()
		}
		latitudes[k] = math.Asin(z)
		// Extend each validated latitude rim to a virtual north-pole cap. The cap
		// proof checks the exact sphere/circle/plane loci and full-period pcurves;
		// original zone seam endpoints and both loop orientations were checked above.
		local := *f
		local.Entities = make(map[int64]Entity, len(f.Entities)+20)
		for id, ent := range f.Entities {
			local.Entities[id] = ent
		}
		next := int64(-1)
		add := func(kind string, args ...Value) int64 {
			id := next
			next--
			local.Entities[id] = Entity{ID: id, Records: []Record{{Type: kind, Args: args}}}
			return id
		}
		refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
		str := Value{Kind: String}
		truth := Value{Kind: Enumeration, Text: "T"}
		falsev := Value{Kind: Enumeration, Text: "F"}
		derived := Value{Kind: Derived}
		pole := frame.Origin().Add(frame.ZAxis().Scale(radius))
		point := add("CARTESIAN_POINT", str, Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: real(pole.X)}, {Kind: Number, Text: real(pole.Y)}, {Kind: Number, Text: real(pole.Z)}}})
		vp := add("VERTEX_POINT", str, refv(point))
		edge := add("EDGE_CURVE", str, refv(vertex), refv(vp), refv(seamGeometry), truth)
		use := func(edge int64, sense Value) Value {
			return refv(add("ORIENTED_EDGE", str, derived, derived, refv(edge), sense))
		}
		loop := add("EDGE_LOOP", str, Value{Kind: Aggregate, Items: []Value{use(edge, falsev), use(rim, truth), use(edge, truth)}})
		bound := add("FACE_BOUND", str, refv(loop), truth)
		newSide := add("ADVANCED_FACE", str, Value{Kind: Aggregate, Items: []Value{refv(bound)}}, side.Args[2], truth)
		loop = add("EDGE_LOOP", str, Value{Kind: Aggregate, Items: []Value{use(rim, falsev)}})
		bound = add("FACE_BOUND", str, refv(loop), truth)
		newCap := add("ADVANCED_FACE", str, Value{Kind: Aggregate, Items: []Value{refv(bound)}}, cap.Args[2], falsev)
		if _, e := sphericalCapBody(&local, name, []int64{newSide, newCap}); e != nil {
			return AnalyticBody{}, e
		}
	}
	if len(usedCaps) != 2 || latitudes[0] >= latitudes[1] {
		return fail()
	}
	solid, e := parametric.SphereZone(frame, radius, latitudes[0], latitudes[1], radius*1e-10)
	return AnalyticBody{name, solid}, e
}
