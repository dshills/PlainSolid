package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

// ReadSplineLensBody proves an explicit closed two-face elliptical graph lens.
func ReadSplineLensBody(r io.Reader, l Limits, o CurvedOvalOptions) (SplineLensBody, error) {
	return readSplineLensBody(r, l, o, false)
}
func readSplineLensBody(r io.Reader, l Limits, o CurvedOvalOptions, partitioned bool, refined ...bool) (SplineLensBody, error) {
	fail := func() (SplineLensBody, error) {
		return SplineLensBody{}, fmt.Errorf("step: unsupported exact spline lens body")
	}
	var e error
	o, e = o.defaults()
	if e != nil {
		return SplineLensBody{}, e
	}
	o.MaxOperations /= 2
	if o.MaxOperations < 10 {
		return SplineLensBody{}, geom.ErrConvergence
	}
	l, e = l.defaults()
	if e != nil {
		return SplineLensBody{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return SplineLensBody{}, e
	}
	var unit float64
	var faces []int64
	var sourceShell, sourceModel int64
	repCount := 0
	points := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, rec := range en.Records {
			if strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") && rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
				return fail()
			}
			if rec.Type == "CARTESIAN_POINT" {
				points++
			}
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" {
				return fail()
			}
		}
		rep, e := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
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
			m, e := simple(f, item, "MANIFOLD_SOLID_BREP", 2)
			if e != nil {
				return fail()
			}
			sourceShell, e = ref(m.Args[1])
			if e != nil {
				return fail()
			}
			shell, e := simple(f, sourceShell, "CLOSED_SHELL", 2)
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
	if repCount != 1 || len(faces) != 2 || points > l.MaxVertices || l.MaxFaces < 2 || l.MaxEdgeUses < 8 {
		return fail()
	}
	all := map[int64][]ovalShellUse{}
	faceUses := map[int64][]ovalShellUse{}
	ownedBound := map[int64]int64{}

	for _, fid := range faces {
		maximum := 4
		if partitioned {
			maximum = 128
		}
		uses, e := curvedOvalFaceUsesRange(f, fid, 4, maximum)
		if e != nil {
			return fail()
		}
		if len(uses) < 4 || (!partitioned && len(uses) != 4) {
			return fail()
		}
		fr, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		positive, e := boolean(fr.Args[3])
		if e != nil {
			return fail()
		}
		perFaceUses := map[int64]bool{}
		for _, u := range uses {
			bound, e := simple(f, u.bound, "FACE_BOUND", 3)
			if e != nil {
				bound, e = simple(f, u.bound, "FACE_OUTER_BOUND", 3)
			}
			if e != nil {
				return fail()
			}
			sense, e := boolean(bound.Args[2])
			if e != nil || sense != positive {
				return fail()
			}
		}
		faceUses[fid] = uses
		for _, u := range uses {
			if (ownedBound[u.bound] != 0 && ownedBound[u.bound] != fid) || perFaceUses[u.use] {
				return fail()
			}
			ownedBound[u.bound], perFaceUses[u.use] = fid, true
			all[u.edge] = append(all[u.edge], u)
		}
	}
	edgeCount := len(faceUses[faces[0]])
	if len(all) != edgeCount || len(faceUses[faces[1]]) != edgeCount || edgeCount > l.MaxEdgeUses/2 {
		return fail()
	}
	shared := map[int64]bool{}
	for eid, us := range all {
		if len(us) == 2 {
			if us[0].face == us[1].face || us[0].sense != us[1].sense || us[0].support == us[1].support {
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
	if len(shared) != edgeCount {
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
			case "CLOSED_SHELL":
				if id != sourceShell {
					return fail()
				}
			case "MANIFOLD_SOLID_BREP":
				if id != sourceModel {
					return fail()
				}
			case "OPEN_SHELL", "SHELL_BASED_SURFACE_MODEL", "FACETED_BREP", "BREP_WITH_VOIDS", "ORIENTED_CLOSED_SHELL", "ORIENTED_OPEN_SHELL":
				return fail()
			}
		}
	}
	body, e := simple(f, sourceModel, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	result := SplineLensBody{Name: name, LengthUnit: unit, SourceBodyID: sourceModel}
	upper, lower := false, false
	var sourcePieces [2][]lensPieceGeometry
	var firstEllipse *QuadraticOval
	firstShear := 0.
	for _, fid := range faces {
		local := cloneOvalFile(f)
		fr, e := ovalRecord(local, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		positive, e := boolean(fr.Args[3])
		if e != nil {
			return fail()
		}
		fr.Args[3] = Value{Kind: Enumeration, Text: "T"}
		ovalSet(local, fid, fr)
		for _, u := range faceUses[fid] {
			bound, e := ovalRecord(local, u.bound, "FACE_BOUND", 3)
			if e != nil {
				bound, e = ovalRecord(local, u.bound, "FACE_OUTER_BOUND", 3)
			}
			if e != nil {
				return fail()
			}
			bound.Args[2] = Value{Kind: Enumeration, Text: "T"}
			ovalSet(local, u.bound, bound)

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
		var face CurvedOvalFace
		var pieces []lensPieceGeometry
		if partitioned {
			face, pieces, e = decodePartitionedLensFace(local, fid, o, len(refined) > 0 && refined[0], len(refined) > 1 && refined[1])
			if e != nil {
				return SplineLensBody{}, fmt.Errorf("step: lens face #%d: %w", fid, e)
			}
		} else {
			var raw bytes.Buffer
			var mapping map[int64]int64
			e = writeStandalone(&raw, unit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
				ids, remap := copySplineGeometryWithIDs(emit, local, []int64{fid})
				mapping = remap
				shell := emit(fmt.Sprintf("OPEN_SHELL('',(#%d))", ids[0]))
				return []int64{emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell))}
			})
			if e != nil {
				return SplineLensBody{}, e
			}
			face, e = readCurvedOvalFace(&raw, l, o, true)
			if e != nil {
				return SplineLensBody{}, e
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
		}
		index := 0
		if positive {
			if upper {
				return fail()
			}
			upper = true
			result.Upper = face.Geometry
			result.Center = face.Outer.Center
			result.Radii = face.Outer.Radii
			result.Shear = face.ellipseShear
			if !partitioned {
				result.SourceEdges = face.SourceEdges
			}
		} else {
			if lower {
				return fail()
			}
			lower = true
			result.Lower = face.Geometry
			index = 1
		}
		sourcePieces[index] = pieces
		result.FaceNames[index] = face.Name
		result.SourceFaceIDs[index] = fid
		ids := []int64{}
		if partitioned {
			for _, p := range pieces {
				ids = append(ids, p.SourceEdgeID)
			}
		} else {
			ids = append(ids, face.SourceEdges[:]...)
		}
		for _, id := range ids {
			if !shared[id] {
				return fail()
			}
		}
		// The two charts must describe the same explicit UV ellipse.
		if firstEllipse == nil {
			q := face.Outer
			firstEllipse = &q
			firstShear = face.ellipseShear
		} else if firstEllipse.Center != face.Outer.Center || firstEllipse.Radii != face.Outer.Radii || firstShear != face.ellipseShear {
			return fail()
		}
		if len(face.Holes) != 0 {
			return fail()
		}
		if result.Center != (geom.Vec2{}) && (result.Center != face.Outer.Center || result.Radii != face.Outer.Radii) {
			return fail()
		}
	}
	if !upper || !lower {
		return fail()
	}
	solid, e := certifySplineLens(result, o)
	if e != nil {
		return SplineLensBody{}, e
	}
	if partitioned {
		plan, e := lensPiecePlan(result, o)
		if e != nil {
			return SplineLensBody{}, e
		}
		if len(plan) != edgeCount {
			return fail()
		}
		for i, p := range plan {
			for owner := 0; owner < 2; owner++ {
				if len(sourcePieces[owner]) != len(plan) {
					return fail()
				}
				source := sourcePieces[owner][i]
				if source.Quarter != p.Quarter || source.Start.Cmp(p.Start) != 0 || source.End.Cmp(p.End) != 0 {
					return fail()
				}
				if owner == 1 && source.SourceEdgeID != sourcePieces[0][i].SourceEdgeID {
					return fail()
				}
				if e = compareLensPiece(source.UV, p.UV, o); e != nil {
					return SplineLensBody{}, e
				}
				if e = compareLensPiece(source.Model, p.Model, o); e != nil {
					return SplineLensBody{}, e
				}
			}
			p.SourceEdgeID = sourcePieces[0][i].SourceEdgeID
			result.sourcePieces = append(result.sourcePieces, p.LensSeamPiece)
			if len(refined) > 0 && refined[0] {
				result.sourceCurves = append(result.sourceCurves, [3]*geom.BSpline{sourcePieces[0][i].ModelSpline, sourcePieces[0][i].UVSpline, sourcePieces[1][i].UVSpline})
				if len(refined) > 1 && refined[1] {
					result.sourceRationalCurves = append(result.sourceRationalCurves, [3]RationalLensCurve{sourcePieces[0][i].ModelRational, sourcePieces[0][i].UVRational, sourcePieces[1][i].UVRational})
				}
			}
		}
	}
	result.Solid = solid
	return result, nil
}
