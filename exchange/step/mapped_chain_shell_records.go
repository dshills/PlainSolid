package step

import (
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type mappedChainStepFace struct {
	id, supportID int64
	surface       *geom.BSplineSurface
	reversed      bool
	uses          []analyticUse
	plane         *mappedChainForeignPlane
}

func parseMappedChainStepFace(f *File, id int64, foreign bool) (mappedChainStepFace, error) {
	var zero mappedChainStepFace
	face, err := simple(f, id, "ADVANCED_FACE", 4)
	if err != nil || face.Args[0].Kind != String {
		return zero, geom.ErrInvalid
	}
	forward, err := boolean(face.Args[3])
	if err != nil {
		return zero, err
	}
	sid, err := ref(face.Args[2])
	if err != nil {
		return zero, err
	}
	bounds, err := listRefs(face.Args[1], 1)
	if err != nil || len(bounds) != 1 {
		return zero, geom.ErrInvalid
	}
	kind := "FACE_OUTER_BOUND"
	if foreign {
		if _, ok := component(f.Entities[bounds[0]], "FACE_BOUND"); ok {
			kind = "FACE_BOUND"
		}
	}
	bound, err := simple(f, bounds[0], kind, 3)
	if err != nil || bound.Args[0].Kind != String {
		return zero, geom.ErrInvalid
	}
	sense, err := boolean(bound.Args[2])
	if err != nil || sense != forward {
		return zero, geom.ErrInvalid
	}
	lid, err := ref(bound.Args[1])
	if err != nil {
		return zero, err
	}
	loop, err := simple(f, lid, "EDGE_LOOP", 2)
	if err != nil || loop.Args[0].Kind != String {
		return zero, geom.ErrInvalid
	}
	ids, err := listRefs(loop.Args[1], 3)
	if err != nil || len(ids) > 24 {
		return zero, geom.ErrInvalid
	}
	out := mappedChainStepFace{id: id, supportID: sid, reversed: !forward}
	seen := make(map[int64]bool)
	for _, use := range ids {
		u, err := simple(f, use, "ORIENTED_EDGE", 5)
		if err != nil || u.Args[0].Kind != String || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived {
			return zero, geom.ErrInvalid
		}
		eid, err := ref(u.Args[3])
		if err != nil || seen[eid] {
			return zero, geom.ErrInvalid
		}
		sense, err := boolean(u.Args[4])
		if err != nil {
			return zero, err
		}
		seen[eid] = true
		out.uses = append(out.uses, analyticUse{eid, sense})
	}
	_, plane := component(f.Entities[sid], "PLANE")
	if foreign && plane {
		out.surface, out.plane, err = readMappedChainForeignPlane(f, sid, out.uses)
	} else {
		out.surface, err = decodeMappedChainStepSurface(f, sid)
	}
	if err != nil {
		return zero, err
	}
	return out, nil
}

func decodeMappedChainStepSurface(f *File, id int64) (*geom.BSplineSurface, error) {
	s, err := DecodeBSplineSurface(f, id, 81)
	if err != nil || s.ClosedU != "F" || s.ClosedV != "F" || s.SelfIntersect != "F" {
		return nil, geom.ErrInvalid
	}
	data := s.Geometry.Data()
	if data.CountU != data.DegreeU+1 || data.CountV != data.DegreeV+1 {
		return nil, geom.ErrConvergence
	}
	k, _ := component(f.Entities[id], "B_SPLINE_SURFACE_WITH_KNOTS")
	var rows, ku, kv Value
	if len(k.Args) == 13 {
		rows, ku, kv = k.Args[3], k.Args[10], k.Args[11]
	} else {
		b, _ := component(f.Entities[id], "B_SPLINE_SURFACE")
		w, _ := component(f.Entities[id], "RATIONAL_B_SPLINE_SURFACE")
		rows, ku, kv = b.Args[2], k.Args[2], k.Args[3]
		for _, row := range w.Args[0].Items {
			for _, v := range row.Items {
				if _, err := wrappedIsoSourceNumber(v); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, knots := range []Value{ku, kv} {
		for _, v := range knots.Items {
			if _, err := wrappedIsoSourceNumber(v); err != nil {
				return nil, err
			}
		}
	}
	for _, row := range rows.Items {
		for _, p := range row.Items {
			x, err := mappedChainStepPoint(f, p.Ref, 3)
			if err != nil {
				return nil, err
			}
			if _, err := geom.RepresentedExactPoint([3]*big.Rat(x)); err != nil {
				return nil, err
			}
		}
	}
	return s.Geometry, nil
}

func mappedChainStepVertex(f *File, id int64) ([3]*big.Rat, error) {
	var zero [3]*big.Rat
	r, err := simple(f, id, "VERTEX_POINT", 2)
	if err != nil || r.Args[0].Kind != String {
		return zero, geom.ErrInvalid
	}
	p, err := ref(r.Args[1])
	if err != nil {
		return zero, err
	}
	x, err := mappedChainStepPoint(f, p, 3)
	if err != nil {
		return zero, err
	}
	return [3]*big.Rat(x), nil
}

func mappedChainStepPCurve(f *File, id int64) (support, curve int64, err error) {
	p, e := simple(f, id, "PCURVE", 3)
	if e != nil || p.Args[0].Kind != String {
		return 0, 0, geom.ErrInvalid
	}
	support, e = ref(p.Args[1])
	if e != nil {
		return 0, 0, e
	}
	rid, e := ref(p.Args[2])
	if e != nil {
		return 0, 0, e
	}
	r, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil || r.Args[0].Kind != String {
		return 0, 0, geom.ErrInvalid
	}
	curves, e := listRefs(r.Args[1], 1)
	if e != nil || len(curves) != 1 {
		return 0, 0, geom.ErrInvalid
	}
	cid, e := ref(r.Args[2])
	if e != nil {
		return 0, 0, e
	}
	context := f.Entities[cid]
	dim, has := component(context, "GEOMETRIC_REPRESENTATION_CONTEXT")
	par, parameter := component(context, "PARAMETRIC_REPRESENTATION_CONTEXT")
	rep, representation := component(context, "REPRESENTATION_CONTEXT")
	if !has || !parameter || !representation || len(context.Records) != 3 ||
		len(dim.Args) != 1 || len(par.Args) != 0 || len(rep.Args) != 2 ||
		rep.Args[0].Kind != String || rep.Args[1].Kind != String {
		return 0, 0, geom.ErrInvalid
	}
	n, err := integer(dim.Args[0])
	if err != nil || n != 2 {
		return 0, 0, geom.ErrInvalid
	}
	return support, curves[0], nil
}

func mappedChainStepRawUse(f *File, id int64) (int64, trimmed.UVInterval, bool, error) {
	var empty trimmed.UVInterval
	r, err := simple(f, id, "TRIMMED_CURVE", 6)
	if err != nil || r.Args[0].Kind != String || r.Args[5].Kind != Enumeration || r.Args[5].Text != "PARAMETER" {
		return 0, empty, false, geom.ErrInvalid
	}
	basis, err := ref(r.Args[1])
	if err != nil {
		return 0, empty, false, err
	}
	forward, err := boolean(r.Args[4])
	if err != nil {
		return 0, empty, false, err
	}
	var limits [2]float64
	for i, v := range r.Args[2:4] {
		if v.Kind != Aggregate || len(v.Items) != 1 {
			return 0, empty, false, geom.ErrInvalid
		}
		q := v.Items[0]
		if q.Kind != Typed || q.Text != "PARAMETER_VALUE" || len(q.Items) != 1 {
			return 0, empty, false, geom.ErrInvalid
		}
		limits[i], err = wrappedIsoSourceNumber(q.Items[0])
		if err != nil {
			return 0, empty, false, err
		}
	}
	if limits[0] >= limits[1] {
		return 0, empty, false, geom.ErrInvalid
	}
	return basis, trimmed.UVInterval{Min: limits[0], Max: limits[1]}, !forward, nil
}
