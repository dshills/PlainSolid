package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/exchange/iges"
	"plainsolid/geom"
	"strings"
)

// QuadraticOvalOpenShell is exactly an affine annulus and the disk filling its
// sole hole. Supports keep independent charts; the common model rim is shared.
// This is an embedded open planar sheet, not a material solid.
type QuadraticOvalOpenShell struct {
	Annulus, Disk    QuadraticOvalFace
	LengthUnit       float64
	SourceSharedEdge int64
}

func certifyOvalShell(m QuadraticOvalOpenShell) error {
	if len(m.Annulus.Holes) != 1 || len(m.Disk.Holes) != 0 {
		return geom.ErrInvalid
	}
	a, e := iges.NewQuadraticOvalSurfaceWithHoles(m.Annulus.Geometry, m.Annulus.Outer, m.Annulus.Holes, .001)
	if e != nil {
		return e
	}
	b, e := iges.NewQuadraticOvalSurface(m.Disk.Geometry, m.Disk.Outer, .001)
	if e != nil {
		return e
	}
	ac := a.Entities[a.CurvesOnSurfaces[1].ModelCurve].Spline.Curve
	bc := b.Entities[b.CurvesOnSurfaces[0].ModelCurve].Spline.Curve
	x, e := geom.NewExactSplineCurve(ac, geom.SurfacePatchOptions{})
	if e != nil {
		return e
	}
	y, e := geom.NewExactSplineCurve(bc, geom.SurfacePatchOptions{})
	if e != nil {
		return e
	}
	same, e := geom.CompareExactCurvesContext(context.Background(), x, y, geom.ExactCurveComparisonOptions{ReverseB: true})
	if e != nil {
		return e
	}
	if !same.Complete || !same.Equal {
		return geom.ErrConvergence
	}
	return nil
}
func cloneOvalFile(f *File) *File {
	q := *f
	q.Entities = map[int64]Entity{}
	for id, en := range f.Entities {
		q.Entities[id] = en
	}
	return &q
}
func ovalSet(f *File, id int64, r Record) { f.Entities[id] = Entity{ID: id, Records: []Record{r}} }
func ovalRecord(f *File, id int64, kind string, n int) (Record, error) {
	r, e := simple(f, id, kind, n)
	r.Args = append([]Value(nil), r.Args...)
	return r, e
}

// reverseOvalCurve reverses a canonical four-span curve's directed controls.
// Its symmetric knot vector is independently verified by the face decoder.
func reverseOvalCurve(f *File, id int64) error {
	en := f.Entities[id]
	en.Records = append([]Record(nil), en.Records...)
	found := false
	for i, r := range en.Records {
		r.Args = append([]Value(nil), r.Args...)
		index := -1
		switch r.Type {
		case "B_SPLINE_CURVE":
			index = 1
			found = true
		case "B_SPLINE_CURVE_WITH_KNOTS":
			if len(en.Records) == 1 {
				index = 2
				found = true
			}
		case "RATIONAL_B_SPLINE_CURVE":
			index = 0
		}
		if index >= 0 {
			if index >= len(r.Args) || r.Args[index].Kind != Aggregate {
				return geom.ErrInvalid
			}
			v := r.Args[index]
			v.Items = append([]Value(nil), v.Items...)
			for a, b := 0, len(v.Items)-1; a < b; a, b = a+1, b-1 {
				v.Items[a], v.Items[b] = v.Items[b], v.Items[a]
			}
			r.Args[index] = v
		}
		en.Records[i] = r
	}
	if !found {
		return geom.ErrInvalid
	}
	f.Entities[id] = en
	return nil
}

type ovalShellUse struct {
	face, bound, loop, use, edge, curve, support int64
	sense                                        bool
}

