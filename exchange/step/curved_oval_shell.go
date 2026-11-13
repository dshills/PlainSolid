package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

// CurvedOvalOpenShell is a connected two-face sheet with four shared rim edges.
// Each independently charted face is certified injective and its model curves
// equal exact support composition. No general global shell embedding or material
// closure is asserted by this open-shell API.
type CurvedOvalOpenShell struct {
	Annulus, Disk     CurvedOvalFace
	LengthUnit        float64
	SourceSharedEdges [4]int64
}

func certifyCurvedOvalShell(m CurvedOvalOpenShell, o CurvedOvalOptions) error {
	if len(m.Annulus.Holes) != 1 || len(m.Disk.Holes) != 0 {
		return geom.ErrInvalid
	}
	_, a, e := certifyCurvedOvalHoles(m.Annulus, o)
	if e != nil {
		return e
	}
	_, b, e := certifyCurvedOvalHoles(m.Disk, o)
	if e != nil {
		return e
	}
	for i := 0; i < 4; i++ {
		same, e := geom.CompareExactCurvesContext(context.Background(), a[1][i], b[0][3-i], geom.ExactCurveComparisonOptions{ReverseB: true, MaxOperations: o.MaxOperations / 10, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return e
		}
		if !same.Complete || !same.Equal {
			return geom.ErrConvergence
		}
	}
	return nil
}
func curvedOvalFaceUses(f *File, id int64) ([]ovalShellUse, error) {
	return curvedOvalFaceUsesRange(f, id, 4, 4)
}
func curvedOvalFaceUsesRange(f *File, id int64, minimum, maximum int) ([]ovalShellUse, error) {
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return nil, e
	}
	sid, e := ref(face.Args[2])
	if e != nil {
		return nil, e
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) > 2 {
		return nil, geom.ErrInvalid
	}
	out := []ovalShellUse{}
	for _, bid := range bounds {
		bound, e := simple(f, bid, "FACE_OUTER_BOUND", 3)
		if e != nil {
			bound, e = simple(f, bid, "FACE_BOUND", 3)
		}
		if e != nil {
			return nil, e
		}
		lid, e := ref(bound.Args[1])
		if e != nil {
			return nil, e
		}
		loop, e := simple(f, lid, "EDGE_LOOP", 2)
		if e != nil {
			return nil, e
		}
		uses, e := listRefs(loop.Args[1], 1)
		if e != nil || len(uses) < minimum || len(uses) > maximum {
			return nil, geom.ErrInvalid
		}
		for _, useID := range uses {
			u, e := simple(f, useID, "ORIENTED_EDGE", 5)
			if e != nil {
				return nil, e
			}
			eid, e := ref(u.Args[3])
			if e != nil {
				return nil, e
			}
			sense, e := boolean(u.Args[4])
			if e != nil {
				return nil, e
			}
			edge, e := simple(f, eid, "EDGE_CURVE", 5)
			if e != nil {
				return nil, e
			}
			cid, e := ref(edge.Args[3])
			if e != nil {
				return nil, e
			}
			out = append(out, ovalShellUse{id, bid, lid, useID, eid, cid, sid, sense})
		}
	}
	return out, nil
}
func ReadCurvedOvalOpenShell(r io.Reader, l Limits, o CurvedOvalOptions) (CurvedOvalOpenShell, error) {
	fail := func() (CurvedOvalOpenShell, error) {
		return CurvedOvalOpenShell{}, fmt.Errorf("step: unsupported connected oval shell")
	}
	var e error
	o, e = o.defaults()
	if e != nil {
		return CurvedOvalOpenShell{}, e
	}
	o.MaxOperations /= 2
	if o.MaxOperations < 10 {
		return CurvedOvalOpenShell{}, geom.ErrConvergence
	}
	l, e = l.defaults()
	if e != nil {
		return CurvedOvalOpenShell{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return CurvedOvalOpenShell{}, e
	}
	var unit float64
	var faces []int64
	var sourceShell, sourceModel int64
	repCount := 0
	points := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, rec := range en.Records {
			if strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") && rec.Type != "MANIFOLD_SURFACE_SHAPE_REPRESENTATION" {
				return fail()
			}
			if rec.Type == "CARTESIAN_POINT" {
				points++
			}
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" {
				return fail()
			}
		}
		rep, e := simple(f, id, "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", 3)
		if e != nil {
			continue
		}
		repCount++
		ctx, _ := ref(rep.Args[2])
		unit, e = contextUnits(f, ctx)
		if e != nil {
			return fail()
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		models := 0
		for _, item := range items {
			if _, e := simple(f, item, "AXIS2_PLACEMENT_3D", 4); e == nil {
				if _, e = placement(f, item); e != nil {
					return fail()
				}
				continue
			}
			models++
			sourceModel = item
			m, e := simple(f, item, "SHELL_BASED_SURFACE_MODEL", 2)
			if e != nil {
				return fail()
			}
			ss, e := listRefs(m.Args[1], 1)
			if e != nil || len(ss) != 1 {
				return fail()
			}
			sourceShell = ss[0]
			shell, e := simple(f, ss[0], "OPEN_SHELL", 2)
			if e != nil {
				return fail()
			}
			faces, e = listRefs(shell.Args[1], 2)
			if e != nil || len(faces) != 2 || faces[0] == faces[1] {
				return fail()
			}
		}
		if models != 1 {
			return fail()
		}
	}
	if repCount != 1 || len(faces) != 2 || points > l.MaxVertices || l.MaxFaces < 2 || l.MaxEdgeUses < 12 {
		return fail()
	}
	all := map[int64][]ovalShellUse{}
	faceUses := map[int64][]ovalShellUse{}
	ownedBound := map[int64]int64{}
	ownedLoop := map[int64]int64{}
	ownedUse := map[int64]bool{}
	for _, fid := range faces {
		uses, e := curvedOvalFaceUses(f, fid)
		if e != nil {
			return fail()
		}
		faceUses[fid] = uses
		for _, u := range uses {
			if (ownedBound[u.bound] != 0 && ownedBound[u.bound] != fid) || (ownedLoop[u.loop] != 0 && ownedLoop[u.loop] != u.bound) || ownedUse[u.use] {
				return fail()
			}
			ownedBound[u.bound], ownedLoop[u.loop], ownedUse[u.use] = fid, u.bound, true
			all[u.edge] = append(all[u.edge], u)
		}
	}
	if len(all) != 8 || len(faceUses[faces[0]])+len(faceUses[faces[1]]) != 12 {
		return fail()
	}
	shared := map[int64]bool{}
	for eid, us := range all {
		if len(us) == 2 {
			if us[0].face == us[1].face || us[0].sense == us[1].sense || us[0].support == us[1].support {
				return fail()
			}
			shared[eid] = true
		} else if len(us) != 1 {
			return fail()
		}
		sc, e := simple(f, us[0].curve, "SURFACE_CURVE", 4)
		if e != nil {
			return fail()
		}
		pcs, e := listRefs(sc.Args[2], 1)
		if e != nil || len(pcs) != len(us) {
			return fail()
		}
		seen := map[int64]bool{}
		for _, pcid := range pcs {
			pc, e := simple(f, pcid, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			sid, e := ref(pc.Args[1])
			if e != nil || seen[sid] {
				return fail()
			}
			seen[sid] = true
			found := false
			for _, u := range us {
				found = found || sid == u.support
			}
			if !found {
				return fail()
			}
		}
	}
	if len(shared) != 4 {
		return fail()
	}
	// Bound every dependency before the existing geometry copier handles source
	// entities. A cycle, deep chain or unowned topological item fails closed.
	visited, active := map[int64]bool{}, map[int64]bool{}
	var visit func(int64, int) error
	visit = func(id int64, depth int) error {
		if active[id] || depth >= l.MaxDepth {
			return geom.ErrInvalid
		}
		if visited[id] {
			return nil
		}
		en, ok := f.Entities[id]
		if !ok {
			return geom.ErrInvalid
		}
		active[id] = true
		var value func(Value) error
		value = func(v Value) error {
			if v.Kind == Reference {
				return visit(v.Ref, depth+1)
			}
			for _, x := range v.Items {
				if e := value(x); e != nil {
					return e
				}
			}
			return nil
		}
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				if e := value(v); e != nil {
					return e
				}
			}
		}
		active[id] = false
		visited[id] = true
		return nil
	}
	for _, fid := range faces {
		if e = visit(fid, 0); e != nil {
			return fail()
		}
	}
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			switch rec.Type {
			case "ADVANCED_FACE", "EDGE_CURVE", "VERTEX_POINT", "ORIENTED_EDGE", "EDGE_LOOP", "FACE_BOUND", "FACE_OUTER_BOUND", "SURFACE_CURVE", "PCURVE", "B_SPLINE_CURVE", "B_SPLINE_SURFACE", "B_SPLINE_CURVE_WITH_KNOTS", "B_SPLINE_SURFACE_WITH_KNOTS":
				if !visited[id] {
					return fail()
				}
			case "OPEN_SHELL":
				if id != sourceShell {
					return fail()
				}
			case "SHELL_BASED_SURFACE_MODEL":
				if id != sourceModel {
					return fail()
				}
			case "CLOSED_SHELL", "MANIFOLD_SOLID_BREP", "FACETED_BREP", "BREP_WITH_VOIDS", "ORIENTED_CLOSED_SHELL", "ORIENTED_OPEN_SHELL":
				return fail()
			}
		}
	}
	result := CurvedOvalOpenShell{LengthUnit: unit}
	annulus, disk := false, false
	for _, fid := range faces {
		local := cloneOvalFile(f)
		for _, u := range faceUses[fid] {
			sc, e := ovalRecord(local, u.curve, "SURFACE_CURVE", 4)
			if e != nil {
				return fail()
			}
			pcs, _ := listRefs(sc.Args[2], 1)
			selected := int64(0)
			for _, pcid := range pcs {
				pc, _ := simple(local, pcid, "PCURVE", 3)
				sid, _ := ref(pc.Args[1])
				if sid == u.support {
					selected = pcid
				}
			}
			if selected == 0 {
				return fail()
			}
			sc.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: selected}}}
			ovalSet(local, u.curve, sc)
			if !u.sense {
				mid, _ := ref(sc.Args[1])
				pc, _ := simple(local, selected, "PCURVE", 3)
				rid, _ := ref(pc.Args[2])
				rep, e := simple(local, rid, "DEFINITIONAL_REPRESENTATION", 3)
				if e != nil {
					return fail()
				}
				curves, e := listRefs(rep.Args[1], 1)
				if e != nil || len(curves) != 1 {
					return fail()
				}
				if e = reverseOvalCurve(local, mid); e != nil {
					return fail()
				}
				if e = reverseOvalCurve(local, curves[0]); e != nil {
					return fail()
				}
				edge, e := ovalRecord(local, u.edge, "EDGE_CURVE", 5)
				if e != nil {
					return fail()
				}
				edge.Args[1], edge.Args[2] = edge.Args[2], edge.Args[1]
				ovalSet(local, u.edge, edge)
				oe, e := ovalRecord(local, u.use, "ORIENTED_EDGE", 5)
				if e != nil {
					return fail()
				}
				oe.Args[4] = Value{Kind: Enumeration, Text: "T"}
				ovalSet(local, u.use, oe)
			}
		}
		var raw bytes.Buffer
		var mapping map[int64]int64
		e = writeStandalone(&raw, unit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
			ids, remap := copySplineGeometryWithIDs(emit, local, []int64{fid})
			mapping = remap
			shell := emit(fmt.Sprintf("OPEN_SHELL('',(#%d))", ids[0]))
			return []int64{emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell))}
		})
		if e != nil {
			return CurvedOvalOpenShell{}, e
		}
		face, e := ReadCurvedOvalFace(&raw, l, o)
		if e != nil {
			return CurvedOvalOpenShell{}, e
		}
		reverse := map[int64]int64{}
		for old, n := range mapping {
			reverse[n] = old
		}
		face.SourceFaceID = fid
		for i, id := range face.SourceEdges {
			face.SourceEdges[i] = reverse[id]
		}
		for j, loop := range face.SourceHoleEdges {
			for i, id := range loop {
				face.SourceHoleEdges[j][i] = reverse[id]
			}
		}
		if len(face.Holes) == 1 {
			if annulus {
				return fail()
			}
			annulus = true
			result.Annulus = face
		} else if len(face.Holes) == 0 {
			if disk {
				return fail()
			}
			disk = true
			result.Disk = face
		} else {
			return fail()
		}
	}
	if !annulus || !disk {
		return fail()
	}
	for _, id := range result.Annulus.SourceEdges {
		if shared[id] {
			return fail()
		}
	}
	for i, id := range result.Annulus.SourceHoleEdges[0] {
		if !shared[id] {
			return fail()
		}
		result.SourceSharedEdges[i] = id
	}
	for _, id := range result.Disk.SourceEdges {
		if !shared[id] {
			return fail()
		}
	}
	if e = certifyCurvedOvalShell(result, o); e != nil {
		return CurvedOvalOpenShell{}, e
	}
	return result, nil
}
