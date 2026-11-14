package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// MappedPlanarOccurrence retains an explicit mapped item, not a product occurrence.
type MappedPlanarOccurrence struct {
	Name             string
	ItemID, TargetID int64
	Transform        geom.Affine
}

// MappedPlanarMaterial keeps one original planar body and explicitly placed uses.
// Mapping origin is identity; local model and per-owner pcurves remain unchanged.
type MappedPlanarMaterial struct {
	Name                                  string
	Body                                  Body
	LengthUnit                            float64
	Source                                *File
	SourceBodyID, MapID, RepresentationID int64
	Occurrences                           []MappedPlanarOccurrence
}

func mappedPlanarFailure() error { return fmt.Errorf("step: unsupported exact mapped planar material") }
func mappedExactCoordinate(f *File, id int64, kind string, dim int) (geom.Vec3, error) {
	r, e := simple(f, id, kind, 2)
	if e != nil {
		return geom.Vec3{}, e
	}
	if r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != dim {
		return geom.Vec3{}, mappedPlanarFailure()
	}
	var v [3]float64
	for i, x := range r.Args[1].Items {
		v[i], e = cylinderSupportNumber(x)
		if e != nil {
			return geom.Vec3{}, e
		}
	}
	return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
}
func mappedExactFrame(f *File, id int64) (geom.Frame, error) {
	r, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return geom.Frame{}, e
	}
	p, e := ref(r.Args[1])
	if e != nil {
		return geom.Frame{}, e
	}
	o, e := mappedExactCoordinate(f, p, "CARTESIAN_POINT", 3)
	if e != nil {
		return geom.Frame{}, e
	}
	zi, e := ref(r.Args[2])
	if e != nil {
		return geom.Frame{}, e
	}
	xi, e := ref(r.Args[3])
	if e != nil {
		return geom.Frame{}, e
	}
	z, e := mappedExactCoordinate(f, zi, "DIRECTION", 3)
	if e != nil {
		return geom.Frame{}, e
	}
	x, e := mappedExactCoordinate(f, xi, "DIRECTION", 3)
	if e != nil || !cylinderSupportUnit(x) || !cylinderSupportUnit(z) || x.Dot(z) != 0 {
		return geom.Frame{}, mappedPlanarFailure()
	}
	return geom.NewFrame(o, x, z)
}
func mappedRatVec(v geom.Vec3) [3]*big.Rat {
	return [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}
func mappedLineExact(f *File, id int64, dim int, frame *geom.Frame, points [2]geom.Vec3) error {
	r, e := simple(f, id, "LINE", 3)
	if e != nil {
		return e
	}
	pid, e := ref(r.Args[1])
	if e != nil {
		return e
	}
	p, e := mappedExactCoordinate(f, pid, "CARTESIAN_POINT", dim)
	if e != nil {
		return e
	}
	vid, e := ref(r.Args[2])
	if e != nil {
		return e
	}
	v, e := simple(f, vid, "VECTOR", 3)
	if e != nil {
		return e
	}
	m, e := cylinderSupportNumber(v.Args[2])
	if e != nil || m <= 0 {
		return mappedPlanarFailure()
	}
	did, e := ref(v.Args[1])
	if e != nil {
		return e
	}
	d, e := mappedExactCoordinate(f, did, "DIRECTION", dim)
	if e != nil || !cylinderSupportUnit(d) {
		return mappedPlanarFailure()
	}
	origin := mappedRatVec(p)
	if frame != nil {
		origin = mappedRatVec(frame.Origin())
		x, y := mappedRatVec(frame.XAxis()), mappedRatVec(frame.YAxis())
		for i := range origin {
			origin[i].Add(origin[i], new(big.Rat).Mul(x[i], new(big.Rat).SetFloat64(p.X)))
			origin[i].Add(origin[i], new(big.Rat).Mul(y[i], new(big.Rat).SetFloat64(p.Y)))
		}
		d = frame.XAxis().Scale(d.X).Add(frame.YAxis().Scale(d.Y))
	}
	for _, point := range points {
		q := mappedRatVec(point)
		for i, a := range []float64{d.X, d.Y, d.Z} {
			if a == 0 && q[i].Cmp(origin[i]) != 0 {
				return mappedPlanarFailure()
			}
		}
	}
	return nil
}
func mappedPlanarExactEdges(f *File, bodyID int64) error {
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
		if _, e = mappedExactFrame(f, plane.Args[1].Ref); e != nil {
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
		if e = mappedLineExact(f, s.Args[1].Ref, 3, nil, points); e != nil {
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
			frame, e := mappedExactFrame(f, plane.Args[1].Ref)
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
			if e = mappedLineExact(f, items[0], 2, &frame, points); e != nil {
				return e
			}
		}
	}
	return nil
}

// ReadMappedPlanarMaterial supports one original cardinal-plane closed body,
// one identity representation map, and up to16 explicit proper unit-scale rigid
// targets in the observed AP214 eight-argument operator form. No NAUO assembly,
// nested mapping, implicit axes, reflection, scaling or mixed units is inferred.
func ReadMappedPlanarMaterial(r io.Reader, l Limits) (MappedPlanarMaterial, error) {
	fail := func() (MappedPlanarMaterial, error) { return MappedPlanarMaterial{}, mappedPlanarFailure() }
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return MappedPlanarMaterial{}, e
	}
	types := map[string][]int64{}
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			types[rec.Type] = append(types[rec.Type], id)
		}
	}
	if len(types["REPRESENTATION_MAP"]) != 1 || len(types["SHAPE_REPRESENTATION"]) != 1 || len(types["ADVANCED_BREP_SHAPE_REPRESENTATION"]) != 1 || len(types["MANIFOLD_SOLID_BREP"]) != 1 || len(types["MAPPED_ITEM"]) < 1 || len(types["MAPPED_ITEM"]) > min(16, l.MaxEdgeUses) || len(types["NEXT_ASSEMBLY_USAGE_OCCURRENCE"]) != 0 {
		return fail()
	}
	rootID, mapID, bodyID := types["SHAPE_REPRESENTATION"][0], types["REPRESENTATION_MAP"][0], types["MANIFOLD_SOLID_BREP"][0]
	root, e := simple(f, rootID, "SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	name, e := textValue(root.Args[0])
	if e != nil {
		return fail()
	}
	unit, e := contextUnits(f, root.Args[2].Ref)
	if e != nil {
		return fail()
	}
	mapping, e := simple(f, mapID, "REPRESENTATION_MAP", 2)
	if e != nil {
		return fail()
	}
	origin, e := mappedExactFrame(f, mapping.Args[0].Ref)
	if e != nil || frameTransform(origin) != geom.Identity() {
		return fail()
	}
	localID := mapping.Args[1].Ref
	if localID != types["ADVANCED_BREP_SHAPE_REPRESENTATION"][0] {
		return fail()
	}
	local, e := simple(f, localID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	u, e := contextUnits(f, local.Args[2].Ref)
	if e != nil || u != unit || local.Args[2].Ref != root.Args[2].Ref {
		return fail()
	}
	items, e := listRefs(local.Args[1], 1)
	if e != nil || len(items) != 2 || items[0] != mapping.Args[0].Ref || items[1] != bodyID {
		return fail()
	}
	uses, e := listRefs(root.Args[1], 1)
	if e != nil || len(uses) != len(types["MAPPED_ITEM"]) {
		return fail()
	}
	out := MappedPlanarMaterial{Name: name, LengthUnit: unit, Source: f, MapID: mapID, SourceBodyID: bodyID, RepresentationID: rootID}
	seen := map[int64]bool{}
	for _, id := range uses {
		if seen[id] {
			return fail()
		}
		seen[id] = true
		item, e := simple(f, id, "MAPPED_ITEM", 3)
		if e != nil || item.Args[1].Ref != mapID {
			return fail()
		}
		name, e := textValue(item.Args[0])
		if e != nil {
			return fail()
		}
		target := item.Args[2].Ref
		op, e := simple(f, target, "CARTESIAN_TRANSFORMATION_OPERATOR_3D", 8)
		if e != nil {
			return fail()
		}
		for i := 0; i < 3; i++ {
			s, e := textValue(op.Args[i])
			if e != nil || s != "" {
				return fail()
			}
		}
		scale, e := cylinderSupportNumber(op.Args[6])
		if e != nil || scale != 1 {
			return fail()
		}
		var axes [3]geom.Vec3
		for j, k := range []int{3, 4, 7} {
			axes[j], e = mappedExactCoordinate(f, op.Args[k].Ref, "DIRECTION", 3)
			if e != nil || !cylinderSupportUnit(axes[j]) {
				return fail()
			}
		}
		if axes[0].Cross(axes[1]) != axes[2] {
			return fail()
		}
		p, e := mappedExactCoordinate(f, op.Args[5].Ref, "CARTESIAN_POINT", 3)
		if e != nil {
			return fail()
		}
		m := geom.Affine{{axes[0].X, axes[1].X, axes[2].X, p.X}, {axes[0].Y, axes[1].Y, axes[2].Y, p.Y}, {axes[0].Z, axes[1].Z, axes[2].Z, p.Z}}
		out.Occurrences = append(out.Occurrences, MappedPlanarOccurrence{Name: name, ItemID: id, TargetID: target, Transform: m})
	}
	if e = mappedPlanarExactEdges(f, bodyID); e != nil {
		return fail()
	}
	body, e := readBody(f, bodyID, l.MaxVertices, l.MaxFaces, l.MaxEdgeUses, true)
	if e != nil {
		return fail()
	}
	if e = poly.ValidateEmbeddedBoundary(context.Background(), body.Solid, poly.Options{}); e != nil {
		return fail()
	}
	out.Body = body
	// Every parsed entity must be reachable from the explicit root/product records;
	// this rejects detached geometry and ignored alternate operator graphs.
	reached := map[int64]bool{}
	var walk func(int64, int) error
	walk = func(id int64, depth int) error {
		if depth > l.MaxDepth {
			return mappedPlanarFailure()
		}
		if reached[id] {
			return nil
		}
		en, ok := f.Entities[id]
		if !ok {
			return mappedPlanarFailure()
		}
		reached[id] = true
		var value func(Value) error
		value = func(v Value) error {
			if v.Kind == Reference {
				return walk(v.Ref, depth+1)
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
		return nil
	}

	if e = walk(rootID, 0); e != nil {
		return fail()
	}
	metadata := map[string]bool{"SHAPE_DEFINITION_REPRESENTATION": true, "PRODUCT_DEFINITION_SHAPE": true, "PRODUCT_DEFINITION": true, "PRODUCT_DEFINITION_FORMATION": true, "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE": true, "PRODUCT": true, "PRODUCT_CONTEXT": true, "PRODUCT_DEFINITION_CONTEXT": true, "APPLICATION_CONTEXT": true, "APPLICATION_PROTOCOL_DEFINITION": true, "PRODUCT_RELATED_PRODUCT_CATEGORY": true}
	for id, en := range f.Entities {
		if reached[id] {
			continue
		}
		for _, rec := range en.Records {
			if !metadata[rec.Type] {
				return fail()
			}
		}
	}
	if len(types["SHAPE_DEFINITION_REPRESENTATION"]) != 1 {
		return fail()
	}
	sd, e := simple(f, types["SHAPE_DEFINITION_REPRESENTATION"][0], "SHAPE_DEFINITION_REPRESENTATION", 2)
	if e != nil || sd.Args[1].Ref != rootID {
		return fail()
	}
	for _, kind := range []string{"SHAPE_DEFINITION_REPRESENTATION", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_RELATED_PRODUCT_CATEGORY"} {
		for _, id := range types[kind] {
			if e = walk(id, 0); e != nil {
				return fail()
			}
		}
	}
	if len(reached) != len(f.Entities) {
		return fail()
	}
	return out, nil
}

// WriteMappedPlanarMaterialSource revalidates and preserves an authored mapped
// graph. It is source write-back, not a writer for edited cached native geometry.
func WriteMappedPlanarMaterialSource(w io.Writer, source *File) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e := ReadMappedPlanarMaterial(bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

func mappedPlanarSourceBytes(source *File) ([]byte, error) {
	var staged bytes.Buffer
	staged.WriteString("ISO-10303-21;\nHEADER;\n")
	identity := map[int64]int64{}
	for id := range source.Entities {
		identity[id] = id
	}
	for _, r := range source.Header {
		staged.WriteString(assemblyEntity(Entity{Records: []Record{r}}, identity) + ";\n")
	}
	staged.WriteString("ENDSEC;\nDATA;\n")
	seen := map[int64]bool{}
	for _, id := range source.Order {
		en, ok := source.Entities[id]
		if !ok || id <= 0 || seen[id] {
			return nil, mappedPlanarFailure()
		}
		seen[id] = true
		fmt.Fprintf(&staged, "#%d=%s;\n", id, assemblyEntity(en, identity))
	}
	if len(seen) != len(source.Entities) {
		return nil, mappedPlanarFailure()
	}
	staged.WriteString("ENDSEC;END-ISO-10303-21;\n")
	return staged.Bytes(), nil
}
