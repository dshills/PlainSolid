package step

import (
	"errors"
	"io"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
)

// SplitCylinderModel retains the authored four-face/six-edge ancestry while
// Solid is the equivalent canonical cylinder. Coordinates are restricted to a
// translated signed-coordinate cylinder with antipodal local ±X seams. No source repair occurs.
type SplitCylinderModel struct {
	Frame                      geom.Frame // exact original signed-coordinate source frame
	Name                       string
	LengthUnit, Radius, Height float64
	Solid                      parametric.Solid
	Body, Shell                int64
	Faces                      [4]int64 // upper angular half, lower angular half, bottom, top
	Edges                      [6]int64 // bottom halves, top halves, +X and -X generatrices
}

func splitFailure() error { return errors.New("step: unsupported exact model-only split cylinder") }

// ReadSplitCylinderModel accepts one independent model-only BRep with two
// complementary semicylindrical faces. It requires exact represented source
// numbers, direct circle/line edges, positive physical circle direction and complete oriented
// ownership. Pcurves, transforms, assemblies and tolerance repair are excluded.
func ReadSplitCylinderModel(r io.Reader, l Limits) (SplitCylinderModel, error) {
	m, e := readCylinderCells(r, l, 2)
	if e != nil {
		return SplitCylinderModel{}, e
	}
	out := SplitCylinderModel{Frame: m.Frame, Name: m.Name, LengthUnit: m.LengthUnit, Radius: m.Radius, Height: m.Height, Solid: m.Solid, Body: m.Body, Shell: m.Shell}
	copy(out.Faces[:], m.Faces)
	copy(out.Edges[:], m.Edges)
	return out, nil
}

type cylinderCellsModel struct {
	Frame                      geom.Frame
	Name                       string
	LengthUnit, Radius, Height float64
	Solid                      parametric.Solid
	Body, Shell                int64
	Faces, Edges               []int64
}

