package step

import (
	"errors"
	"plainsolid/geom"
)

func decodeOrthogonalShellFace(f *File, fid int64, maxPoints, remaining int) (OrthogonalSplineFace, [][]geom.Vec2, []OrthogonalShellUse, error) {
	fail := func() (OrthogonalSplineFace, [][]geom.Vec2, []OrthogonalShellUse, error) {
		return OrthogonalSplineFace{}, nil, nil, errors.New("step: invalid orthogonal shell loops")
	}
	face, e := simple(f, fid, "ADVANCED_FACE", 4)
	if e != nil {
		return fail()
	}
	support, e := ref(face.Args[2])
	if e != nil {
		return fail()
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) > 1+MaxSplineFaceHoles {
		return fail()
	}
	result := OrthogonalSplineFace{}
	holes := [][]geom.Vec2{}
	allUses := []OrthogonalShellUse{}
	seen := map[int64]bool{}
	hasOuter := false
	for _, bid := range bounds {
		if seen[bid] {
			return fail()
		}
		seen[bid] = true
		view, e := splinePolygonBoundView(f, fid, bid, false, MaxOrthogonalSplineVertices)
		if e != nil {
			return fail()
		}
		n, e := orthogonalUseCount(view, fid)
		if e != nil || n > remaining {
			return fail()
		}
		remaining -= n
		_, uses, e := analyticFaceLoop(view, fid, n)
		if e != nil {
			return fail()
		}
		physical := make([]OrthogonalShellUse, len(uses))
		for i, use := range uses {
			start, end, g, e := analyticEdge(view, use.id)
			if e != nil {
				return fail()
			}
			physical[i] = OrthogonalShellUse{EdgeID: use.id, StartVertexID: start, EndVertexID: end, Forward: use.sense}
			sc, e := simple(view, g, "SURFACE_CURVE", 4)
			if e != nil || sc.Args[3].Kind != Enumeration || (sc.Args[3].Text != "PCURVE_S1" && sc.Args[3].Text != "PCURVE_S2") {
				return fail()
			}
			pcs, e := listRefs(sc.Args[2], 1)
			if e != nil || len(pcs) > 2 {
				return fail()
			}
			selected := int64(0)
			owners := map[int64]bool{}
			for _, pcid := range pcs {
				pc, e := simple(view, pcid, "PCURVE", 3)
				if e != nil {
					return fail()
				}
				owner, e := ref(pc.Args[1])
				if e != nil || owners[owner] {
					return fail()
				}
				owners[owner] = true
				if owner == support {
					selected = pcid
				}
			}
			if selected == 0 {
				return fail()
			}
			sc.Args = append([]Value(nil), sc.Args...)
			sc.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: selected}}}
			sc.Args[3] = Value{Kind: Enumeration, Text: "PCURVE_S1"}
			view.Entities[g] = Entity{ID: g, Records: []Record{sc}}
		}
		decoded, e := decodeOrthogonalSplineFace(view, fid, maxPoints)
		loop := 0
		if e == nil {
			if hasOuter {
				return fail()
			}
			hasOuter = true
			result = decoded
		} else {
			reverse, e := splinePolygonBoundView(view, fid, bid, true, MaxOrthogonalSplineVertices)
			if e != nil {
				return fail()
			}
			decoded, e = decodeOrthogonalSplineFace(reverse, fid, maxPoints)
			if e != nil {
				return fail()
			}
			holes = append(holes, decoded.Boundary)
			loop = len(holes)
		}
		for i := range physical {
			physical[i].Loop = loop
		}
		allUses = append(allUses, physical...)
	}
	if !hasOuter {
		return fail()
	}
	if len(holes) > 0 {
		if e := validateOrthogonalHoles(result.Boundary, holes); e != nil {
			return fail()
		}
	}
	return result, holes, allUses, nil
}