func ovalFaceUses(f *File, id int64) ([]ovalShellUse, error) {
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
		if e != nil || len(uses) != 1 {
			return nil, geom.ErrInvalid
		}
		u, e := simple(f, uses[0], "ORIENTED_EDGE", 5)
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
		out = append(out, ovalShellUse{id, bid, lid, uses[0], eid, cid, sid, sense})
	}
	return out, nil
}
func WriteQuadraticOvalOpenShell(w io.Writer, m QuadraticOvalOpenShell) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := certifyOvalShell(m); e != nil {
		return e
	}
	var raw bytes.Buffer
	if e := WriteQuadraticOvalFaces(&raw, QuadraticOvalFaceModel{Faces: []QuadraticOvalFace{m.Annulus, m.Disk}, LengthUnit: m.LengthUnit}); e != nil {
		return e
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		return e
	}
	faces := []int64{}
	for _, id := range f.Order {
		if _, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
			faces = append(faces, id)
		}
	}
	if len(faces) != 2 {
		return geom.ErrInvalid
	}
	a, e := ovalFaceUses(f, faces[0])
	if e != nil {
		return e
	}
	b, e := ovalFaceUses(f, faces[1])
	if e != nil || len(a) != 2 || len(b) != 1 {
		return geom.ErrInvalid
	}
	hole, disk := a[1], b[0]
	hsc, e := ovalRecord(f, hole.curve, "SURFACE_CURVE", 4)
	if e != nil {
		return e
	}
	dsc, e := simple(f, disk.curve, "SURFACE_CURVE", 4)
	if e != nil {
		return e
	}
	pcs, e := listRefs(dsc.Args[2], 1)
	if e != nil || len(pcs) != 1 {
		return geom.ErrInvalid
	}
	pc, e := simple(f, pcs[0], "PCURVE", 3)
	if e != nil {
		return e
	}
	rid, _ := ref(pc.Args[2])
	rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil {
		return e
	}
	curves, e := listRefs(rep.Args[1], 1)
	if e != nil || len(curves) != 1 {
		return geom.ErrInvalid
	}
	if e = reverseOvalCurve(f, curves[0]); e != nil {
		return e
	}
	hsc.Args[2].Items = append(append([]Value(nil), hsc.Args[2].Items...), Value{Kind: Reference, Ref: pcs[0]})
	ovalSet(f, hole.curve, hsc)
	use, e := ovalRecord(f, disk.use, "ORIENTED_EDGE", 5)
	if e != nil {
		return e
	}
	use.Args[3] = Value{Kind: Reference, Ref: hole.edge}
	use.Args[4] = Value{Kind: Enumeration, Text: "F"}
	ovalSet(f, disk.use, use)
	var staged bytes.Buffer
	e = writeStandalone(&staged, m.LengthUnit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := copySplineGeometry(emit, f, faces)
		shell := emit("OPEN_SHELL(''," + references(ids) + ")")
		return []int64{emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell))}
	})
	if e != nil {
		return e
	}
	if _, e = ReadQuadraticOvalOpenShell(bytes.NewReader(staged.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}

func ReadQuadraticOvalOpenShell(r io.Reader, l Limits) (QuadraticOvalOpenShell, error) {
	fail := func() (QuadraticOvalOpenShell, error) {
		return QuadraticOvalOpenShell{}, fmt.Errorf("step: unsupported connected oval shell")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return QuadraticOvalOpenShell{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return QuadraticOvalOpenShell{}, e
	}
	var unit float64
	var faces []int64
	var sourceShell, sourceModel int64
	repCount := 0
	points := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, rec := range en.Records {
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
	if repCount != 1 || len(faces) != 2 || points > l.MaxVertices || l.MaxFaces < 2 || l.MaxEdgeUses < 3 {
		return fail()
	}
	all := map[int64][]ovalShellUse{}
	faceUses := map[int64][]ovalShellUse{}
	ownedBound := map[int64]bool{}
	ownedLoop := map[int64]bool{}
	ownedUse := map[int64]bool{}
	for _, fid := range faces {
		uses, e := ovalFaceUses(f, fid)
		if e != nil {
			return fail()
		}
		faceUses[fid] = uses
		for _, u := range uses {
			if ownedBound[u.bound] || ownedLoop[u.loop] || ownedUse[u.use] {
				return fail()
			}
			ownedBound[u.bound], ownedLoop[u.loop], ownedUse[u.use] = true, true, true
			all[u.edge] = append(all[u.edge], u)
		}
	}
	if len(all) != 2 || len(faceUses[faces[0]])+len(faceUses[faces[1]]) != 3 {
		return fail()
	}
	shared := int64(0)
	for eid, us := range all {
		if len(us) == 2 {
			if shared != 0 || us[0].face == us[1].face || us[0].sense == us[1].sense || us[0].support == us[1].support {
				return fail()
			}
			shared = eid
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
	if shared == 0 {
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
			case "ADVANCED_FACE", "EDGE_CURVE", "VERTEX_POINT", "ORIENTED_EDGE", "EDGE_LOOP", "FACE_BOUND", "FACE_OUTER_BOUND", "SURFACE_CURVE", "PCURVE", "B_SPLINE_CURVE_WITH_KNOTS", "B_SPLINE_SURFACE_WITH_KNOTS":
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
			case "CLOSED_SHELL", "MANIFOLD_SOLID_BREP", "BREP_WITH_VOIDS":
				return fail()
			}
		}
	}
	result := QuadraticOvalOpenShell{LengthUnit: unit, SourceSharedEdge: shared}
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
				oe, e := ovalRecord(local, u.use, "ORIENTED_EDGE", 5)
				if e != nil {
					return fail()
				}
				oe.Args[4] = Value{Kind: Enumeration, Text: "T"}
				ovalSet(local, u.use, oe)
			}
		}
		var raw bytes.Buffer
		e = writeStandalone(&raw, unit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
			ids := copySplineGeometry(emit, local, []int64{fid})
			shell := emit(fmt.Sprintf("OPEN_SHELL('',(#%d))", ids[0]))
			return []int64{emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell))}
		})
		if e != nil {
			return QuadraticOvalOpenShell{}, e
		}
		m, e := ReadQuadraticOvalFaces(&raw, l)
		if e != nil || len(m.Faces) != 1 {
			return fail()
		}
		face := m.Faces[0]
		face.SourceFaceID = fid
		face.SourceEdges = nil
		for _, u := range faceUses[fid] {
			face.SourceEdges = append(face.SourceEdges, u.edge)
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
	if e = certifyOvalShell(result); e != nil {
		return QuadraticOvalOpenShell{}, e
	}
	return result, nil
}