func cylinderCellRole(p geom.Vec3, r float64, n int) int {
	points := []geom.Vec3{{X: r}, {X: -r}}
	if n == 4 {
		points = []geom.Vec3{{X: r}, {Y: r}, {X: -r}, {Y: -r}}
	}
	for i, q := range points {
		if p.X == q.X && p.Y == q.Y {
			return i
		}
	}
	return -1
}
func cylinderCellLoops(n int) [][]int {
	loops := make([][]int, n+2)
	for k := 0; k < n; k++ {
		loops[k] = []int{k + 1, 2*n + (k+1)%n + 1, -(n + k + 1), -(2*n + k + 1)}
	}
	for k := 0; k < n; k++ {
		loops[n] = append(loops[n], -(n - k))
		loops[n+1] = append(loops[n+1], n+k+1)
	}
	return loops
}
func readCylinderCells(r io.Reader, l Limits, cells int) (cylinderCellsModel, error) {
	fail := func() (cylinderCellsModel, error) { return cylinderCellsModel{}, splitFailure() }
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	if l.MaxFaces < cells+2 || l.MaxVertices < 2*cells || l.MaxEdgeUses < 6*cells {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return cylinderCellsModel{}, e
	}
	if len(f.Entities) > 256 {
		return fail()
	}
	sourceFrame, e := splitNormalizeFrame(f)
	if e != nil {
		return fail()
	}
	// Every entity must belong to the single representation's reference closure.
	var repID int64
	allowed := map[string]bool{}
	for _, k := range []string{"ADVANCED_BREP_SHAPE_REPRESENTATION", "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_OUTER_BOUND", "FACE_BOUND", "EDGE_LOOP", "ORIENTED_EDGE", "EDGE_CURVE", "VERTEX_POINT", "CARTESIAN_POINT", "DIRECTION", "VECTOR", "LINE", "CIRCLE", "AXIS2_PLACEMENT_3D", "CYLINDRICAL_SURFACE", "PLANE", "GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT", "REPRESENTATION_CONTEXT", "LENGTH_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT", "NAMED_UNIT", "SI_UNIT"} {
		allowed[k] = true
	}
	for id, ent := range f.Entities {
		for _, rec := range ent.Records {
			if !allowed[rec.Type] {
				return fail()
			}
			if rec.Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
				if repID != 0 {
					return fail()
				}
				repID = id
			}
		}
	}
	rep, e := simple(f, repID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	seen := map[int64]bool{}
	active := map[int64]bool{}
	var walk func(int64, int) bool
	walk = func(id int64, d int) bool {
		if active[id] || d > 32 {
			return false
		}
		if seen[id] {
			return true
		}
		ent, ok := f.Entities[id]
		if !ok {
			return false
		}
		active[id] = true
		var value func(Value) bool
		value = func(v Value) bool {
			if v.Kind == Reference {
				return walk(v.Ref, d+1)
			}
			for _, x := range v.Items {
				if !value(x) {
					return false
				}
			}
			return true
		}
		for _, rec := range ent.Records {
			for _, v := range rec.Args {
				if !value(v) {
					return false
				}
			}
		}
		delete(active, id)
		seen[id] = true
		return true
	}
	if !walk(repID, 0) || len(seen) != len(f.Entities) {
		return fail()
	}
	ids, e := listRefs(rep.Args[1], 1)
	if e != nil || len(ids) != 1 {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	unit, e := analyticSupportContextUnits(f, ctx)
	if e != nil {
		return fail()
	}
	body, e := simple(f, ids[0], "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	sh, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, sh, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	faces, e := listRefs(shell.Args[1], cells+2)
	if e != nil || len(faces) != cells+2 {
		return fail()
	}
	out := cylinderCellsModel{Frame: sourceFrame, Name: name, LengthUnit: unit, Body: ids[0], Shell: sh, Faces: make([]int64, cells+2), Edges: make([]int64, 3*cells)}
	type faceData struct {
		id    int64
		side  bool
		z     float64
		sense bool
		uses  []analyticUse
	}
	fs := []faceData{}
	for _, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		ent := f.Entities[sid]
		if len(ent.Records) != 1 {
			return fail()
		}
		rec := ent.Records[0]
		side := rec.Type == "CYLINDRICAL_SURFACE"
		if !side && rec.Type != "PLANE" {
			return fail()
		}
		arity := 2
		if side {
			arity = 3
		}
		rec, e = simple(f, sid, rec.Type, arity)
		if e != nil {
			return fail()
		}
		pid, e := ref(rec.Args[1])
		if e != nil {
			return fail()
		}
		z, e := splitPlacement(f, pid)
		if e != nil {
			return fail()
		}
		if side {
			if z != 0 {
				return fail()
			}
			rad, e := cylinderSupportNumber(rec.Args[2])
			if e != nil || rad <= 0 {
				return fail()
			}
			if out.Radius != 0 && out.Radius != rad {
				return fail()
			}
			out.Radius = rad
		}
		n := cells
		if side {
			n = 4
		}
		_, uses, e := cylinderCellFaceLoop(f, id, n)
		if e != nil {
			return fail()
		}
		sense, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		fs = append(fs, faceData{id, side, z, sense, uses})
		if !side && z > out.Height {
			out.Height = z
		}
	}
	if out.Radius <= 0 || out.Height <= 0 {
		return fail()
	}
	vertices := map[int64]int{}
	edges := map[int64]int{}
	edgeEnds := map[int64][2]int64{}
	for _, face := range fs {
		for _, u := range face.uses {
			if _, ok := edges[u.id]; ok {
				continue
			}
			a, b, g, curveSense, e := cylinderCellEdge(f, u.id)
			if e != nil || a == b {
				return fail()
			}
			points := [2]geom.Vec3{}
			for i, v := range []int64{a, b} {
				vr, e := simple(f, v, "VERTEX_POINT", 2)
				if e != nil {
					return fail()
				}
				p, e := ref(vr.Args[1])
				if e != nil {
					return fail()
				}
				xyz, e := splitTriple(f, p, "CARTESIAN_POINT")
				if e != nil || cylinderCellRole(xyz, out.Radius, cells) < 0 || (xyz.Z != 0 && xyz.Z != out.Height) {
					return fail()
				}
				points[i] = xyz
				role := cylinderCellRole(xyz, out.Radius, cells)
				if xyz.Z > 0 {
					role += cells
				}
				if old, ok := vertices[v]; ok && old != role {
					return fail()
				}
				vertices[v] = role
			}
			role := -1
			ent := f.Entities[g]
			if len(ent.Records) != 1 {
				return fail()
			}
			rec := ent.Records[0]
			if rec.Type == "CIRCLE" {
				rec, e = simple(f, g, "CIRCLE", 3)
				if e != nil || points[0].Z != points[1].Z || vertices[b]%cells != (vertices[a]+1)%cells {
					return fail()
				}
				pid, e := ref(rec.Args[1])
				if e != nil {
					return fail()
				}
				z, normal, e := cylinderCellCirclePlacement(f, pid)
				if e != nil || curveSense != (normal > 0) || z != points[0].Z {
					return fail()
				}
				rad, e := cylinderSupportNumber(rec.Args[2])
				if e != nil || rad != out.Radius {
					return fail()
				}
				role = vertices[a]
			} else if rec.Type == "LINE" {
				rec, e = simple(f, g, "LINE", 3)
				if e != nil || points[0].X != points[1].X || points[0].Y != points[1].Y || points[0].Z != 0 || points[1].Z != out.Height {
					return fail()
				}
				pid, e := ref(rec.Args[1])
				if e != nil {
					return fail()
				}
				p, e := splitTriple(f, pid, "CARTESIAN_POINT")
				if e != nil || p != points[0] {
					return fail()
				}
				vid, e := ref(rec.Args[2])
				if e != nil {
					return fail()
				}
				v, e := simple(f, vid, "VECTOR", 3)
				if e != nil {
					return fail()
				}
				did, e := ref(v.Args[1])
				if e != nil {
					return fail()
				}
				d, e := splitTriple(f, did, "DIRECTION")
				if e != nil || (d != (geom.Vec3{Z: 1}) && d != (geom.Vec3{Z: -1})) || curveSense != (d.Z > 0) {
					return fail()
				}
				mag, e := cylinderSupportNumber(v.Args[2])
				if e != nil || mag <= 0 {
					return fail()
				}
				role = 2*cells + vertices[a]
			} else {
				return fail()
			}
			if role < 0 || role >= 3*cells || out.Edges[role] != 0 {
				return fail()
			}
			out.Edges[role] = u.id
			edges[u.id] = role
			edgeEnds[u.id] = [2]int64{a, b}
		}
	}
	if len(vertices) != 2*cells || len(edges) != 3*cells {
		return fail()
	}
	roles := map[int]bool{}
	for _, r := range vertices {
		if roles[r] {
			return fail()
		}
		roles[r] = true
	}
	expected := cylinderCellLoops(cells)
	// Values encode edge role+1 and directed coedge sense. Cyclic rotation is free.
	for _, face := range fs {
		seq := []int{}
		for i, u := range face.uses {
			r := edges[u.id] + 1
			if !u.sense {
				r = -r
			}
			seq = append(seq, r)
			a := edgeEnds[u.id]
			if !u.sense {
				a[0], a[1] = a[1], a[0]
			}
			next := face.uses[(i+1)%len(face.uses)]
			b := edgeEnds[next.id]
			if !next.sense {
				b[0], b[1] = b[1], b[0]
			}
			if a[1] != b[0] {
				return fail()
			}
		}
		matched := -1
		for k, ex := range expected {
			if len(ex) != len(seq) {
				continue
			}
			for shift := range ex {
				ok := true
				for j := range ex {
					if seq[j] != ex[(j+shift)%len(ex)] {
						ok = false
					}
				}
				if ok {
					matched = k
				}
			}
		}
		if matched < 0 || out.Faces[matched] != 0 {
			return fail()
		}
		if matched < cells {
			if !face.side || !face.sense {
				return fail()
			}
		} else {
			if face.side || face.sense != (matched == cells+1) || face.z != []float64{0, out.Height}[matched-cells] {
				return fail()
			}
		}
		out.Faces[matched] = face.id
	}
	for _, id := range out.Faces {
		if id == 0 {
			return fail()
		}
	}
	out.Solid, e = parametric.Cylinder(sourceFrame, out.Radius, out.Height)
	if e != nil {
		return fail()
	}
	return out, nil
}
func splitTriple(f *File, id int64, kind string) (geom.Vec3, error) {
	r, e := simple(f, id, kind, 2)
	if e != nil || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != 3 {
		return geom.Vec3{}, splitFailure()
	}
	a := [3]float64{}
	for i, v := range r.Args[1].Items {
		a[i], e = cylinderSupportNumber(v)
		if e != nil {
			return geom.Vec3{}, e
		}
	}
	return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, nil
}
func splitPlacement(f *File, id int64) (float64, error) {
	r, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return 0, e
	}
	p, e := ref(r.Args[1])
	if e != nil {
		return 0, e
	}
	v, e := splitTriple(f, p, "CARTESIAN_POINT")
	if e != nil || v.X != 0 || v.Y != 0 {
		return 0, splitFailure()
	}
	for i, w := range []geom.Vec3{{Z: 1}, {X: 1}} {
		d, e := ref(r.Args[i+2])
		if e != nil {
			return 0, e
		}
		x, e := splitTriple(f, d, "DIRECTION")
		if e != nil || x != w {
			return 0, splitFailure()
		}
	}
	return v.Z, nil
}

