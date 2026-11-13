package step

import "errors"

func (a Assembly) productItems(definition int64) ([]int64, []bool, error) {
	if a.Source == nil {
		return nil, nil, errors.New("step: missing assembly source")
	}
	var ids []int64
	var advanced []bool
	found := false
	for _, p := range a.Products {
		if p.DefinitionID != definition {
			continue
		}
		found = true
		for _, rid := range p.Representations {
			ent, ok := a.Source.Entities[rid]
			if !ok || len(ent.Records) != 1 || len(ent.Records[0].Args) != 3 {
				return nil, nil, errors.New("step: invalid product representation")
			}
			r := ent.Records[0]
			items, e := listRefs(r.Args[1], 1)
			if e != nil {
				return nil, nil, e
			}
			for _, id := range items {
				if e := a.Source.Entities[id]; len(e.Records) == 1 && e.Records[0].Type == "AXIS2_PLACEMENT_3D" {
					if _, err := placement(a.Source, id); err != nil {
						return nil, nil, err
					}
					continue
				}
				if r.Type != "FACETED_BREP_SHAPE_REPRESENTATION" && r.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
					return nil, nil, errors.New("step: product is not a supported solid representation")
				}
				ids = append(ids, id)
				advanced = append(advanced, r.Type == "ADVANCED_BREP_SHAPE_REPRESENTATION")
			}
		}
	}
	if !found {
		return nil, nil, errors.New("step: unknown product definition")
	}
	return ids, advanced, nil
}

// PlanarBodies decodes a product's planar bodies in local coordinates, without
// applying any occurrence transform. Assembly-only products return no bodies.
func (a Assembly) PlanarBodies(definition int64, l Limits) ([]Body, error) {
	l, e := l.defaults()
	if e != nil {
		return nil, e
	}
	ids, advanced, e := a.productItems(definition)
	if e != nil {
		return nil, e
	}
	var out []Body
	nv, nf, nu := 0, 0, 0
	seen := map[int64]bool{}
	for i, id := range ids {
		if seen[id] || len(out) >= l.MaxSolids {
			return nil, errors.New("step: duplicate body or solid limit")
		}
		seen[id] = true
		b, e := readBody(a.Source, id, l.MaxVertices-nv, l.MaxFaces-nf, l.MaxEdgeUses-nu, advanced[i])
		if e != nil {
			return nil, e
		}
		nv += len(b.Solid.Vertices())
		nf += len(b.Solid.Faces())
		for _, f := range b.Solid.Faces() {
			nu += len(f.Boundary)
			for _, h := range f.Holes {
				nu += len(h)
			}
		}
		out = append(out, b)
	}
	return out, nil
}

// AnalyticBodies decodes supported exact analytic products in local coordinates.
func (a Assembly) AnalyticBodies(definition int64, l Limits) ([]AnalyticBody, error) {
	l, e := l.defaults()
	if e != nil {
		return nil, e
	}
	ids, advanced, e := a.productItems(definition)
	if e != nil {
		return nil, e
	}
	var out []AnalyticBody
	nv, nf, nu := 0, 0, 0
	seen := map[int64]bool{}
	for i, id := range ids {
		if !advanced[i] || seen[id] || len(out) >= l.MaxSolids {
			return nil, errors.New("step: unsupported analytic product")
		}
		seen[id] = true
		b, e := analyticBody(a.Source, id)
		if e != nil {
			return nil, e
		}
		v, f := len(b.Solid.Vertices()), len(b.Solid.Faces())
		if v > l.MaxVertices-nv || f > l.MaxFaces-nf || f > (l.MaxEdgeUses-nu)/4 {
			return nil, errors.New("step: analytic product limit")
		}
		nv += v
		nf += f
		nu += 4 * f
		out = append(out, b)
	}
	return out, nil
}
