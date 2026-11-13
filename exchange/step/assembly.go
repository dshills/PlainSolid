package step

import (
	"errors"
	"io"
	"plainsolid/geom"
	"strings"
)

// AssemblyProduct is keyed by product-definition ID. Multiple occurrences refer
// to the same product without duplicating its local-coordinate geometry.
type AssemblyProduct struct {
	DefinitionID, ProductID       int64
	Identifier, Name, Description string
	Representations               []int64
}

// AssemblyOccurrence maps coordinates of Child into Parent with Transform.
// Parent/Child are product-definition IDs; ID is the source NAUO entity ID.
type AssemblyOccurrence struct {
	ID               int64
	Identifier, Name string
	Parent, Child    int64
	Transform        geom.Affine
}

// Assembly preserves explicit product/occurrence structure, never flattening
// instances. Source retains geometry references for supported entity decoders.
type Assembly struct {
	Products    []AssemblyProduct
	Occurrences []AssemblyOccurrence
	Roots       []int64
	LengthUnit  float64
	Source      *File
}

func frameTransform(f geom.Frame) geom.Affine {
	x, y, z, o := f.XAxis(), f.YAxis(), f.ZAxis(), f.Origin()
	return geom.Affine{{x.X, y.X, z.X, o.X}, {x.Y, y.Y, z.Y, o.Y}, {x.Z, y.Z, z.Z, o.Z}}
}
func namedReferences(r Record, indices ...int) ([]int64, error) {
	out := make([]int64, len(indices))
	for i, j := range indices {
		if j >= len(r.Args) {
			return nil, errors.New("step: missing assembly attribute")
		}
		v, e := ref(r.Args[j])
		if e != nil {
			return nil, e
		}
		out[i] = v
	}
	return out, nil
}

