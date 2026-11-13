package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func axialProfileBody(f *File, name string, faces []int64) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: unsupported stepped cylinder graph")
	}
	if len(faces) < 7 || len(faces) > 129 || len(faces)%2 != 1 {
		return fail()
	}
	var sideIDs, capIDs []int64
	var frames []geom.Frame
	var radii, tops []float64
	var cones []bool
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
			frame, _, e = axialWallFrame(f, id, frame, r)
			if e != nil {
				return fail()
			}
			sideIDs = append(sideIDs, id)
			frames = append(frames, frame)
			radii = append(radii, r)
			tops = append(tops, r)
			cones = append(cones, false)
		case ConicalSurface:
			cf, r, a := v.Parameters()
			frame, err := geom.NewFrame(cf.Origin(), cf.XAxis(), cf.ZAxis().Scale(-1))
			if err != nil {
				return fail()
			}
			canonical, offset, err := axialWallFrame(f, id, frame, r)
			if err != nil || math.Abs(offset) > 1e-10*r {
				return fail()
			}
			_, uses, err := analyticFaceLoop(f, id, 4)
			if err != nil {
				return fail()
			}
			h := 0.
			for _, u := range uses {
				start, _, _, err := analyticEdge(f, u.id)
				if err != nil {
					return fail()
				}
				p, err := analyticVertex(f, start)
				if err != nil {
					return fail()
				}
				h = math.Max(h, frame.ToLocal(p).Z)
			}
			top := r - h*math.Tan(a)
			if !finiteNumber(top) || top <= 0 {
				return fail()
			}
			sideIDs = append(sideIDs, id)
			frames = append(frames, canonical)
			radii = append(radii, r)
			tops = append(tops, top)
			cones = append(cones, true)
		case geom.PlaneSurface:
			capIDs = append(capIDs, id)
		default:
			return fail()
		}
	}
	if len(sideIDs) < 3 || len(capIDs) != len(sideIDs)+1 {
		return fail()
	}
	shoulders := 0
	totalBounds := 0
	for _, id := range capIDs {
		r, _ := simple(f, id, "ADVANCED_FACE", 4)
		bounds, e := listRefs(r.Args[1], 1)
		if e != nil {
			return fail()
		}
		totalBounds += len(bounds)
		if len(bounds) == 2 {
			shoulders++
		}
	}
	if shoulders != len(sideIDs)-1 || totalBounds != 2*len(sideIDs) {
		return fail()
	}
	axial := make([]float64, len(frames))
	for i, fr := range frames {
		axial[i] = fr.Origin().Sub(frames[0].Origin()).Dot(frames[0].ZAxis())
		if !finiteNumber(axial[i]) {
			return fail()
		}
	}
	for i := range radii {
		for j := i + 1; j < len(radii); j++ {
			if axial[j] < axial[i] {
				axial[i], axial[j] = axial[j], axial[i]
				radii[i], radii[j] = radii[j], radii[i]
				tops[i], tops[j] = tops[j], tops[i]
				cones[i], cones[j] = cones[j], cones[i]
				frames[i], frames[j] = frames[j], frames[i]
				sideIDs[i], sideIDs[j] = sideIDs[j], sideIDs[i]
			}
		}
	}
	for i := range radii {
		if radii[i] <= 0 || (i > 0 && radii[i] == tops[i-1]) || frames[0].XAxis().Distance(frames[i].XAxis()) > 1e-10 || frames[0].ZAxis().Distance(frames[i].ZAxis()) > 1e-10 {
			return fail()
		}
	}
	heights := make([]float64, len(radii))

	usedBounds := map[int64]bool{}
	for which, sideID := range sideIDs {
		normalized := f
		var err error
		if !cones[which] {
			normalized, err = normalizeAxialWall(f, sideID)
		}
		if err != nil {
			return AnalyticBody{}, err
		}
		local := *normalized
		local.Entities = make(map[int64]Entity, len(f.Entities)+20)
		for id, ent := range normalized.Entities {
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
		if e != nil || !sense {
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
		if e != nil || !orientation {
			return fail()
		}
		// Validate each complete wall using temporary individual disk caps.
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
		bottomEdge := int64(0)
		for _, u := range uses {
			a, b, _, err := analyticEdge(f, u.id)
			if err != nil {
				return fail()
			}
			if a == b && u.sense {
				if bottomEdge != 0 {
					return fail()
				}
				bottomEdge = u.id
			}
		}
		virtual := []int64{sid}
		for _, capID := range capIDs {
			cap, _ := simple(f, capID, "ADVANCED_FACE", 4)
			bs, e := listRefs(cap.Args[1], 1)
			if e != nil || (len(bs) != 1 && len(bs) != 2) {
				return fail()
			}
			found := int64(0)
			matchedBottom := false
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
				matchedBottom = eid == bottomEdge
				us, e := boolean(u.Args[4])
				if e != nil {
					return fail()
				}
				effective := us == o
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
				continue
			}
			nc := clone(cap)
			if len(bs) == 2 {
				expected := false
				if matchedBottom {
					if which == 0 {
						return fail()
					}
					expected = tops[which-1] > radii[which]
				} else {
					if which+1 >= len(radii) {
						return fail()
					}
					expected = tops[which] > radii[which+1]
				}
				sense, e := boolean(cap.Args[3])
				if e != nil || sense != expected {
					return fail()
				}
				nc.Args[3] = Value{Kind: Enumeration, Text: "T"}
				if matchedBottom {
					nc.Args[3].Text = "F"
				}
			}

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

	if len(usedBounds) != 2*len(radii) {
		return fail()
	}
	for i := 1; i < len(radii); i++ {
		delta := frames[i-1].ToLocal(frames[i].Origin())
		tol := 1e-10 * math.Max(radii[0], heights[i-1]+heights[i])
		if math.Abs(delta.X) > tol || math.Abs(delta.Y) > tol || math.Abs(delta.Z-heights[i-1]) > tol {
			return fail()
		}
	}
	profile := []geom.Vec2{{}, {X: radii[0]}}
	height := 0.
	for i, r := range tops {
		height += heights[i]
		profile = append(profile, geom.Vec2{X: r, Y: height})
		if i+1 < len(radii) {
			profile = append(profile, geom.Vec2{X: radii[i+1], Y: height})
		}
	}
	profile = append(profile, geom.Vec2{Y: height})
	solid, e := parametric.Revolve(frames[0], profile)
	return AnalyticBody{name, solid}, e
}