// splitNormalizeFrame changes only the private parsed geometry, preserving IDs.
// Every subtraction/dot product is rational; conversion must remain exact.
func splitNormalizeFrame(f *File) (geom.Frame, error) {
	var pid int64
	for _, id := range f.Order {
		e := f.Entities[id]
		if len(e.Records) == 1 && e.Records[0].Type == "CYLINDRICAL_SURFACE" {
			r, e := simple(f, id, "CYLINDRICAL_SURFACE", 3)
			if e != nil {
				return geom.Frame{}, e
			}
			pid, e = ref(r.Args[1])
			if e != nil {
				return geom.Frame{}, e
			}
			break
		}
	}
	r, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return geom.Frame{}, e
	}
	coords := [3]geom.Vec3{}
	for i, kind := range []string{"CARTESIAN_POINT", "DIRECTION", "DIRECTION"} {
		id, e := ref(r.Args[i+1])
		if e != nil {
			return geom.Frame{}, e
		}
		coords[i], e = splitTriple(f, id, kind)
		if e != nil {
			return geom.Frame{}, e
		}
	}
	origin, z, x := coords[0], coords[1], coords[2]
	if !cylinderSupportUnit(z) || !cylinderSupportUnit(x) || z.Dot(x) != 0 {
		return geom.Frame{}, splitFailure()
	}
	frame, e := geom.NewFrame(origin, x, z)
	if e != nil {
		return geom.Frame{}, e
	}
	axes := []geom.Vec3{x, z.Cross(x), z}
	for id, ent := range f.Entities {
		if len(ent.Records) != 1 {
			continue
		}
		rec := ent.Records[0]
		if rec.Type != "CARTESIAN_POINT" && rec.Type != "DIRECTION" {
			continue
		}
		v, e := splitTriple(f, id, rec.Type)
		if e != nil {
			return geom.Frame{}, e
		}
		in := []float64{v.X, v.Y, v.Z}
		org := []float64{origin.X, origin.Y, origin.Z}
		items := make([]Value, 3)
		for j, a := range axes {
			av := []float64{a.X, a.Y, a.Z}
			q := new(big.Rat)
			for k, sign := range av {
				if sign == 0 {
					continue
				}
				w := new(big.Rat).SetFloat64(in[k])
				if rec.Type == "CARTESIAN_POINT" {
					w.Sub(w, new(big.Rat).SetFloat64(org[k]))
				}
				if sign < 0 {
					w.Neg(w)
				}
				q.Add(q, w)
			}
			_, exact := q.Float64()
			if !exact {
				return geom.Frame{}, splitFailure()
			}
			text := q.FloatString(q.Denom().BitLen() - 1)
			if strings.Contains(text, ".") {
				text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
			}
			items[j] = Value{Kind: Number, Text: text}
			if _, e = cylinderSupportNumber(items[j]); e != nil {
				return geom.Frame{}, e
			}
		}
		rec.Args = append([]Value(nil), rec.Args...)
		rec.Args[1] = Value{Kind: Aggregate, Items: items}
		ent.Records = []Record{rec}
		f.Entities[id] = ent
	}
	return frame, nil
}

