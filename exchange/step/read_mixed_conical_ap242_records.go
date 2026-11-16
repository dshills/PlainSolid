package step

import "plainsolid/geom"

type mixedConicalAP242Records struct {
	protocol                     int64
	definitionShapes, categories map[int64]int64
	occurrences                  []ExactNAUOOccurrence
}

func proveMixedConicalAP242Records(a Assembly, layout mixedConicalAP242Layout, l Limits) (mixedConicalAP242Records, error) {
	var zero mixedConicalAP242Records
	f := a.Source
	productIDs := map[int64]bool{}
	for _, p := range a.Products {
		if productIDs[p.ProductID] {
			return zero, errMixedConicalAP242
		}
		productIDs[p.ProductID] = true
	}
	uses, err := mixedConicalAP242UseIndex(a, layout)
	if err != nil {
		return zero, err
	}
	out := mixedConicalAP242Records{definitionShapes: map[int64]int64{}, categories: map[int64]int64{}}
	linked, relationships, transforms := map[int64]bool{}, map[int64]bool{}, map[int64]bool{}
	var roots []int64
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if !layout.geometry[id] && !mixedConicalAP242Metadata(r.Type) {
				return zero, errMixedConicalAP242
			}
			switch r.Type {
			case "APPLICATION_PROTOCOL_DEFINITION":
				if out.protocol != 0 || !validMixedConicalProtocol(f, id) {
					return zero, errMixedConicalAP242
				}
				out.protocol = id
				roots = append(roots, id)
			case "SHAPE_DEFINITION_REPRESENTATION":
				definition, err := mixedConicalDefinition(f, id)
				if err != nil || out.definitionShapes[definition] != 0 {
					return zero, errMixedConicalAP242
				}
				out.definitionShapes[definition] = id
				roots = append(roots, id)
			case "PRODUCT_RELATED_PRODUCT_CATEGORY":
				product, err := mixedConicalCategory(f, id)
				if err != nil || !productIDs[product] || out.categories[product] != 0 {
					return zero, errMixedConicalAP242
				}
				out.categories[product] = id
				roots = append(roots, id)
			case "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION":
				u, err := mixedConicalOccurrence(f, id, uses)
				if err != nil || linked[u.NAUOID] || relationships[u.RelationshipID] || transforms[u.TransformationID] {
					return zero, errMixedConicalAP242
				}
				linked[u.NAUOID], relationships[u.RelationshipID], transforms[u.TransformationID] = true, true, true
				out.occurrences = append(out.occurrences, u)
				roots = append(roots, id)
			}
		}
	}
	if out.protocol == 0 || len(out.definitionShapes) != len(a.Products) || len(out.categories) != len(a.Products) ||
		len(out.occurrences) != len(a.Occurrences) {
		return zero, errMixedConicalAP242
	}
	all, err := nauoClosure(f, roots, l.MaxDepth)
	if err != nil || len(all) != len(f.Entities) {
		return zero, errMixedConicalAP242
	}
	return out, nil
}

func mixedConicalAP242UseIndex(a Assembly, layout mixedConicalAP242Layout) (map[int64]AssemblyOccurrence, error) {
	uses, leaves := map[int64]AssemblyOccurrence{}, map[int64]bool{}
	root, group := a.Roots[0], layout.group.DefinitionID
	parent, outer, inner := root, 0, 0
	if group != 0 {
		parent = group
	}
	for _, u := range a.Occurrences {
		switch {
		case group != 0 && u.Parent == root && u.Child == group:
			outer++
		case u.Parent == parent && layout.parts[u.Child] != 0:
			leaves[u.Child] = true
			inner++
		default:
			return nil, errMixedConicalAP242
		}
		uses[u.ID] = u
	}
	if len(leaves) != len(layout.parts) || group != 0 && (outer < 2 || outer > 8 || inner > 8) {
		return nil, errMixedConicalAP242
	}
	return uses, nil
}

