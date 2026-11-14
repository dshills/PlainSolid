package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/csg/poly"
)

// ExactNAUOPlanarProduct retains one source-local body and its original product.
type ExactNAUOPlanarProduct struct {
	Product      AssemblyProduct
	Body         Body
	SourceBodyID int64
}

// ExactPlanarNAUOProducts is a flat assembly, not an inferred mapped hierarchy.
type ExactPlanarNAUOProducts struct {
	Source      *File
	Root        AssemblyProduct
	Parts       []ExactNAUOPlanarProduct
	LengthUnit  float64
	Occurrences []ExactNAUOOccurrence
}

// ReadExactPlanarNAUOProducts proves a flat root with 2–7 independently owned
// cardinal planar products and up to 16 original NAUO occurrences.
func ReadExactPlanarNAUOProducts(r io.Reader, l Limits) (ExactPlanarNAUOProducts, error) {
	fail := func() (ExactPlanarNAUOProducts, error) { return ExactPlanarNAUOProducts{}, mappedPlanarFailure() }
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return ExactPlanarNAUOProducts{}, e
	}
	if len(a.Products) < 3 || len(a.Products) > 8 || len(a.Roots) != 1 || len(a.Occurrences) < 2 || len(a.Occurrences) > 16 {
		return fail()
	}
	f := a.Source
	out := ExactPlanarNAUOProducts{Source: f, LengthUnit: a.LengthUnit}
	parts := map[int64]bool{}
	var rootRep Record
	for _, p := range a.Products {
		if len(p.Representations) != 1 {
			return fail()
		}
		if p.DefinitionID == a.Roots[0] {
			out.Root = p
			rootRep, e = simple(f, p.Representations[0], "SHAPE_REPRESENTATION", 3)
			if e != nil {
				return fail()
			}
		} else {
			parts[p.DefinitionID] = true
		}
	}
	prefix, e := nauoSIPrefix(f, rootRep.Args[2].Ref)
	if e != nil {
		return fail()
	}
	allowedRoots := []int64{}
	reps := []Record{rootRep}
	owned := map[int64]int64{}
	nv, nf, nu := 0, 0, 0
	for _, p := range a.Products {
		if !parts[p.DefinitionID] {
			continue
		}
		rep, e := simple(f, p.Representations[0], "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		u, e := nauoSIPrefix(f, rep.Args[2].Ref)
		if e != nil || u != prefix {
			return fail()
		}
		ids, advanced, e := a.productItems(p.DefinitionID)
		if e != nil || len(ids) != 1 || !advanced[0] {
			return fail()
		}
		if e = mappedPlanarExactEdges(f, ids[0]); e != nil {
			return fail()
		}
		closure, e := nauoClosure(f, ids, lim.MaxDepth)
		if e != nil {
			return fail()
		}
		for id := range closure {
			for _, r := range f.Entities[id].Records {
				if planarSplineOwnedType(r.Type) || r.Type == "CARTESIAN_POINT" || r.Type == "DIRECTION" || r.Type == "VECTOR" || r.Type == "AXIS2_PLACEMENT_3D" {
					if owner := owned[id]; owner != 0 && owner != p.DefinitionID {
						return fail()
					}
					owned[id] = p.DefinitionID
				}
			}
		}
		body, e := readBody(f, ids[0], lim.MaxVertices, lim.MaxFaces, lim.MaxEdgeUses, true)
		if e != nil {
			return fail()
		}
		nv += len(body.Solid.Vertices())
		nf += len(body.Solid.Faces())
		for _, face := range body.Solid.Faces() {
			nu += len(face.Boundary)
			for _, h := range face.Holes {
				nu += len(h)
			}
		}
		if nv > lim.MaxVertices || nf > lim.MaxFaces || nu > lim.MaxEdgeUses {
			return fail()
		}
		if e = poly.ValidateEmbeddedBoundary(context.Background(), body.Solid, poly.Options{}); e != nil {
			return fail()
		}
		out.Parts = append(out.Parts, ExactNAUOPlanarProduct{Product: p, Body: body, SourceBodyID: ids[0]})
		allowedRoots = append(allowedRoots, ids[0])
		reps = append(reps, rep)
	}
	for _, rep := range reps {
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		for _, id := range items {
			en := f.Entities[id]
			if len(en.Records) == 1 && en.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				if _, e = mappedRationalFrame(f, id); e != nil {
					return fail()
				}
				allowedRoots = append(allowedRoots, id)
			}
		}
	}
	geometry, e := nauoClosure(f, allowedRoots, lim.MaxDepth)
	if e != nil {
		return fail()
	}
	used := map[int64]bool{}
	occurrences := map[int64]AssemblyOccurrence{}
	for _, u := range a.Occurrences {
		if u.Parent != out.Root.DefinitionID || !parts[u.Child] {
			return fail()
		}
		occurrences[u.ID] = u
		used[u.Child] = true
	}
	if len(used) != len(parts) {
		return fail()
	}
	roots := []int64{}
	linked := map[int64]bool{}
	relSeen, trSeen := map[int64]bool{}, map[int64]bool{}
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, rec := range en.Records {
			if (planarSplineOwnedType(rec.Type) || rec.Type == "CARTESIAN_POINT" || rec.Type == "DIRECTION" || rec.Type == "VECTOR" || rec.Type == "AXIS2_PLACEMENT_3D") && !geometry[id] {
				return fail()
			}
		}
		if len(en.Records) != 1 {
			continue
		}
		rec := en.Records[0]
		switch rec.Type {
		case "SHAPE_DEFINITION_REPRESENTATION", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_RELATED_PRODUCT_CATEGORY":
			roots = append(roots, id)
		case "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION":
			roots = append(roots, id)
			if len(rec.Args) != 2 {
				return fail()
			}
			pds, e := simple(f, rec.Args[1].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
			if e != nil {
				return fail()
			}
			oid := pds.Args[2].Ref
			u, ok := occurrences[oid]
			if !ok || linked[oid] {
				return fail()
			}
			linked[oid] = true
			relID := rec.Args[0].Ref
			var tid int64
			for _, v := range f.Entities[relID].Records {
				if v.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
					tid = v.Args[0].Ref
				}
			}
			if relSeen[relID] || trSeen[tid] {
				return fail()
			}
			relSeen[relID] = true
			trSeen[tid] = true
			tr, e := simple(f, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
			if e != nil {
				return fail()
			}
			fromID, toID := tr.Args[2].Ref, tr.Args[3].Ref
			from, e := mappedRationalFrame(f, fromID)
			if e != nil {
				return fail()
			}
			to, e := mappedRationalFrame(f, toID)
			if e != nil {
				return fail()
			}
			out.Occurrences = append(out.Occurrences, ExactNAUOOccurrence{Name: u.Name, NAUOID: oid, CDSRID: id, RelationshipID: relID, TransformationID: tid, FromPlacementID: fromID, ToPlacementID: toID, ParentID: u.Parent, ChildID: u.Child, pose: composeMappedPose(to, inverseMappedPose(from))})
		}
	}
	if len(linked) != len(a.Occurrences) {
		return fail()
	}
	all, e := nauoClosure(f, roots, lim.MaxDepth)
	if e != nil || len(all) != len(f.Entities) {
		return fail()
	}
	return out, nil
}
func WriteExactPlanarNAUOProductsSource(w io.Writer, source *File, l Limits) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadExactPlanarNAUOProducts(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
