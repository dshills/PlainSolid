package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// ExactDAGNAUOOccurrence is one expanded root-to-leaf use. Path preserves
// every original relationship record; the same inner record can occur on
// multiple paths. The world pose is immutable and composed exactly.
type ExactDAGNAUOOccurrence struct {
	Path []ExactNAUOOccurrence
	pose mappedRationalBasis
}

func (o ExactDAGNAUOOccurrence) Matrix() [3][4]*big.Rat {
	return (ExactMappedOccurrence{pose: o.pose}).Matrix()
}
func (o ExactDAGNAUOOccurrence) ApplyExact(p geom.Vec3) ([3]*big.Rat, error) {
	return (ExactMappedOccurrence{pose: o.pose}).ApplyExact(p)
}
func (o ExactDAGNAUOOccurrence) ApplyRepresented(p geom.Vec3) (geom.Vec3, error) {
	return (ExactMappedOccurrence{pose: o.pose}).ApplyRepresented(p)
}

// ExactDAGPlanarNAUO retains a bounded original product DAG and source-local bodies.
type ExactDAGPlanarNAUO struct {
	Source        *File
	Root          AssemblyProduct
	Groups        []AssemblyProduct
	Parts         []ExactNAUOPlanarProduct
	LengthUnit    float64
	Relationships []ExactNAUOOccurrence
	Occurrences   []ExactDAGNAUOOccurrence
}

// ReadExactDAGPlanarNAUO proves a bounded DAG with at least two intermediate groups.
// It permits at most 8 products, 32 direct uses, 4 relationship edges per path,
// and 64 expanded leaf uses. Every leaf has an independently owned exact planar body.
func ReadExactDAGPlanarNAUO(r io.Reader, l Limits) (ExactDAGPlanarNAUO, error) {
	fail := func() (ExactDAGPlanarNAUO, error) { return ExactDAGPlanarNAUO{}, mappedPlanarFailure() }
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return ExactDAGPlanarNAUO{}, e
	}
	if len(a.Products) < 4 || len(a.Products) > 8 || len(a.Roots) != 1 || len(a.Occurrences) < 3 || len(a.Occurrences) > 32 {
		return fail()
	}
	f := a.Source
	out := ExactDAGPlanarNAUO{Source: f, LengthUnit: a.LengthUnit}
	parts := map[int64]bool{}
	var rootRep Record
	groupReps := []Record{}
	groups := map[int64]bool{}
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
				groups[p.DefinitionID] = true
				out.Groups = append(out.Groups, p)
				rep, e := simple(f, p.Representations[0], "SHAPE_REPRESENTATION", 3)
				if e != nil {
					return fail()
				}
				groupReps = append(groupReps, rep)
			} else {
				parts[p.DefinitionID] = true
			}
		}
	}
	if len(groups) < 2 || len(parts) == 0 {
		return fail()
	}
	prefix, e := nauoSIPrefix(f, rootRep.Args[2].Ref)
	if e != nil {
		return fail()
	}
	for _, rep := range groupReps {
		gp, e := nauoSIPrefix(f, rep.Args[2].Ref)
		if e != nil || gp != prefix {
			return fail()
		}
	}
	allowedRoots := []int64{}
	reps := append([]Record{rootRep}, groupReps...)
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
	occurrences := map[int64]AssemblyOccurrence{}
	incoming, outgoing := map[int64]int{}, map[int64]int{}
	for _, u := range a.Occurrences {
		if u.Parent != out.Root.DefinitionID && !groups[u.Parent] {
			return fail()
		}
		if !groups[u.Child] && !parts[u.Child] {
			return fail()
		}
		occurrences[u.ID] = u
		incoming[u.Child]++
		outgoing[u.Parent]++
	}
	for _, p := range a.Products {
		if p.DefinitionID != out.Root.DefinitionID && incoming[p.DefinitionID] == 0 {
			return fail()
		}
		if (p.DefinitionID == out.Root.DefinitionID || groups[p.DefinitionID]) && outgoing[p.DefinitionID] == 0 {
			return fail()
		}
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
	children := map[int64][]ExactNAUOOccurrence{}
	for _, u := range out.Relationships {
		children[u.ParentID] = append(children[u.ParentID], u)
	}
	active := map[int64]bool{}
	reached := map[int64]bool{}
	var expand func(int64, []ExactNAUOOccurrence, mappedRationalBasis) error
	expand = func(id int64, path []ExactNAUOOccurrence, pose mappedRationalBasis) error {
		if len(path) > 4 || len(path) > lim.MaxDepth || active[id] {
			return mappedPlanarFailure()
		}
		reached[id] = true
		if parts[id] {
			if len(out.Occurrences) >= 64 {
				return mappedPlanarFailure()
			}
			out.Occurrences = append(out.Occurrences, ExactDAGNAUOOccurrence{Path: append([]ExactNAUOOccurrence(nil), path...), pose: pose})
			return nil
		}
		active[id] = true
		for _, u := range children[id] {
			next := u.pose
			if len(path) > 0 {
				next = composeMappedPose(pose, u.pose)
			}
			if e := expand(u.ChildID, append(path, u), next); e != nil {
				return e
			}
		}
		delete(active, id)
		return nil
	}
	if e = expand(out.Root.DefinitionID, nil, mappedRationalBasis{}); e != nil || len(reached) != len(a.Products) {
		return fail()
	}
	return out, nil
}
func WriteExactDAGPlanarNAUOSource(w io.Writer, source *File, l Limits) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadExactDAGPlanarNAUO(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
