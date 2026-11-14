package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// ExactNestedNAUOOccurrence is one expanded root-to-leaf use. Path preserves
// both original relationship records; the same inner record can occur on
// multiple paths. The world pose is immutable and composed exactly.
type ExactNestedNAUOOccurrence struct {
	Path [2]ExactNAUOOccurrence
	pose mappedRationalBasis
}

func (o ExactNestedNAUOOccurrence) Matrix() [3][4]*big.Rat {
	return (ExactMappedOccurrence{pose: o.pose}).Matrix()
}
func (o ExactNestedNAUOOccurrence) ApplyExact(p geom.Vec3) ([3]*big.Rat, error) {
	return (ExactMappedOccurrence{pose: o.pose}).ApplyExact(p)
}
func (o ExactNestedNAUOOccurrence) ApplyRepresented(p geom.Vec3) (geom.Vec3, error) {
	return (ExactMappedOccurrence{pose: o.pose}).ApplyRepresented(p)
}

// ExactNestedPlanarNAUO retains a root, one shared intermediate group, and local planar products.
type ExactNestedPlanarNAUO struct {
	Source        *File
	Root, Group   AssemblyProduct
	Parts         []ExactNAUOPlanarProduct
	LengthUnit    float64
	Relationships []ExactNAUOOccurrence
	Occurrences   []ExactNestedNAUOOccurrence
}

// ReadExactNestedPlanarNAUO proves one intermediate group used twice by the root,
// with 1–6 cardinal planar products and at most 16 direct uses beneath the group.
func ReadExactNestedPlanarNAUO(r io.Reader, l Limits) (ExactNestedPlanarNAUO, error) {
	fail := func() (ExactNestedPlanarNAUO, error) { return ExactNestedPlanarNAUO{}, mappedPlanarFailure() }
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return ExactNestedPlanarNAUO{}, e
	}
	if len(a.Products) < 3 || len(a.Products) > 8 || len(a.Roots) != 1 || len(a.Occurrences) < 3 || len(a.Occurrences) > 18 {
		return fail()
	}
	f := a.Source
	out := ExactNestedPlanarNAUO{Source: f, LengthUnit: a.LengthUnit}
	parts := map[int64]bool{}
	var rootRep, groupRep Record
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
			en := f.Entities[p.Representations[0]]
			if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_REPRESENTATION" {
				if out.Group.DefinitionID != 0 {
					return fail()
				}
				out.Group = p
				groupRep, e = simple(f, p.Representations[0], "SHAPE_REPRESENTATION", 3)
				if e != nil {
					return fail()
				}
			} else {
				parts[p.DefinitionID] = true
			}
		}
	}
	if out.Group.DefinitionID == 0 || len(parts) == 0 {
		return fail()
	}
	prefix, e := nauoSIPrefix(f, rootRep.Args[2].Ref)
	if e != nil {
		return fail()
	}
	gp, e := nauoSIPrefix(f, groupRep.Args[2].Ref)
	if e != nil || gp != prefix {
		return fail()
	}
	allowedRoots := []int64{}
	reps := []Record{rootRep, groupRep}
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
	rootUses, groupUses := 0, 0
	for _, u := range a.Occurrences {
		if u.Parent == out.Root.DefinitionID && u.Child == out.Group.DefinitionID {
			rootUses++
		} else if u.Parent == out.Group.DefinitionID && parts[u.Child] {
			groupUses++
			used[u.Child] = true
		} else {
			return fail()
		}
		occurrences[u.ID] = u
	}
	if len(used) != len(parts) || rootUses != 2 || groupUses < 1 || groupUses > 16 {
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
			out.Relationships = append(out.Relationships, ExactNAUOOccurrence{Name: u.Name, NAUOID: oid, CDSRID: id, RelationshipID: relID, TransformationID: tid, FromPlacementID: fromID, ToPlacementID: toID, ParentID: u.Parent, ChildID: u.Child, pose: composeMappedPose(to, inverseMappedPose(from))})
		}
	}
	if len(linked) != len(a.Occurrences) {
		return fail()
	}
	all, e := nauoClosure(f, roots, lim.MaxDepth)
	if e != nil || len(all) != len(f.Entities) {
		return fail()
	}
	for _, outer := range out.Relationships {
		if outer.ParentID != out.Root.DefinitionID {
			continue
		}
		for _, inner := range out.Relationships {
			if inner.ParentID != out.Group.DefinitionID {
				continue
			}
			out.Occurrences = append(out.Occurrences, ExactNestedNAUOOccurrence{Path: [2]ExactNAUOOccurrence{outer, inner}, pose: composeMappedPose(outer.pose, inner.pose)})
		}
	}
	return out, nil
}
func WriteExactNestedPlanarNAUOSource(w io.Writer, source *File, l Limits) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadExactNestedPlanarNAUO(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
