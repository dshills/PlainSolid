package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// ExactNAUOOccurrence retains original product/relationship ownership and an
// immutable exact pose. It is not a MAPPED_ITEM and has no invented map identity.
type ExactNAUOOccurrence struct {
	Name                                                                                                string
	NAUOID, CDSRID, RelationshipID, TransformationID, FromPlacementID, ToPlacementID, ParentID, ChildID int64
	pose                                                                                                mappedRationalBasis
}

func (o ExactNAUOOccurrence) Matrix() [3][4]*big.Rat {
	return (ExactMappedOccurrence{pose: o.pose}).Matrix()
}
func (o ExactNAUOOccurrence) ApplyExact(p geom.Vec3) ([3]*big.Rat, error) {
	return (ExactMappedOccurrence{pose: o.pose}).ApplyExact(p)
}
func (o ExactNAUOOccurrence) ApplyRepresented(p geom.Vec3) (geom.Vec3, error) {
	return (ExactMappedOccurrence{pose: o.pose}).ApplyRepresented(p)
}

type ExactPlanarNAUO struct {
	Source       *File
	Root, Part   AssemblyProduct
	Body         Body
	SourceBodyID int64
	LengthUnit   float64
	Occurrences  []ExactNAUOOccurrence
}

func inverseMappedPose(b mappedRationalBasis) (a mappedRationalBasis) {
	for i := 0; i < 3; i++ {
		a.X[i] = new(big.Rat).Set([]*big.Rat{b.X[0], b.Y[0], b.Z[0]}[i])
		a.Y[i] = new(big.Rat).Set([]*big.Rat{b.X[1], b.Y[1], b.Z[1]}[i])
		a.Z[i] = new(big.Rat).Set([]*big.Rat{b.X[2], b.Y[2], b.Z[2]}[i])
	}
	for i, v := range [][3]*big.Rat{b.X, b.Y, b.Z} {
		a.Origin[i] = new(big.Rat).Neg(mappedRationalDot(v, b.Origin))
	}
	return
}
func nauoClosure(f *File, roots []int64, maxDepth int) (map[int64]bool, error) {
	seen, active := map[int64]bool{}, map[int64]bool{}
	var walk func(int64, int) error
	walk = func(id int64, depth int) error {
		if depth > maxDepth || active[id] {
			return mappedPlanarFailure()
		}
		if seen[id] {
			return nil
		}
		en, ok := f.Entities[id]
		if !ok {
			return mappedPlanarFailure()
		}
		active[id] = true
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
		for _, r := range en.Records {
			for _, v := range r.Args {
				if e := value(v); e != nil {
					return e
				}
			}
		}
		delete(active, id)
		seen[id] = true
		return nil
	}
	for _, id := range roots {
		if e := walk(id, 0); e != nil {
			return nil, e
		}
	}
	return seen, nil
}
func nauoSIPrefix(f *File, ctx int64) (string, error) {
	en := f.Entities[ctx]
	unitID := int64(0)
	for _, r := range en.Records {
		if r.Type != "GLOBAL_UNIT_ASSIGNED_CONTEXT" {
			continue
		}
		if len(r.Args) != 1 {
			return "", mappedPlanarFailure()
		}
		ids, e := listRefs(r.Args[0], 1)
		if e != nil {
			return "", e
		}
		for _, id := range ids {
			for _, u := range f.Entities[id].Records {
				if u.Type == "LENGTH_UNIT" {
					if unitID != 0 {
						return "", mappedPlanarFailure()
					}
					unitID = id
				}
			}
		}
	}
	u := f.Entities[unitID]
	if len(u.Records) != 3 {
		return "", mappedPlanarFailure()
	}
	prefix := ""
	seen := map[string]bool{}
	for _, r := range u.Records {
		if seen[r.Type] {
			return "", mappedPlanarFailure()
		}
		seen[r.Type] = true
		switch r.Type {
		case "LENGTH_UNIT":
			if len(r.Args) != 0 {
				return "", mappedPlanarFailure()
			}
		case "NAMED_UNIT":
			if len(r.Args) != 1 || r.Args[0].Kind != Derived {
				return "", mappedPlanarFailure()
			}
		case "SI_UNIT":
			if len(r.Args) != 2 || r.Args[1].Kind != Enumeration || r.Args[1].Text != "METRE" {
				return "", mappedPlanarFailure()
			}
			if r.Args[0].Kind == Omitted {
				prefix = "$"
			} else if r.Args[0].Kind == Enumeration {
				prefix = r.Args[0].Text
			} else {
				return "", mappedPlanarFailure()
			}
		default:
			return "", mappedPlanarFailure()
		}
	}
	if prefix == "" {
		return "", mappedPlanarFailure()
	}
	return prefix, nil
}

// ReadExactPlanarNAUO proves one local cardinal planar part used twice by one root product
// through original NAUO/CDSR/representation relationships. Separate SI contexts
// must have the same exact prefix declaration. Model vertices, plane/owner lines
// and rational from/to placement composition are independently checked. No mapped
// items, nesting, extra bodies, detached geometry or approximate supports enter.
func ReadExactPlanarNAUO(r io.Reader, l Limits) (ExactPlanarNAUO, error) {
	fail := func() (ExactPlanarNAUO, error) { return ExactPlanarNAUO{}, mappedPlanarFailure() }
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return ExactPlanarNAUO{}, e
	}
	if len(a.Products) != 2 || len(a.Roots) != 1 || len(a.Occurrences) != 2 {
		return fail()
	}
	f := a.Source
	out := ExactPlanarNAUO{Source: f, LengthUnit: a.LengthUnit}
	for _, p := range a.Products {
		if len(p.Representations) != 1 {
			return fail()
		}
		if p.DefinitionID == a.Roots[0] {
			out.Root = p
		} else {
			out.Part = p
		}
	}
	if out.Part.DefinitionID == 0 {
		return fail()
	}
	rootRep, partRep := out.Root.Representations[0], out.Part.Representations[0]
	rr, e := simple(f, rootRep, "SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	pr, e := simple(f, partRep, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	p1, e := nauoSIPrefix(f, rr.Args[2].Ref)
	if e != nil {
		return fail()
	}
	p2, e := nauoSIPrefix(f, pr.Args[2].Ref)
	if e != nil || p1 != p2 {
		return fail()
	}
	ids, advanced, e := a.productItems(out.Part.DefinitionID)
	if e != nil || len(ids) != 1 || !advanced[0] {
		return fail()
	}
	out.SourceBodyID = ids[0]
	if e = mappedPlanarExactEdges(f, ids[0]); e != nil {
		return fail()
	}
	body, e := readBody(f, ids[0], lim.MaxVertices, lim.MaxFaces, lim.MaxEdgeUses, true)
	if e != nil {
		return fail()
	}
	if e = poly.ValidateEmbeddedBoundary(context.Background(), body.Solid, poly.Options{}); e != nil {
		return fail()
	}
	out.Body = body
	allowedRoots := []int64{ids[0]}
	for _, rep := range []Record{rr, pr} {
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
	for _, u := range a.Occurrences {
		if u.Parent != out.Root.DefinitionID || u.Child != out.Part.DefinitionID {
			return fail()
		}
		occurrences[u.ID] = u
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
	if len(linked) != 2 {
		return fail()
	}
	all, e := nauoClosure(f, roots, lim.MaxDepth)
	if e != nil || len(all) != len(f.Entities) {
		return fail()
	}
	return out, nil
}
func WriteExactPlanarNAUOSource(w io.Writer, source *File, l Limits) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadExactPlanarNAUO(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
