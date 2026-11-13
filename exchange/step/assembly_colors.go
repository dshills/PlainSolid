package step

import (
	"bytes"
	"errors"
	"io"
)

// ColoredAssembly preserves occurrence structure and product-local planar styles.
// ProductBodies is keyed by product-definition ID, matching Assembly.Products.
type ColoredAssembly struct {
	Assembly      Assembly
	ProductBodies map[int64][]FaceColoredBody
}
type ColoredAssemblyPart struct {
	ID, Name string
	Bodies   []FaceColoredBody
}
type ColoredAssemblyModel struct {
	Products    []ColoredAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

// ReadColoredAssembly accepts product-local body and face RGB fills. Occurrence
// overrides and unsupported presentation semantics fail rather than being lost.
func ReadColoredAssembly(r io.Reader, l Limits) (ColoredAssembly, error) {
	fail := func() (ColoredAssembly, error) {
		return ColoredAssembly{}, errors.New("step: invalid colored planar assembly")
	}
	l, e := l.defaults()
	if e != nil {
		return ColoredAssembly{}, e
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return ColoredAssembly{}, e
	}
	ids, contexts, e := colorBodyIDs(a.Source)
	if e != nil {
		return fail()
	}
	local := map[int64]Body{}
	productIDs := map[int64][]int64{}
	nv, nf, nu := 0, 0, 0
	for _, p := range a.Products {
		items, advanced, e := a.productItems(p.DefinitionID)
		if e != nil {
			return fail()
		}
		productIDs[p.DefinitionID] = items
		for i, id := range items {
			if _, ok := local[id]; ok || !advanced[i] || len(local) >= l.MaxSolids {
				return fail()
			}
			b, e := readBody(a.Source, id, l.MaxVertices-nv, l.MaxFaces-nf, l.MaxEdgeUses-nu, true)
			if e != nil {
				return ColoredAssembly{}, e
			}
			nv += len(b.Solid.Vertices())
			nf += len(b.Solid.Faces())
			for _, face := range b.Solid.Faces() {
				nu += len(face.Boundary)
				for _, hole := range face.Holes {
					nu += len(hole)
				}
			}
			local[id] = b
		}
	}
	m := Model{LengthUnit: a.LengthUnit}
	seen := map[int64]bool{}
	for _, id := range ids {
		b, ok := local[id]
		if !ok || seen[id] {
			return fail()
		}
		seen[id] = true
		m.Bodies = append(m.Bodies, b)
	}
	if len(seen) != len(local) {
		return fail()
	}
	colors, e := readStylesForBodies(a.Source, m, ids, contexts, true)
	if e != nil {
		return ColoredAssembly{}, e
	}
	byID := map[int64]FaceColoredBody{}
	for i, id := range ids {
		byID[id] = colors.Bodies[i]
	}
	result := ColoredAssembly{Assembly: a, ProductBodies: map[int64][]FaceColoredBody{}}
	for _, p := range a.Products {
		result.ProductBodies[p.DefinitionID] = nil
		for _, id := range productIDs[p.DefinitionID] {
			result.ProductBodies[p.DefinitionID] = append(result.ProductBodies[p.DefinitionID], byID[id])
		}
	}
	return result, nil
}
func WriteColoredAssembly(w io.Writer, m ColoredAssemblyModel) error {
	fail := func() error { return errors.New("step: invalid colored planar assembly model") }
	if w == nil {
		return fail()
	}
	base := AssemblyModel{Occurrences: m.Occurrences, LengthUnit: m.LengthUnit}
	for _, p := range m.Products {
		part := AssemblyPart{ID: p.ID, Name: p.Name}
		for _, b := range p.Bodies {
			if b.SurfaceColor != nil && !validRGB(*b.SurfaceColor) {
				return fail()
			}
			for index, c := range b.FaceColors {
				if index < 0 || index >= len(b.Body.Solid.Faces()) || !validRGB(c) {
					return fail()
				}
			}
			part.Bodies = append(part.Bodies, b.Body)
		}
		base.Products = append(base.Products, part)
	}
	var raw bytes.Buffer
	if e := WriteAssembly(&raw, base); e != nil {
		return e
	}
	a, e := ReadAssembly(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		return e
	}
	_, contexts, e := colorBodyIDs(a.Source)
	if e != nil {
		return e
	}
	parts := map[string]ColoredAssemblyPart{}
	for _, p := range m.Products {
		parts[p.ID] = p
	}
	targets := []styleTarget{}
	for _, product := range a.Products {
		p, ok := parts[product.Identifier]
		if !ok {
			return fail()
		}
		items, _, e := a.productItems(product.DefinitionID)
		if e != nil || len(items) != len(p.Bodies) {
			return fail()
		}
		for i, id := range items {
			b := p.Bodies[i]
			if b.SurfaceColor != nil {
				targets = append(targets, styleTarget{id, contexts[id], *b.SurfaceColor})
			}
			faces, e := bodyFaceIDs(a.Source, id)
			if e != nil {
				return e
			}
			for j, fid := range faces {
				if color, ok := b.FaceColors[j]; ok {
					targets = append(targets, styleTarget{fid, contexts[id], color})
				}
			}
		}
	}
	var out bytes.Buffer
	if e := appendSurfaceStyles(&out, raw.Bytes(), a.Source, targets); e != nil {
		return e
	}
	if _, e := ReadColoredAssembly(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}