// Edge direction is kept distinct from its model curve parameter direction.
// Only circle normal/sense pairs whose physical direction is positive local
// azimuth enter the existing angular coverage proof. Lines remain physically forward.
func cylinderCellEdge(f *File, id int64) (int64, int64, int64, bool, error) {
	r, e := simple(f, id, "EDGE_CURVE", 5)
	if e != nil {
		return 0, 0, 0, false, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return 0, 0, 0, false, e
	}
	a, e := ref(r.Args[1])
	if e != nil {
		return 0, 0, 0, false, e
	}
	b, e := ref(r.Args[2])
	if e != nil {
		return 0, 0, 0, false, e
	}
	g, e := ref(r.Args[3])
	if e != nil {
		return 0, 0, 0, false, e
	}
	sense, e := boolean(r.Args[4])
	return a, b, g, sense, e
}
func cylinderCellCirclePlacement(f *File, id int64) (float64, int, error) {
	r, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return 0, 0, e
	}
	p, e := ref(r.Args[1])
	if e != nil {
		return 0, 0, e
	}
	v, e := splitTriple(f, p, "CARTESIAN_POINT")
	if e != nil || v.X != 0 || v.Y != 0 {
		return 0, 0, splitFailure()
	}
	axis, e := ref(r.Args[2])
	if e != nil {
		return 0, 0, e
	}
	z, e := splitTriple(f, axis, "DIRECTION")
	if e != nil || (z != (geom.Vec3{Z: 1}) && z != (geom.Vec3{Z: -1})) {
		return 0, 0, splitFailure()
	}
	reference, e := ref(r.Args[3])
	if e != nil {
		return 0, 0, e
	}
	x, e := splitTriple(f, reference, "DIRECTION")
	if e != nil || x != (geom.Vec3{X: 1}) {
		return 0, 0, splitFailure()
	}
	return v.Z, int(z.Z), nil
}

