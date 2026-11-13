package step

import (
	"errors"
	"math"
	"plainsolid/geom"
)

// axialWallFrame derives the lower trim level without changing the cylinder's
// axis. The full wall/rim proof is still performed after normalization.
func axialWallFrame(f *File, faceID int64, frame geom.Frame, radius float64) (geom.Frame, float64, error) {
	fail := func() (geom.Frame, float64, error) {
		return geom.Frame{}, 0, errors.New("step: invalid shifted cylinder trim")
	}
	_, uses, e := analyticFaceLoop(f, faceID, 4)
	if e != nil {
		return fail()
	}
	levels := []float64{}
	seen := map[int64]bool{}
	for _, u := range uses {
		if seen[u.id] {
			continue
		}
		seen[u.id] = true
		a, b, g, e := analyticEdge(f, u.id)
		if e != nil {
			return fail()
		}
		if a != b {
			continue
		}
		sc, e := simple(f, g, "SURFACE_CURVE", 4)
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
		local := frame.ToLocal(cf.Origin())
		if !local.IsFinite() || math.Hypot(local.X, local.Y) > 1e-10*radius {
			return fail()
		}
		levels = append(levels, local.Z)
	}
	if len(levels) != 2 || levels[0] == levels[1] {
		return fail()
	}
	offset := math.Min(levels[0], levels[1])
	canonical, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: offset}), frame.XAxis(), frame.ZAxis())
	return canonical, offset, e
}

// normalizeAxialWall performs only proved parameter-origin changes. It checks
// original seam model/pcurve phase before translating them together, preserving
// exact model loci. All remaining graph checks use the ordinary cylinder proof.
func normalizeAxialWall(f *File, faceID int64) (*File, error) {
	fail := func() (*File, error) { return nil, errors.New("step: inconsistent shifted cylinder parameter origins") }
	face, uses, e := analyticFaceLoop(f, faceID, 4)
	if e != nil {
		return fail()
	}
	sid, e := ref(face.Args[2])
	if e != nil {
		return fail()
	}
	surface, e := simple(f, sid, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	pid, e := ref(surface.Args[1])
	if e != nil {
		return fail()
	}
	frame, e := placement(f, pid)
	if e != nil {
		return fail()
	}
	radius, e := numeric(surface.Args[2])
	if e != nil || radius <= 0 {
		return fail()
	}
	canonical, offset, e := axialWallFrame(f, faceID, frame, radius)
	if e != nil {
		return fail()
	}
	local := *f
	local.Entities = make(map[int64]Entity, len(f.Entities)+32)
	next := int64(-1000)
	for id, ent := range f.Entities {
		local.Entities[id] = ent
		if id <= next {
			next = id - 1
		}
	}
	clone := func(r Record) Record { return Record{Type: r.Type, Args: append([]Value(nil), r.Args...)} }
	add := func(r Record) int64 {
		id := next
		next--
		local.Entities[id] = Entity{ID: id, Records: []Record{r}}
		return id
	}
	point3 := func(p geom.Vec3) int64 {
		return add(Record{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: real(p.X)}, {Kind: Number, Text: real(p.Y)}, {Kind: Number, Text: real(p.Z)}}}}})
	}
	placementRecord, _ := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
	placementRecord = clone(placementRecord)
	placementRecord.Args[1] = Value{Kind: Reference, Ref: point3(canonical.Origin())}
	surface = clone(surface)
	surface.Args[1] = Value{Kind: Reference, Ref: add(placementRecord)}
	local.Entities[sid] = Entity{ID: sid, Records: []Record{surface}}
	seen := map[int64]bool{}
	for _, use := range uses {
		if seen[use.id] {
			continue
		}
		seen[use.id] = true
		start, end, g, e := analyticEdge(f, use.id)
		if e != nil {
			return fail()
		}
		kind := "SURFACE_CURVE"
		isSeam := start != end
		if isSeam {
			kind = "SEAM_CURVE"
		}
		curve, e := simple(f, g, kind, 4)
		if e != nil {
			return fail()
		}
		seamZ := 0.
		if isSeam {
			lineID, e := ref(curve.Args[1])
			if e != nil {
				return fail()
			}
			origin, direction, e := lineGeometry(f, lineID, 3)
			if e != nil || direction.Distance(frame.ZAxis()) > 1e-10 {
				return fail()
			}
			originLocal := frame.ToLocal(origin)
			if !originLocal.IsFinite() || math.Abs(originLocal.X-radius) > radius*1e-10 || math.Abs(originLocal.Y) > radius*1e-10 {
				return fail()
			}
			seamZ = originLocal.Z
			startPoint, e := analyticVertex(f, start)
			if e != nil {
				return fail()
			}
			line, e := simple(f, lineID, "LINE", 3)
			if e != nil {
				return fail()
			}
			line = clone(line)
			line.Args[1] = Value{Kind: Reference, Ref: point3(startPoint)}
			local.Entities[lineID] = Entity{ID: lineID, Records: []Record{line}}
		}
		pcs, e := listRefs(curve.Args[2], 2)
		if e != nil || len(pcs) != 2 {
			return fail()
		}
		for _, pcID := range pcs {
			pc, e := simple(f, pcID, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			support, e := ref(pc.Args[1])
			if e != nil {
				return fail()
			}
			if support != sid {
				continue
			}
			rid, e := ref(pc.Args[2])
			if e != nil {
				return fail()
			}
			rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
			if e != nil {
				return fail()
			}
			items, e := listRefs(rep.Args[1], 1)
			if e != nil || len(items) != 1 {
				return fail()
			}
			line, e := simple(f, items[0], "LINE", 3)
			if e != nil {
				return fail()
			}
			pointID, e := ref(line.Args[1])
			if e != nil {
				return fail()
			}
			p, e := coordinate(f, pointID, "CARTESIAN_POINT", 2)
			if e != nil {
				return fail()
			}
			v := p.Y - offset
			if isSeam {
				if math.Abs(p.Y-seamZ) > 1e-10*math.Max(radius, math.Abs(seamZ)) {
					return fail()
				}
				v = p.Y - seamZ
			}
			if !finiteNumber(v) {
				return fail()
			}
			newPoint := add(Record{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: real(p.X)}, {Kind: Number, Text: real(v)}}}}})
			line = clone(line)
			line.Args[1] = Value{Kind: Reference, Ref: newPoint}
			rep = clone(rep)
			rep.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: add(line)}}}
			pc = clone(pc)
			pc.Args[2] = Value{Kind: Reference, Ref: add(rep)}
			local.Entities[pcID] = Entity{ID: pcID, Records: []Record{pc}}
		}
	}
	return &local, nil
}
