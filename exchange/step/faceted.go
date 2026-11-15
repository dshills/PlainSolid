package step

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"plainsolid/brep"
	"plainsolid/geom"
)

// Body is an independently closed, connected faceted solid.
type Body struct {
	Name  string
	Solid brep.Solid
}

// Model retains file coordinate units without rescaling coordinates. LengthUnit
// is meters per coordinate unit. Mixed-unit representations are rejected.
// Assembly placements, product metadata and styles are not imported.
type Model struct {
	Bodies     []Body
	LengthUnit float64
}

func simple(f *File, id int64, kind string, n int) (Record, error) {
	e, ok := f.Entities[id]
	if !ok || len(e.Records) != 1 || e.Records[0].Type != kind {
		return Record{}, fmt.Errorf("step: #%d must be simple %s", id, kind)
	}
	r := e.Records[0]
	if len(r.Args) != n {
		return Record{}, fmt.Errorf("step: #%d %s needs %d attributes", id, kind, n)
	}
	return r, nil
}
func ref(v Value) (int64, error) {
	if v.Kind != Reference {
		return 0, errors.New("step: expected entity reference")
	}
	return v.Ref, nil
}
func listRefs(v Value, min int) ([]int64, error) {
	if v.Kind != Aggregate || len(v.Items) < min {
		return nil, errors.New("step: expected reference list")
	}
	a := make([]int64, len(v.Items))
	seen := map[int64]bool{}
	for i, x := range v.Items {
		id, e := ref(x)
		if e != nil {
			return nil, e
		}
		if seen[id] {
			return nil, errors.New("step: duplicate aggregate reference")
		}
		seen[id] = true
		a[i] = id
	}
	return a, nil
}
func textValue(v Value) (string, error) {
	if v.Kind != String {
		return "", errors.New("step: expected string")
	}
	return v.Text, nil
}
func numeric(v Value) (float64, error) {
	if v.Kind != Number {
		return 0, errors.New("step: expected number")
	}
	x, e := strconv.ParseFloat(v.Text, 64)
	if e != nil || math.IsInf(x, 0) || math.IsNaN(x) {
		return 0, errors.New("step: invalid finite number")
	}
	return x, nil
}
func component(e Entity, name string) (Record, bool) {
	for _, r := range e.Records {
		if r.Type == name {
			return r, true
		}
	}
	return Record{}, false
}
func contextUnits(f *File, id int64) (float64, error) {
	e, ok := f.Entities[id]
	if !ok {
		return 0, errors.New("step: missing context")
	}
	for _, r := range e.Records {
		switch r.Type {
		case "GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT", "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT", "REPRESENTATION_CONTEXT":
		default:
			return 0, fmt.Errorf("step: unsupported context component %s", r.Type)
		}
	}
	g, ok := component(e, "GEOMETRIC_REPRESENTATION_CONTEXT")
	if !ok || len(g.Args) != 1 {
		return 0, errors.New("step: unsupported geometric context")
	}
	dim, err := numeric(g.Args[0])
	if err != nil || dim != 3 {
		return 0, errors.New("step: only three-dimensional contexts supported")
	}
	r, ok := component(e, "REPRESENTATION_CONTEXT")
	if !ok || len(r.Args) != 2 || r.Args[0].Kind != String || r.Args[1].Kind != String {
		return 0, errors.New("step: invalid representation context")
	}
	u, ok := component(e, "GLOBAL_UNIT_ASSIGNED_CONTEXT")
	if !ok || len(u.Args) != 1 {
		return 0, errors.New("step: explicit units required")
	}
	ids, err := listRefs(u.Args[0], 1)
	if err != nil {
		return 0, err
	}
	unit := 0.
	for _, id := range ids {
		ent := f.Entities[id]
		lengthRecord, isLength := component(ent, "LENGTH_UNIT")
		if !isLength {
			continue
		}
		if len(lengthRecord.Args) != 0 || unit != 0 {
			return 0, errors.New("step: invalid or multiple length units")
		}
		unit, err = lengthUnit(f, id)
		if err != nil {
			return 0, err
		}

	}
	if unit == 0 {
		return 0, errors.New("step: missing length unit")
	}
	return unit, nil
}