func mixedConicalAP242Metadata(kind string) bool {
	switch kind {
	case "APPLICATION_CONTEXT", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_CONTEXT", "PRODUCT_DEFINITION_CONTEXT",
		"PRODUCT", "PRODUCT_DEFINITION_FORMATION", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_SHAPE",
		"PRODUCT_RELATED_PRODUCT_CATEGORY", "SHAPE_DEFINITION_REPRESENTATION", "SHAPE_REPRESENTATION",
		"ADVANCED_BREP_SHAPE_REPRESENTATION", "NEXT_ASSEMBLY_USAGE_OCCURRENCE", "ITEM_DEFINED_TRANSFORMATION",
		"REPRESENTATION_RELATIONSHIP", "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION", "SHAPE_REPRESENTATION_RELATIONSHIP",
		"CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", "GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT",
		"REPRESENTATION_CONTEXT", "LENGTH_UNIT", "NAMED_UNIT", "SI_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT",
		"GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT", "UNCERTAINTY_MEASURE_WITH_UNIT":
		return true
	}
	return false
}

func validMixedConicalProtocol(f *File, id int64) bool {
	r, err := simple(f, id, "APPLICATION_PROTOCOL_DEFINITION", 4)
	if err != nil || r.Args[0].Kind != String || r.Args[0].Text != "international standard" ||
		r.Args[1].Kind != String || r.Args[1].Text != "ap242_managed_model_based_3d_engineering" {
		return false
	}
	year, err := integer(r.Args[2])
	if err != nil || year != 2013 {
		return false
	}
	app, err := simple(f, r.Args[3].Ref, "APPLICATION_CONTEXT", 1)
	return err == nil && app.Args[0].Kind == String && app.Args[0].Text == "Managed model based 3d engineering"
}

func mixedConicalDefinition(f *File, id int64) (int64, error) {
	r, err := simple(f, id, "SHAPE_DEFINITION_REPRESENTATION", 2)
	if err != nil {
		return 0, err
	}
	shape, err := simple(f, r.Args[0].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
	if err != nil {
		return 0, err
	}
	return ref(shape.Args[2])
}

func mixedConicalCategory(f *File, id int64) (int64, error) {
	r, err := simple(f, id, "PRODUCT_RELATED_PRODUCT_CATEGORY", 3)
	if err != nil || r.Args[0].Kind != String || r.Args[0].Text != "part" || r.Args[1].Kind != Omitted {
		return 0, errMixedConicalAP242
	}
	ids, err := listRefs(r.Args[2], 1)
	if err != nil || len(ids) != 1 {
		return 0, errMixedConicalAP242
	}
	return ids[0], nil
}

func mixedConicalOccurrence(f *File, id int64, uses map[int64]AssemblyOccurrence) (ExactNAUOOccurrence, error) {
	var zero ExactNAUOOccurrence
	cd, err := simple(f, id, "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", 2)
	if err != nil {
		return zero, err
	}
	shape, err := simple(f, cd.Args[1].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
	if err != nil {
		return zero, err
	}
	use, ok := uses[shape.Args[2].Ref]
	if !ok {
		return zero, errMixedConicalAP242
	}
	rel := cd.Args[0].Ref
	var transform int64
	for _, r := range f.Entities[rel].Records {
		if r.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
			if len(r.Args) != 1 || transform != 0 {
				return zero, errMixedConicalAP242
			}
			transform = r.Args[0].Ref
		}
	}
	tr, err := simple(f, transform, "ITEM_DEFINED_TRANSFORMATION", 4)
	if err != nil {
		return zero, err
	}
	from, err := mappedRationalFrame(f, tr.Args[2].Ref)
	if err != nil {
		return zero, err
	}
	neutral, err := independentTorusAssemblyPose(from)
	if err != nil || neutral != geom.Identity() {
		return zero, errMixedConicalAP242
	}
	to, err := mappedRationalFrame(f, tr.Args[3].Ref)
	if err != nil {
		return zero, err
	}
	for row := range 3 {
		x, _ := to.X[row].Float64()
		y, _ := to.Y[row].Float64()
		z, _ := to.Z[row].Float64()
		t, _ := to.Origin[row].Float64()
		if use.Transform[row] != [4]float64{x, y, z, t} {
			return zero, errMixedConicalAP242
		}
	}
	return ExactNAUOOccurrence{
		Name: use.Name, NAUOID: use.ID, CDSRID: id, RelationshipID: rel, TransformationID: transform,
		FromPlacementID: tr.Args[2].Ref, ToPlacementID: tr.Args[3].Ref, ParentID: use.Parent, ChildID: use.Child, pose: to,
	}, nil
}
