package step

import (
	"context"
	"io"
	"plainsolid/geom"
)

// exactFlatSingleProductAssembly authenticates metadata, closed ownership and
// exact occurrence frames; the caller must authenticate its local body family.
type exactFlatSingleProductAssembly struct {
	Assembly     Assembly
	Product      AssemblyProduct
	SourceBodyID int64
	Occurrences  []ExactNAUOOccurrence
}

func readExactFlatSingleProductAssembly(ctx context.Context, r io.Reader, l Limits) (exactFlatSingleProductAssembly, error) {
	var zero exactFlatSingleProductAssembly
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
	a, e := ReadAssembly(r, l)
	if e != nil {
		return zero, e
	}
	if len(a.Products) != 2 || len(a.Roots) != 1 || len(a.Occurrences) < 2 || len(a.Occurrences) > 16 {
		return zero, geom.ErrInvalid
	}
	out := exactFlatSingleProductAssembly{Assembly: a}
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
