package step

import "plainsolid/geom"

func mappedPythagoreanExactEdges(f *File, bodyID int64) error {
	b, e := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return e
	}
	shell, e := simple(f, b.Args[1].Ref, "CLOSED_SHELL", 2)
	if e != nil {
		return e
	}
	faces, e := listRefs(shell.Args[1], 1)
	if e != nil {
		return e
	}
	owners := map[int64]map[int64]bool{}
	for _, fid := range faces {
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return e
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return e
		}
		plane, e := simple(f, sid, "PLANE", 2)
		if e != nil {
			return e
		}
		if _, e = mappedRationalFrame(f, plane.Args[1].Ref); e != nil {
			return e
		}
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil {
			return e
		}
		for _, bid := range bounds {
			be := f.Entities[bid]
			if len(be.Records) != 1 || len(be.Records[0].Args) != 3 {
				return mappedPlanarFailure()
			}
			loop, e := simple(f, be.Records[0].Args[1].Ref, "EDGE_LOOP", 2)
			if e != nil {
				return e
			}
			uses, e := listRefs(loop.Args[1], 1)
			if e != nil {
				return e
			}
			for _, uid := range uses {
				u, e := simple(f, uid, "ORIENTED_EDGE", 5)
				if e != nil {
					return e
				}
				eid, e := ref(u.Args[3])
				if e != nil {
					return e
				}
				if owners[eid] == nil {
					owners[eid] = map[int64]bool{}
				}
				if owners[eid][sid] {
					return mappedPlanarFailure()
				}
				owners[eid][sid] = true
			}
		}
	}
	for eid, own := range owners {
		if len(own) != 2 {
			return mappedPlanarFailure()
		}
		edge, e := simple(f, eid, "EDGE_CURVE", 5)
		if e != nil {
			return e
		}
		var points [2]geom.Vec3
		for i := range points {
			v, e := simple(f, edge.Args[i+1].Ref, "VERTEX_POINT", 2)
			if e != nil {
				return e
			}
			points[i], e = mappedExactCoordinate(f, v.Args[1].Ref, "CARTESIAN_POINT", 3)
			if e != nil {
				return e
			}
		}
		s, e := simple(f, edge.Args[3].Ref, "SURFACE_CURVE", 4)
		if e != nil {
			return e
		}
		if e = mappedRationalLine(f, s.Args[1].Ref, 3, nil, points); e != nil {
			return e
		}
		pcs, e := listRefs(s.Args[2], 2)
		if e != nil || len(pcs) != 2 {
			return mappedPlanarFailure()
		}
		seen := map[int64]bool{}
		for _, id := range pcs {
			pc, e := simple(f, id, "PCURVE", 3)
			if e != nil {
				return e
			}
			sid := pc.Args[1].Ref
			if !own[sid] || seen[sid] {
				return mappedPlanarFailure()
			}
			seen[sid] = true
			plane, e := simple(f, sid, "PLANE", 2)
			if e != nil {
				return e
			}
			frame, e := mappedRationalFrame(f, plane.Args[1].Ref)
			if e != nil {
				return e
			}
			rep, e := simple(f, pc.Args[2].Ref, "DEFINITIONAL_REPRESENTATION", 3)
			if e != nil {
				return e
			}
			items, e := listRefs(rep.Args[1], 1)
			if e != nil || len(items) != 1 {
				return mappedPlanarFailure()
			}
			if e = mappedRationalLine(f, items[0], 2, &frame, points); e != nil {
				return e
			}
		}
	}
	return nil
}
