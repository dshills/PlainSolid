package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
	"sort"
)

// WrappedRationalStationAssembly retains one independently proved weighted
// station product, shared by 2..16 flat proper signed-coordinate occurrences.
// Original wrapped supports and model/pcurve topology remain product-local.
type WrappedRationalStationAssembly struct {
	Assembly                         Assembly
	Product                          AssemblyProduct
	Body                             WrappedRationalStationLoftBody
	SourceBodyID                     int64
	SourceFaceIDs, SourceWallFaceIDs []int64
	Occurrences                      []ExactNAUOOccurrence
}
type WrappedRationalStationAssemblyPart struct {
	ID, Name string
	Body     *WrappedRationalStationLoftBody
}
type WrappedRationalStationAssemblyModel struct {
	Products    []WrappedRationalStationAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

func ReadWrappedRationalStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedRationalStationAssembly, error) {
	return readWeightedStationAssembly(ctx, r, l, o, func(raw []byte, l Limits, o WrappedSurfaceOptions) (WrappedRationalStationLoftBody, error) {
		return ReadWrappedRationalStationLoft(bytes.NewReader(raw), l, o)
	})
}

func readWeightedStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions, local func([]byte, Limits, WrappedSurfaceOptions) (WrappedRationalStationLoftBody, error)) (WrappedRationalStationAssembly, error) {
	var zero WrappedRationalStationAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	o, e = o.defaults()
	if e != nil {
		return zero, e
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return zero, e
	}
	if len(a.Products) != 2 || len(a.Roots) != 1 || len(a.Occurrences) < 2 || len(a.Occurrences) > 16 {
		return zero, geom.ErrInvalid
	}
	out := WrappedRationalStationAssembly{Assembly: a}
	f := a.Source
	var geometryRoots []int64
	var prefix string
	var havePrefix bool
	products := map[int64]bool{}
	for _, p := range a.Products {
		if len(p.Representations) != 1 {
			return zero, geom.ErrInvalid
		}
		products[p.ProductID] = true
		kind := "ADVANCED_BREP_SHAPE_REPRESENTATION"
		root := p.DefinitionID == a.Roots[0]
		if root {
			kind = "SHAPE_REPRESENTATION"
		}
		rep, e := simple(f, p.Representations[0], kind, 3)
		if e != nil {
			return zero, e
		}
		u, e := nauoSIPrefix(f, rep.Args[2].Ref)
		if e != nil || (u != "$" && u != "MILLI") || (havePrefix && prefix != u) {
			return zero, geom.ErrInvalid
		}
		prefix = u
		havePrefix = true
		ids, advanced, e := a.productItems(p.DefinitionID)
		if e != nil {
			return zero, e
		}
		if root {
			if len(ids) != 0 {
				return zero, geom.ErrInvalid
			}
		} else {
			if len(ids) != 1 || !advanced[0] {
				return zero, geom.ErrInvalid
			}
			out.Product = p
			out.SourceBodyID = ids[0]
			geometryRoots = append(geometryRoots, ids[0])
			closure, e := nauoClosure(f, []int64{p.Representations[0]}, l.MaxDepth)
			if e != nil {
				return zero, e
			}
			view := &File{Header: f.Header, Entities: map[int64]Entity{}}
			for _, id := range f.Order {
				if closure[id] {
					view.Order = append(view.Order, id)
					view.Entities[id] = f.Entities[id]
				}
			}
			raw, e := mappedPlanarSourceBytes(view)
			if e != nil {
				return zero, e
			}
			body, e := local(raw, l, o)
			if e != nil {
				return zero, e
			}
			if body.LengthUnit != a.LengthUnit {
				return zero, geom.ErrInvalid
			}
			out.Body = body
			out.SourceFaceIDs, e = bodyFaceIDs(f, ids[0])
			if e != nil {
				return zero, e
			}
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return zero, e
		}
		for _, id := range items {
			if _, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4); e == nil {
				frame, e := mappedExactFrame(f, id)
				if e != nil || !planarSplineRigid(frameTransform(frame)) {
					return zero, geom.ErrInvalid
				}
				geometryRoots = append(geometryRoots, id)
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	if out.SourceBodyID == 0 {
		return zero, geom.ErrInvalid
	}
	for _, wall := range out.Body.Walls {
		want := append([]int64(nil), wall.SideEdges[:]...)
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		var found int64
		for _, id := range out.SourceFaceIDs {
			uses, e := weightedAssemblyFaceUses(f, id)
			if e != nil {
				continue
			}
			got := []int64{}
			for _, u := range uses {
				got = append(got, u.id)
			}
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			same := len(got) == len(want)
			for i := range got {
				if got[i] != want[i] {
					same = false
				}
			}
			if same {
				if found != 0 {
					return zero, geom.ErrInvalid
				}
				found = id
			}
		}
		if found == 0 {
			return zero, geom.ErrInvalid
		}
		out.SourceWallFaceIDs = append(out.SourceWallFaceIDs, found)
	}
	geometry, e := nauoClosure(f, geometryRoots, l.MaxDepth)
	if e != nil {
		return zero, e
	}
	occurrences := map[int64]AssemblyOccurrence{}
	for _, u := range a.Occurrences {
		if u.Parent != a.Roots[0] || u.Child != out.Product.DefinitionID || !planarSplineRigid(u.Transform) {
			return zero, geom.ErrInvalid
		}
		occurrences[u.ID] = u
	}
	metadata := map[string]bool{}
	for _, k := range []string{"APPLICATION_CONTEXT", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_CONTEXT", "PRODUCT_DEFINITION_CONTEXT", "PRODUCT", "PRODUCT_DEFINITION_FORMATION", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_SHAPE", "PRODUCT_RELATED_PRODUCT_CATEGORY", "SHAPE_DEFINITION_REPRESENTATION", "SHAPE_REPRESENTATION", "ADVANCED_BREP_SHAPE_REPRESENTATION", "NEXT_ASSEMBLY_USAGE_OCCURRENCE", "ITEM_DEFINED_TRANSFORMATION", "REPRESENTATION_RELATIONSHIP", "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION", "SHAPE_REPRESENTATION_RELATIONSHIP", "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", "GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT", "REPRESENTATION_CONTEXT", "LENGTH_UNIT", "NAMED_UNIT", "SI_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT", "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT", "UNCERTAINTY_MEASURE_WITH_UNIT"} {
		metadata[k] = true
	}
	counts := map[string]int{}
	linked, relations, transforms, categories := map[int64]bool{}, map[int64]bool{}, map[int64]bool{}, map[int64]bool{}
	var roots []int64
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			counts[rec.Type]++
			if !geometry[id] && !metadata[rec.Type] {
				return zero, geom.ErrInvalid
			}
			switch rec.Type {
			case "APPLICATION_PROTOCOL_DEFINITION", "SHAPE_DEFINITION_REPRESENTATION":
				roots = append(roots, id)
			case "PRODUCT_RELATED_PRODUCT_CATEGORY":
				q, e := simple(f, id, rec.Type, 3)
				if e != nil {
					return zero, e
				}
				name, e := textValue(q.Args[0])
				if e != nil || name != "part" {
					return zero, geom.ErrInvalid
				}
				if q.Args[1].Kind != Omitted {
					if _, e = textValue(q.Args[1]); e != nil {
						return zero, e
					}
				}
				ids, e := listRefs(q.Args[2], 1)
				if e != nil || len(ids) != 1 || !products[ids[0]] || categories[ids[0]] {
					return zero, geom.ErrInvalid
				}
				categories[ids[0]] = true
				roots = append(roots, id)
			case "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION":
				roots = append(roots, id)
				cd, e := simple(f, id, rec.Type, 2)
				if e != nil {
					return zero, e
				}
				pds, e := simple(f, cd.Args[1].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
				if e != nil {
					return zero, e
				}
				oid := pds.Args[2].Ref
				u, ok := occurrences[oid]
				if !ok || linked[oid] {
					return zero, geom.ErrInvalid
				}
				linked[oid] = true
				rel := cd.Args[0].Ref
				var tid int64
				for _, r := range f.Entities[rel].Records {
					if r.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
						if len(r.Args) != 1 || tid != 0 {
							return zero, geom.ErrInvalid
						}
						tid = r.Args[0].Ref
					}
				}
				if relations[rel] || transforms[tid] {
					return zero, geom.ErrInvalid
				}
				relations[rel] = true
				transforms[tid] = true
				tr, e := simple(f, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
				if e != nil {
					return zero, e
				}
				from, e := mappedRationalFrame(f, tr.Args[2].Ref)
				if e != nil {
					return zero, e
				}
				to, e := mappedRationalFrame(f, tr.Args[3].Ref)
				if e != nil {
					return zero, e
				}
				neutral, e := independentTorusAssemblyPose(from)
				if e != nil || neutral != geom.Identity() {
					return zero, geom.ErrInvalid
				}
				pose := composeMappedPose(to, inverseMappedPose(from))
				exact, e := independentTorusAssemblyPose(pose)
				if e != nil || exact != u.Transform {
					return zero, geom.ErrInvalid
				}
				out.Occurrences = append(out.Occurrences, ExactNAUOOccurrence{Name: u.Name, NAUOID: oid, CDSRID: id, RelationshipID: rel, TransformationID: tid, FromPlacementID: tr.Args[2].Ref, ToPlacementID: tr.Args[3].Ref, ParentID: u.Parent, ChildID: u.Child, pose: pose})
			}
		}
	}
	if counts["APPLICATION_PROTOCOL_DEFINITION"] != 1 || counts["SHAPE_DEFINITION_REPRESENTATION"] != 2 || counts["CONTEXT_DEPENDENT_SHAPE_REPRESENTATION"] != len(a.Occurrences) || len(linked) != len(a.Occurrences) || (len(categories) != 0 && len(categories) != 2) {
		return zero, geom.ErrInvalid
	}
	all, e := nauoClosure(f, roots, l.MaxDepth)
	if e != nil || len(all) != len(f.Entities) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

// WriteWrappedRationalStationAssembly stages one weighted local body and a
// geometry-free root with shared flat uses. Cached native Solid is not authority.
func WriteWrappedRationalStationAssembly(ctx context.Context, w io.Writer, m WrappedRationalStationAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil || len(m.Products) != 2 || len(m.Occurrences) < 2 || len(m.Occurrences) > 16 {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	for _, p := range m.Products {
		if len(p.ID) > 32768 || len(p.Name) > 32768 {
			return geom.ErrInvalid
		}
		if p.Body != nil {
			if len(p.Body.Name) > 32768 || len(p.Body.CapNames[0]) > 32768 || len(p.Body.CapNames[1]) > 32768 || len(p.Body.Walls) > 16 {
				return geom.ErrInvalid
			}
			for _, wall := range p.Body.Walls {
				if len(wall.Name) > 32768 {
					return geom.ErrInvalid
				}
			}
		}
	}
	for _, u := range m.Occurrences {
		if len(u.ID) > 32768 || len(u.Name) > 32768 || len(u.Parent) > 32768 || len(u.Child) > 32768 {
			return geom.ErrInvalid
		}
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, 2)
	bodies := 0
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		if p.Body != nil {
			if p.Body.LengthUnit != m.LengthUnit {
				return geom.ErrInvalid
			}
			counts[i] = 1
			bodies++
		}
	}
	if bodies != 1 {
		return geom.ErrInvalid
	}
	for _, u := range m.Occurrences {
		if !planarSplineRigid(u.Transform) {
			return geom.ErrInvalid
		}
	}
	var staged bytes.Buffer
	e := writeAssemblyGeometry(&staged, graph, counts, func(i int) (*File, []int64, error) {
		var raw bytes.Buffer
		if e := WriteWrappedRationalStationLoft(&raw, *m.Products[i].Body); e != nil {
			return nil, nil, e
		}
		f, e := Parse(&raw, l)
		if e != nil {
			return nil, nil, e
		}
		var ids []int64
		for _, id := range f.Order {
			if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
				ids = append(ids, id)
			}
		}
		return f, ids, nil
	})
	if e != nil {
		return e
	}
	if _, e = ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(staged.Bytes()), l, o); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return e
}
func WriteWrappedRationalStationAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 8) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(raw), l, o); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// This maps identities only after the selected local material reader proved
// orientation. Inner bore bounds can be false; no source record is changed.
func weightedAssemblyFaceUses(f *File, id int64) ([]analyticUse, error) {
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return nil, e
	}
	ids, e := listRefs(face.Args[1], 1)
	if e != nil || len(ids) != 1 {
		return nil, geom.ErrInvalid
	}
	b, e := simple(f, ids[0], "FACE_BOUND", 3)
	if e != nil {
		b, e = simple(f, ids[0], "FACE_OUTER_BOUND", 3)
	}
	if e != nil {
		return nil, e
	}
	if _, e = boolean(b.Args[2]); e != nil {
		return nil, e
	}
	loop, e := simple(f, b.Args[1].Ref, "EDGE_LOOP", 2)
	if e != nil {
		return nil, e
	}
	edges, e := listRefs(loop.Args[1], 4)
	if e != nil || len(edges) != 4 {
		return nil, geom.ErrInvalid
	}
	var out []analyticUse
	for _, eid := range edges {
		u, e := simple(f, eid, "ORIENTED_EDGE", 5)
		if e != nil {
			return nil, e
		}
		sense, e := boolean(u.Args[4])
		if e != nil {
			return nil, e
		}
		out = append(out, analyticUse{u.Args[3].Ref, sense})
	}
	return out, nil
}
