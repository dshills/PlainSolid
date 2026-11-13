package step

import (
	"bytes"
	"errors"
	"io"
)

// ColoredSplineShellBody stores a product-local body fill and optional face
// overrides. Indices follow Body.Faces and the source CLOSED_SHELL face order.
type ColoredSplineShellBody struct {
	Body         SplineShellBody
	SurfaceColor *RGB
	FaceColors   map[int]RGB
}
type ColoredSplineShellAssemblyPart struct {
	ID, Name string
	Bodies   []ColoredSplineShellBody
}
type ColoredSplineShellAssemblyModel struct {
	Products    []ColoredSplineShellAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}
type ColoredSplineShellAssembly struct {
	Assembly      Assembly
	ProductBodies map[int64][]ColoredSplineShellBody
}

// ReadColoredSplineShellAssembly preserves strict product-local body/face RGB
// fills on certified spline bodies. Unsupported occurrence style overrides fail.
// Geometry, ancestry and style graphs are validated before returning assets.
func ReadColoredSplineShellAssembly(r io.Reader, l Limits) (ColoredSplineShellAssembly, error) {
	a, e := ReadSplineShellAssembly(r, l)
	if e != nil {
		return ColoredSplineShellAssembly{}, e
	}
	ids, contexts, e := colorBodyIDs(a.Assembly.Source)
	if e != nil {
		return ColoredSplineShellAssembly{}, e
	}
	bodies := map[int64]SplineShellBody{}
	productIDs := map[int64][]int64{}
	for _, p := range a.Assembly.Products {
		items, _, e := a.Assembly.productItems(p.DefinitionID)
		if e != nil {
			return ColoredSplineShellAssembly{}, e
		}
		local := a.ProductBodies[p.DefinitionID]
		if len(items) != len(local) {
			return ColoredSplineShellAssembly{}, errors.New("step: spline style geometry count mismatch")
		}
		productIDs[p.DefinitionID] = items
		for i, id := range items {
			bodies[id] = local[i]
		}
	}
	counts := make([]int, len(ids))
	metadata := Model{LengthUnit: a.Assembly.LengthUnit, Bodies: make([]Body, len(ids))}
	if len(ids) != len(bodies) {
		return ColoredSplineShellAssembly{}, errors.New("step: unowned colored spline body")
	}
	for i, id := range ids {
		b, ok := bodies[id]
		if !ok {
			return ColoredSplineShellAssembly{}, errors.New("step: unknown colored spline body")
		}
		counts[i] = len(b.Faces)
	}
	styles, e := readStylesForBodyCounts(a.Assembly.Source, metadata, ids, contexts, true, counts)
	if e != nil {
		return ColoredSplineShellAssembly{}, e
	}
	colors := map[int64]ColoredSplineShellBody{}
	for i, id := range ids {
		c := styles.Bodies[i]
		colors[id] = ColoredSplineShellBody{Body: bodies[id], SurfaceColor: c.SurfaceColor, FaceColors: c.FaceColors}
	}
	out := ColoredSplineShellAssembly{Assembly: a.Assembly, ProductBodies: map[int64][]ColoredSplineShellBody{}}
	for _, p := range a.Assembly.Products {
		for _, id := range productIDs[p.DefinitionID] {
			out.ProductBodies[p.DefinitionID] = append(out.ProductBodies[p.DefinitionID], colors[id])
		}
	}
	return out, nil
}

// WriteColoredSplineShellAssembly validates complete local shell geometry and
// emits shared product-local fills. Every face color index must refer to an
// authoritative Body.Faces entry. Occurrence matrices do not alter style scope.
func WriteColoredSplineShellAssembly(w io.Writer, m ColoredSplineShellAssemblyModel) error {
	fail := func() error { return errors.New("step: invalid colored spline shell assembly") }
	if w == nil {
		return fail()
	}
	base := SplineShellAssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	parts := map[string]ColoredSplineShellAssemblyPart{}
	for _, p := range m.Products {
		part := SplineShellAssemblyPart{ID: p.ID, Name: p.Name}
		parts[p.ID] = p
		for _, b := range p.Bodies {
			if b.SurfaceColor != nil && !validRGB(*b.SurfaceColor) {
				return fail()
			}
			for index, c := range b.FaceColors {
				if index < 0 || index >= len(b.Body.Faces) || !validRGB(c) {
					return fail()
				}
			}
			part.Bodies = append(part.Bodies, b.Body)
		}
		base.Products = append(base.Products, part)
	}
	var raw bytes.Buffer
	if e := WriteSplineShellAssembly(&raw, base); e != nil {
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
	targets := []styleTarget{}
	for _, product := range a.Products {
		part, ok := parts[product.Identifier]
		if !ok {
			return fail()
		}
		ids, _, e := a.productItems(product.DefinitionID)
		if e != nil || len(ids) != len(part.Bodies) {
			return fail()
		}
		for i, id := range ids {
			b := part.Bodies[i]
			if b.SurfaceColor != nil {
				targets = append(targets, styleTarget{id, contexts[id], *b.SurfaceColor})
			}
			faces, e := bodyFaceIDs(a.Source, id)
			if e != nil || len(faces) != len(b.Body.Faces) {
				return fail()
			}
			for j, fid := range faces {
				if color, ok := b.FaceColors[j]; ok {
					targets = append(targets, styleTarget{fid, contexts[id], color})
				}
			}
		}
	}
	var staged bytes.Buffer
	if e := appendSurfaceStyles(&staged, raw.Bytes(), a.Source, targets); e != nil {
		return e
	}
	if _, e := ReadColoredSplineShellAssembly(bytes.NewReader(staged.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return e
}