// Read imports faceted or planar advanced B-reps with explicit SI or conversion-based length units.
// Faceted bodies use polygon loops; advanced bodies use shared linear edges,
// optionally with validated planar parameter-space curves. Bound and surface
// orientations are respected. Faceted coordinate duplicates are welded; advanced
// source vertex/edge identities are preserved. Assemblies, curved faces and
// void-body entities are rejected. Product metadata is not transferred.
func Read(r io.Reader, l Limits) (Model, error) {
	return readPlanarModel(r, l, false)
}

func readPlanarModel(r io.Reader, l Limits, exactSupports bool) (Model, error) {
	l, e := l.defaults()
	if e != nil {
		return Model{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return Model{}, e
	}
	model := Model{}
	used := map[int64]bool{}
	totalVertices, totalFaces, totalUses := 0, 0, 0
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || rec.Type == "ITEM_DEFINED_TRANSFORMATION" || rec.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE" || rec.Type == "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION" || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || strings.HasPrefix(rec.Type, "CARTESIAN_TRANSFORMATION_OPERATOR") {
				return Model{}, fmt.Errorf("step: unsupported assembly/placement entity %s", rec.Type)
			}
		}
		if len(ent.Records) != 1 {
			for _, rec := range ent.Records {
				if strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") {
					return Model{}, errors.New("step: complex shape representation unsupported")
				}
			}
			continue
		}
		kind := ent.Records[0].Type
		if strings.HasSuffix(kind, "SHAPE_REPRESENTATION") && kind != "FACETED_BREP_SHAPE_REPRESENTATION" && kind != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			return Model{}, fmt.Errorf("step: unsupported representation %s", kind)
		}
		if kind != "FACETED_BREP_SHAPE_REPRESENTATION" && kind != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		rep, e := simple(f, id, kind, 3)
		if e != nil {
			return Model{}, e
		}
		if _, e = textValue(rep.Args[0]); e != nil {
			return Model{}, e
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return Model{}, e
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return Model{}, e
		}
		unit, e := contextUnits(f, ctx)
		if e != nil {
			return Model{}, e
		}
		if model.LengthUnit != 0 && unit != model.LengthUnit {
			return Model{}, errors.New("step: mixed length units")
		}
		model.LengthUnit = unit
		for _, bodyID := range items {
			if item := f.Entities[bodyID]; len(item.Records) == 1 && item.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				if _, e := placement(f, bodyID); e != nil {
					return Model{}, e
				}
				continue
			}
			if used[bodyID] {
				return Model{}, errors.New("step: repeated body/assembly instances unsupported")
			}
			if len(model.Bodies) >= l.MaxSolids {
				return Model{}, errors.New("step: solid limit exceeded")
			}
			body, e := readPlanarBody(f, bodyID, l.MaxVertices-totalVertices, l.MaxFaces-totalFaces, l.MaxEdgeUses-totalUses, kind == "ADVANCED_BREP_SHAPE_REPRESENTATION", exactSupports)
			if e != nil {
				return Model{}, e
			}
			totalVertices += len(body.Solid.Vertices())
			totalFaces += len(body.Solid.Faces())
			for _, face := range body.Solid.Faces() {
				totalUses += len(face.Boundary)
				for _, h := range face.Holes {
					totalUses += len(h)
				}
			}
			used[bodyID] = true
			model.Bodies = append(model.Bodies, body)
		}
	}
	if len(model.Bodies) == 0 {
		return Model{}, errors.New("step: no supported planar shape representation")
	}
	for _, ent := range f.Entities {
		for _, r := range ent.Records {
			if (r.Type == "FACETED_BREP" || r.Type == "MANIFOLD_SOLID_BREP") && !used[ent.ID] {
				return Model{}, errors.New("step: unrepresented solid body")
			}
		}
	}
	return model, nil
}
func readBody(f *File, id int64, maxVertices, maxFaces, maxEdgeUses int, advanced bool) (Body, error) {
	return readPlanarBody(f, id, maxVertices, maxFaces, maxEdgeUses, advanced, false)
}

