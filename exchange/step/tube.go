package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func tubeBody(f *File, name string, faces []int64) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: unsupported full annular cylinder graph")
	}
	if len(faces) != 4 {
		return fail()
	}
	var sideIDs, capIDs []int64
	var frames []geom.Frame
	var radii, slopes []float64
	conicalCount := 0
	for _, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		s, e := DecodeAnalyticSurface(f, sid)
		if e != nil {
			return fail()
		}
		switch v := s.(type) {
		case geom.CylinderSurface:
			frame, r := v.Parameters()
			sideIDs = append(sideIDs, id)
			frames = append(frames, frame)
			radii = append(radii, r)
			slopes = append(slopes, 0)
		case ConicalSurface:
			cf, r, a := v.Parameters()
			frame, err := geom.NewFrame(cf.Origin(), cf.XAxis(), cf.ZAxis().Scale(-1))
			if err != nil {
				return fail()
			}
			sideIDs = append(sideIDs, id)
			frames = append(frames, frame)
			radii = append(radii, r)
			slopes = append(slopes, math.Tan(a))
			conicalCount++
		case geom.PlaneSurface:
			capIDs = append(capIDs, id)
		default:
			return fail()
		}
	}
	if len(sideIDs) != 2 || len(capIDs) != 2 || (conicalCount != 0 && conicalCount != 2) {
		return fail()
	}
	if radii[0] < radii[1] {
		sideIDs[0], sideIDs[1] = sideIDs[1], sideIDs[0]
		frames[0], frames[1] = frames[1], frames[0]
		radii[0], radii[1] = radii[1], radii[0]
		slopes[0], slopes[1] = slopes[1], slopes[0]
	}
	if radii[1] <= 0 || radii[0] <= radii[1] || frames[0].Origin().Distance(frames[1].Origin()) > 1e-10*radii[0] || frames[0].XAxis().Distance(frames[1].XAxis()) > 1e-10 || frames[0].ZAxis().Distance(frames[1].ZAxis()) > 1e-10 {
		return fail()
	}
	var heights [2]float64
	usedBounds := map[int64]bool{}
	for which, sideID := range sideIDs {
		local := *f
		local.Entities = make(map[int64]Entity, len(f.Entities)+20)
		for id, ent := range f.Entities {
			local.Entities[id] = ent
		}
		next := int64(-1)
		add := func(r Record) int64 {
			id := next
			next--
			local.Entities[id] = Entity{ID: id, Records: []Record{r}}
			return id
		}
		clone := func(r Record) Record { return Record{Type: r.Type, Args: append([]Value(nil), r.Args...)} }
		side, _ := simple(f, sideID, "ADVANCED_FACE", 4)
		sense, e := boolean(side.Args[3])
		if e != nil || sense != (which == 0) {
			return fail()
		}
		bounds, e := listRefs(side.Args[1], 1)
		if e != nil || len(bounds) != 1 {
			return fail()
		}
		bound, e := simple(f, bounds[0], "FACE_BOUND", 3)
		if e != nil {
			return fail()
		}
		orientation, e := boolean(bound.Args[2])
		if e != nil || orientation != (which == 0) {
			return fail()
		}
		// Normalize inward wall to an outward virtual cylinder, preserving its exact
		// loop/seam/pcurve graph for the existing complete-cylinder proof.
		nb := clone(bound)
		nb.Args[2] = Value{Kind: Enumeration, Text: "T"}
		bid := add(nb)
		ns := clone(side)
		ns.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: bid}}}
		ns.Args[3] = Value{Kind: Enumeration, Text: "T"}
		sid := add(ns)
		_, uses, e := analyticFaceLoop(&local, sid, 4)
		if e != nil {
			return fail()
		}
		edgeIDs := map[int64]bool{}
		for _, u := range uses {
			edgeIDs[u.id] = true
		}
		virtual := []int64{sid}
		for _, capID := range capIDs {
			cap, _ := simple(f, capID, "ADVANCED_FACE", 4)
			bs, e := listRefs(cap.Args[1], 2)
			if e != nil || len(bs) != 2 {
				return fail()
			}
			found := int64(0)
			for _, id := range bs {
				b, e := simple(f, id, "FACE_BOUND", 3)
				if e != nil {
					return fail()
				}
				o, e := boolean(b.Args[2])
				if e != nil {
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
				refs, e := listRefs(loop.Args[1], 1)
				if e != nil || len(refs) != 1 {
					return fail()
				}
				u, e := simple(f, refs[0], "ORIENTED_EDGE", 5)
				if e != nil || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived {
					return fail()
				}
				eid, e := ref(u.Args[3])
				if e != nil {
					return fail()
				}
				if !edgeIDs[eid] {
					continue
				}
				if found != 0 || usedBounds[id] {
					return fail()
				}
				usedBounds[id] = true
				us, e := boolean(u.Args[4])
				if e != nil {
					return fail()
				}
				effective := us == o
				if which == 1 {
					effective = !effective
				}
				nu := clone(u)
				nu.Args[4] = Value{Kind: Enumeration, Text: "F"}
				if effective {
					nu.Args[4].Text = "T"
				}
				uid := add(nu)
				nl := clone(loop)
				nl.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: uid}}}
				newLoop := add(nl)
				nb := clone(b)
				nb.Args[1] = Value{Kind: Reference, Ref: newLoop}
				nb.Args[2] = Value{Kind: Enumeration, Text: "T"}
				found = add(nb)
			}
			if found == 0 {
				return fail()
			}
			nc := clone(cap)
			nc.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: found}}}
			virtual = append(virtual, add(nc))
		}
		body, e := cylinderBody(&local, name, virtual)
		if e != nil {
			return AnalyticBody{}, e
		}
		for _, v := range body.Solid.Vertices() {
			heights[which] = math.Max(heights[which], frames[which].ToLocal(v).Z)
		}
	}
	if len(usedBounds) != 4 || math.Abs(heights[0]-heights[1]) > 1e-10*heights[0] {
		return fail()
	}
	if conicalCount == 2 {
		outerTop, innerTop := radii[0]-heights[0]*slopes[0], radii[1]-heights[0]*slopes[1]
		solid, e := hollowConicalSolid(frames[0], radii[0], outerTop, radii[1], innerTop, heights[0])
		return AnalyticBody{name, solid}, e
	}
	solid, e := parametric.Tube(frames[0], radii[0], heights[0], radii[0]-radii[1])
	return AnalyticBody{name, solid}, e
}