// A false bound orientation reverses the complete loop, not the face support.
// Normalize only this private proof view; original source topology is retained.
func cylinderCellFaceLoop(f *File, id int64, n int) (Record, []analyticUse, error) {
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return Record{}, nil, e
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return Record{}, nil, splitFailure()
	}
	ent := f.Entities[bounds[0]]
	if len(ent.Records) != 1 {
		return Record{}, nil, splitFailure()
	}
	kind := ent.Records[0].Type
	if kind != "FACE_BOUND" && kind != "FACE_OUTER_BOUND" {
		return Record{}, nil, splitFailure()
	}
	bound, e := simple(f, bounds[0], kind, 3)
	if e != nil {
		return Record{}, nil, e
	}
	forward, e := boolean(bound.Args[2])
	if e != nil {
		return Record{}, nil, e
	}
	if forward {
		return analyticFaceLoop(f, id, n)
	}
	local := *f
	local.Entities = make(map[int64]Entity, len(f.Entities))
	for k, v := range f.Entities {
		local.Entities[k] = v
	}
	bound.Args = append([]Value(nil), bound.Args...)
	bound.Args[2] = Value{Kind: Enumeration, Text: "T"}
	ent.Records = []Record{bound}
	local.Entities[bounds[0]] = ent
	rec, uses, e := analyticFaceLoop(&local, id, n)
	if e != nil {
		return Record{}, nil, e
	}
	for i, j := 0, len(uses)-1; i < j; i, j = i+1, j-1 {
		uses[i], uses[j] = uses[j], uses[i]
	}
	for i := range uses {
		uses[i].sense = !uses[i].sense
	}
	return rec, uses, nil
}