func readPlanarBody(f *File, id int64, maxVertices, maxFaces, maxEdgeUses int, advanced, exactSupports bool) (Body, error) {
	bodyKind := "FACETED_BREP"
	if advanced {
		bodyKind = "MANIFOLD_SOLID_BREP"
	}
	r, e := simple(f, id, bodyKind, 2)
	if e != nil {
		return Body{}, e
	}
	name, e := textValue(r.Args[0])
	if e != nil {
		return Body{}, e
	}
	shellID, e := ref(r.Args[1])
	if e != nil {
		return Body{}, e
	}
	shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
	if e != nil {
		return Body{}, e
	}
	if _, e = textValue(shell.Args[0]); e != nil {
		return Body{}, e
	}
	faceIDs, e := listRefs(shell.Args[1], 1)
	if e != nil {
		return Body{}, e
	}
	if len(faceIDs) > maxFaces {
		return Body{}, errors.New("step: face limit exceeded")
	}
	vertices := []geom.Vec3{}
	edges := []brep.Edge{}
	faces := []brep.Face{}
	supportingPlanes := []*geom.Plane{}
	pointIndex := map[geom.Vec3]int{}
	edgeIndex := map[[2]int]int{}
	pointNumber := numeric
	if exactSupports {
		pointNumber = exactVertexContactPointNumber
	}
	point := func(id int64) (int, error) {
		r, e := simple(f, id, "CARTESIAN_POINT", 2)
		if e != nil {
			return 0, e
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return 0, e
		}
		v := r.Args[1]
		if v.Kind != Aggregate || len(v.Items) != 3 {
			return 0, errors.New("step: expected three point coordinates")
		}
		p := geom.Vec3{}
		for i, x := range []*float64{&p.X, &p.Y, &p.Z} {
			*x, e = pointNumber(v.Items[i])
			if e != nil {
				return 0, fmt.Errorf("step: vertex point #%d axis %d: %w", id, i, e)
			}
		}
		if n, ok := pointIndex[p]; ok {
			return n, nil
		}
		if len(vertices) >= maxVertices {
			return 0, errors.New("step: vertex limit exceeded")
		}
		n := len(vertices)
		pointIndex[p] = n
		vertices = append(vertices, p)
		return n, nil
	}
	edgeUses := 0
	linearEdges := map[int64]linearEdge{}
	vertexOwners := map[int]int64{}
	edgeOwners := map[[2]int]int64{}
	bound := func(id int64, outer bool) ([]brep.EdgeUse, error) {
		kind := "FACE_BOUND"
		if outer {
			kind = "FACE_OUTER_BOUND"
		}
		r, e := simple(f, id, kind, 3)
		if e != nil {
			return nil, e
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return nil, e
		}
		loopID, e := ref(r.Args[1])
		if e != nil {
			return nil, e
		}
		orientation := r.Args[2]
		if orientation.Kind != Enumeration || (orientation.Text != "T" && orientation.Text != "F") {
			return nil, errors.New("step: expected boolean bound orientation")
		}
		var ids []int64
		var linear linearLoop
		if advanced {
			loop, e := simple(f, loopID, "EDGE_LOOP", 2)
			if e != nil {
				return nil, e
			}
			if loop.Args[1].Kind != Aggregate || len(loop.Args[1].Items) > maxEdgeUses-edgeUses {
				return nil, errors.New("step: expanded edge-use limit exceeded")
			}
			linear, e = edgeLoop(f, loopID, linearEdges)
			if e != nil {
				return nil, e
			}
			ids = linear.points
		} else {
			loop, e := simple(f, loopID, "POLY_LOOP", 2)
			if e != nil {
				return nil, e
			}
			if _, e = textValue(loop.Args[0]); e != nil {
				return nil, e
			}
			ids, e = listRefs(loop.Args[1], 3)
			if e != nil {
				return nil, e
			}
		}
		if len(ids) > maxEdgeUses-edgeUses {
			return nil, errors.New("step: expanded edge-use limit exceeded")
		}
		edgeUses += len(ids)
		indices := make([]int, len(ids))
		for i, id := range ids {
			indices[i], e = point(id)
			if e != nil {
				return nil, e
			}
			if advanced {
				if prior, ok := vertexOwners[indices[i]]; ok && prior != linear.vertices[i] {
					return nil, errors.New("step: distinct coincident vertices unsupported")
				}
				vertexOwners[indices[i]] = linear.vertices[i]
			}
		}
		uses := make([]brep.EdgeUse, len(indices))
		for i, a := range indices {
			b := indices[(i+1)%len(indices)]
			key := [2]int{a, b}
			if a > b {
				key = [2]int{b, a}
			}
			if advanced {
				if prior, ok := edgeOwners[key]; ok && prior != linear.edges[i] {
					return nil, errors.New("step: distinct coincident edges unsupported")
				}
				edgeOwners[key] = linear.edges[i]
			}
			n, ok := edgeIndex[key]
			if !ok {
				if advanced && exactSupports {
					source := linear.edges[i]
					if err := exactVertexContactLine(f, source, linearEdges[source]); err != nil {
						return nil, fmt.Errorf("step: exact edge #%d: %w", source, err)
					}
				}
				n = len(edges)
				edgeIndex[key] = n
				edges = append(edges, brep.Edge{Start: key[0], End: key[1]})
			}
			uses[i] = brep.EdgeUse{Edge: n, Reversed: a > b}
		}
		if orientation.Text == "F" {
			for i, j := 0, len(uses)-1; i < j; i, j = i+1, j-1 {
				uses[i], uses[j] = uses[j], uses[i]
			}
			for i := range uses {
				uses[i].Reversed = !uses[i].Reversed
			}
		}
		return uses, nil
	}
	for _, id := range faceIDs {
		kind := "FACE"
		count := 2
		if advanced {
			kind = "ADVANCED_FACE"
			count = 4
		}
		ent := f.Entities[id]
		if !advanced && len(ent.Records) == 1 && ent.Records[0].Type == "FACE_SURFACE" {
			kind = "FACE_SURFACE"
			count = 4
		}
		r, e := simple(f, id, kind, count)
		if e != nil {
			return Body{}, e
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return Body{}, e
		}
		ids, e := listRefs(r.Args[1], 1)
		if e != nil {
			return Body{}, e
		}
		face := brep.Face{}
		outerFound := false
		for _, id := range ids {
			ent := f.Entities[id]
			if len(ent.Records) != 1 {
				return Body{}, errors.New("step: complex face bound unsupported")
			}
			outer := ent.Records[0].Type == "FACE_OUTER_BOUND"
			uses, e := bound(id, outer)
			if e != nil {
				return Body{}, e
			}
			if outer {
				if outerFound {
					return Body{}, errors.New("step: multiple outer bounds")
				}
				outerFound = true
				face.Boundary = uses
			} else {
				face.Holes = append(face.Holes, uses)
			}
		}
		if !outerFound {
			if !advanced {
				return Body{}, errors.New("step: explicit outer bound required")
			}
			index, e := largestLoop(face.Holes, vertices, edges)
			if e != nil {
				return Body{}, e
			}
			face.Boundary = face.Holes[index]
			face.Holes = append(face.Holes[:index], face.Holes[index+1:]...)
		}
		plane, e := facePlane(f, r)
		if e != nil {
			return Body{}, e
		}
		if exactSupports && r.Type != "FACE" {
			if e := exactVertexContactPlane(f, r, vertices, edges, face); e != nil {
				return Body{}, fmt.Errorf("step: exact face #%d: %w", id, e)
			}
		}
		supportingPlanes = append(supportingPlanes, plane)
		faces = append(faces, face)
	}
	s, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return Body{}, fmt.Errorf("step: invalid planar topology: %w", e)
	}
	for i, p := range supportingPlanes {
		if e := validateFacePlane(s, i, p, vertices, edges, faces[i]); e != nil {
			return Body{}, e
		}
	}
	if s.Topology().Shells != 1 {
		return Body{}, errors.New("step: closed shell must be connected")
	}
	volume := s.Tessellate().SignedVolume()
	if volume <= 0 || math.IsNaN(volume) || math.IsInf(volume, 0) {
		return Body{}, errors.New("step: outward positive-volume shell required")
	}
	return Body{Name: name, Solid: s}, nil
}