// ReadAssembly reads the explicit AP214/AP242 NAUO/CDSR assembly subset with
// rigid ITEM_DEFINED_TRANSFORMATION placements. MaxSolids limits products,
// MaxEdgeUses occurrences, and MaxDepth graph depth in addition to syntax depth.
// Mixed units, mapped/scaled instances, cycles and incomplete occurrence links
// fail. Geometry remains in product coordinates and is not silently flattened.
func ReadAssembly(r io.Reader, l Limits) (Assembly, error) {
	l, e := l.defaults()
	if e != nil {
		return Assembly{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return Assembly{}, e
	}
	fail := func() (Assembly, error) {
		return Assembly{}, errors.New("step: unsupported or inconsistent assembly graph")
	}
	a := Assembly{Source: f}
	products := map[int64]int{}
	repOwner := map[int64]int64{}
	occ := map[int64]int{}
	repItems := map[int64]map[int64]bool{}
	schemaOK := false
	for _, h := range f.Header {
		if h.Type == "FILE_SCHEMA" && len(h.Args) == 1 && h.Args[0].Kind == Aggregate {
			for _, v := range h.Args[0].Items {
				if v.Kind == String && (strings.HasPrefix(v.Text, "AUTOMOTIVE_DESIGN") || strings.HasPrefix(v.Text, "AP242_MANAGED_MODEL_BASED_3D_ENGINEERING")) {
					schemaOK = true
				}
			}
		}
	}
	if !schemaOK {
		return fail()
	}
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || strings.HasPrefix(rec.Type, "CARTESIAN_TRANSFORMATION_OPERATOR") || rec.Type == "EXTERNALLY_DEFINED_ITEM" || rec.Type == "ASSEMBLY_COMPONENT_USAGE_SUBSTITUTE" {
				return fail()
			}
		}
		if len(ent.Records) != 1 || ent.Records[0].Type != "PRODUCT_DEFINITION" {
			continue
		}
		d, e := simple(f, id, "PRODUCT_DEFINITION", 4)
		if e != nil {
			return fail()
		}
		for _, j := range []int{0, 1} {
			if _, e = textValue(d.Args[j]); e != nil {
				return fail()
			}
		}
		fid, e := ref(d.Args[2])
		if e != nil {
			return fail()
		}
		formation := f.Entities[fid]
		if len(formation.Records) != 1 {
			return fail()
		}
		fr := formation.Records[0]
		if (fr.Type != "PRODUCT_DEFINITION_FORMATION" && fr.Type != "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE") || len(fr.Args) < 3 || len(fr.Args) > 4 {
			return fail()
		}
		for _, j := range []int{0, 1} {
			if _, e = textValue(fr.Args[j]); e != nil {
				return fail()
			}
		}
		pid, e := ref(fr.Args[2])
		if e != nil {
			return fail()
		}
		p, e := simple(f, pid, "PRODUCT", 4)
		if e != nil {
			return fail()
		}
		identifier, e := textValue(p.Args[0])
		if e != nil {
			return fail()
		}
		name, e := textValue(p.Args[1])
		if e != nil {
			return fail()
		}
		desc, e := textValue(p.Args[2])
		if e != nil {
			return fail()
		}
		if _, e = listRefs(p.Args[3], 1); e != nil {
			return fail()
		}
		if len(a.Products) >= l.MaxSolids {
			return fail()
		}
		products[id] = len(a.Products)
		a.Products = append(a.Products, AssemblyProduct{DefinitionID: id, ProductID: pid, Identifier: identifier, Name: name, Description: desc})
	}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 {
			continue
		}
		switch ent.Records[0].Type {
		case "SHAPE_DEFINITION_REPRESENTATION":
			s, e := simple(f, id, "SHAPE_DEFINITION_REPRESENTATION", 2)
			if e != nil {
				return fail()
			}
			refs, e := namedReferences(s, 0, 1)
			if e != nil {
				return fail()
			}
			pds, e := simple(f, refs[0], "PRODUCT_DEFINITION_SHAPE", 3)
			if e != nil {
				return fail()
			}
			definition, e := ref(pds.Args[2])
			if e != nil {
				return fail()
			}
			idx, ok := products[definition]
			if !ok {
				return fail()
			}
			rep := f.Entities[refs[1]]
			if len(rep.Records) != 1 {
				return fail()
			}
			rr := rep.Records[0]
			switch rr.Type {
			case "SHAPE_REPRESENTATION", "FACETED_BREP_SHAPE_REPRESENTATION", "ADVANCED_BREP_SHAPE_REPRESENTATION", "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION", "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION":
			default:
				return fail()
			}
			if len(rr.Args) != 3 {
				return fail()
			}
			if _, e = textValue(rr.Args[0]); e != nil {
				return fail()
			}
			ctx, e := ref(rr.Args[2])
			if e != nil {
				return fail()
			}
			unit, e := contextUnits(f, ctx)
			if e != nil {
				return fail()
			}
			if a.LengthUnit != 0 && a.LengthUnit != unit {
				return fail()
			}
			a.LengthUnit = unit
			items, e := listRefs(rr.Args[1], 1)
			if e != nil {
				return fail()
			}
			repItems[refs[1]] = map[int64]bool{}
			for _, item := range items {
				repItems[refs[1]][item] = true
				if x := f.Entities[item]; len(x.Records) == 1 && x.Records[0].Type == "AXIS2_PLACEMENT_3D" {
					if _, e = placement(f, item); e != nil {
						return fail()
					}
				} else if rr.Type == "SHAPE_REPRESENTATION" {
					return fail()
				}
			}
			if _, exists := repOwner[refs[1]]; exists {
				return fail()
			}
			repOwner[refs[1]] = definition
			a.Products[idx].Representations = append(a.Products[idx].Representations, refs[1])
		case "NEXT_ASSEMBLY_USAGE_OCCURRENCE":
			n, e := simple(f, id, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", 6)
			if e != nil {
				return fail()
			}
			identifier, e := textValue(n.Args[0])
			if e != nil {
				return fail()
			}
			name, e := textValue(n.Args[1])
			if e != nil {
				return fail()
			}
			if _, e = textValue(n.Args[2]); e != nil {
				return fail()
			}
			refs, e := namedReferences(n, 3, 4)
			if e != nil || refs[0] == refs[1] {
				return fail()
			}
			if _, ok := products[refs[0]]; !ok {
				return fail()
			}
			if _, ok := products[refs[1]]; !ok {
				return fail()
			}
			if n.Args[5].Kind != Omitted && n.Args[5].Kind != String {
				return fail()
			}
			if len(a.Occurrences) >= l.MaxEdgeUses {
				return fail()
			}
			occ[id] = len(a.Occurrences)
			a.Occurrences = append(a.Occurrences, AssemblyOccurrence{ID: id, Identifier: identifier, Name: name, Parent: refs[0], Child: refs[1]})
		}
	}
	linked := map[int64]bool{}
	relationships := map[int64]bool{}
	transforms := map[int64]bool{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 || ent.Records[0].Type != "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION" {
			continue
		}
		cd, e := simple(f, id, "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", 2)
		if e != nil {
			return fail()
		}
		refs, e := namedReferences(cd, 0, 1)
		if e != nil {
			return fail()
		}
		pds, e := simple(f, refs[1], "PRODUCT_DEFINITION_SHAPE", 3)
		if e != nil {
			return fail()
		}
		oid, e := ref(pds.Args[2])
		if e != nil {
			return fail()
		}
		idx, ok := occ[oid]
		if !ok || linked[oid] {
			return fail()
		}
		o := &a.Occurrences[idx]
		rel := f.Entities[refs[0]]
		seen := map[string]bool{}
		var rr, rt Record
		for _, r := range rel.Records {
			if seen[r.Type] {
				return fail()
			}
			seen[r.Type] = true
			switch r.Type {
			case "REPRESENTATION_RELATIONSHIP":
				rr = r
			case "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION":
				rt = r
			case "SHAPE_REPRESENTATION_RELATIONSHIP":
				if len(r.Args) != 0 {
					return fail()
				}
			default:
				return fail()
			}
		}
		if len(seen) != 3 || len(rr.Args) != 4 || len(rt.Args) != 1 {
			return fail()
		}
		if _, e = textValue(rr.Args[0]); e != nil {
			return fail()
		}
		if _, e = textValue(rr.Args[1]); e != nil {
			return fail()
		}
		rids, e := namedReferences(rr, 2, 3)
		if e != nil || repOwner[rids[0]] != o.Child || repOwner[rids[1]] != o.Parent {
			return fail()
		}
		tid, e := ref(rt.Args[0])
		if e != nil {
			return fail()
		}
		tr, e := simple(f, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
		if e != nil {
			return fail()
		}
		if _, e = textValue(tr.Args[0]); e != nil {
			return fail()
		}
		if _, e = textValue(tr.Args[1]); e != nil {
			return fail()
		}
		placements, e := namedReferences(tr, 2, 3)
		if e != nil || !repItems[rids[0]][placements[0]] || !repItems[rids[1]][placements[1]] {
			return fail()
		}
		from, e := placement(f, placements[0])
		if e != nil {
			return fail()
		}
		to, e := placement(f, placements[1])
		if e != nil {
			return fail()
		}
		inv, e := frameTransform(from).Inverse()
		if e != nil {
			return fail()
		}
		o.Transform = frameTransform(to).Mul(inv)
		if !o.Transform.IsFinite() {
			return fail()
		}
		linked[oid] = true
		relationships[refs[0]] = true
		transforms[tid] = true
	}
	if len(a.Products) == 0 || len(a.Occurrences) == 0 || len(linked) != len(a.Occurrences) {
		return fail()
	}
	for _, p := range a.Products {
		if len(p.Representations) == 0 {
			return fail()
		}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			switch r.Type {
			case "SHAPE_REPRESENTATION", "FACETED_BREP_SHAPE_REPRESENTATION", "ADVANCED_BREP_SHAPE_REPRESENTATION", "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION", "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION":
				if repOwner[id] == 0 {
					return fail()
				}
			}
			if r.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE" && len(ent.Records) != 1 {
				return fail()
			}

			if strings.Contains(r.Type, "REPRESENTATION_RELATIONSHIP") && !relationships[id] {
				return fail()
			}
			if r.Type == "ITEM_DEFINED_TRANSFORMATION" && !transforms[id] {
				return fail()
			}
		}
	}
	incoming := map[int64]int{}
	children := map[int64][]int64{}
	for _, o := range a.Occurrences {
		incoming[o.Child]++
		children[o.Parent] = append(children[o.Parent], o.Child)
	}
	queue := []int64{}
	depth := map[int64]int{}
	for _, p := range a.Products {
		if incoming[p.DefinitionID] == 0 {
			queue = append(queue, p.DefinitionID)
			a.Roots = append(a.Roots, p.DefinitionID)
			depth[p.DefinitionID] = 1
		}
	}
	processed := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		processed++
		for _, child := range children[id] {
			if depth[id]+1 > depth[child] {
				depth[child] = depth[id] + 1
			}
			if depth[child] > l.MaxDepth {
				return fail()
			}
			incoming[child]--
			if incoming[child] == 0 {
				queue = append(queue, child)
			}
		}
	}
	if processed != len(a.Products) {
		return fail()
	}
	return a, nil
}
