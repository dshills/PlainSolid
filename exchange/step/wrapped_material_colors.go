package step

import (
	"bytes"
	"errors"
	"io"
)

// ColoredWrappedMaterialBody stores a product-local body fill and optional face
// overrides. Face keys are explicit unique ADVANCED_FACE names, independent of shell order.
type ColoredWrappedMaterialBody struct {
	Body         WrappedMaterialBody
	SurfaceColor *RGB
	FaceColors   map[string]RGB
}
type ColoredWrappedMaterialAssemblyPart struct {
	ID, Name string
	Bodies   []ColoredWrappedMaterialBody
}
type ColoredWrappedMaterialAssemblyModel struct {
	Products    []ColoredWrappedMaterialAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}
type ColoredWrappedMaterialAssembly struct {
	Assembly      Assembly
	ProductBodies map[int64][]ColoredWrappedMaterialBody
}

// ReadColoredWrappedMaterialAssembly preserves strict product-local body/face RGB
// fills on certified wrapped material bodies. Unsupported occurrence style overrides fail.
// Geometry, ancestry and style graphs are validated before returning assets.
func ReadColoredWrappedMaterialAssembly(r io.Reader, l Limits, o WrappedSurfaceOptions) (ColoredWrappedMaterialAssembly, error) {
	a, e := ReadWrappedMaterialAssembly(r, l, o)
	if e != nil {
		return ColoredWrappedMaterialAssembly{}, e
	}
	ids, contexts, e := colorBodyIDs(a.Assembly.Source)
	if e != nil {
		return ColoredWrappedMaterialAssembly{}, e
	}
	bodies := map[int64]WrappedMaterialBody{}
	productIDs := map[int64][]int64{}
	for _, p := range a.Assembly.Products {
		items, _, e := a.Assembly.productItems(p.DefinitionID)
		if e != nil {
			return ColoredWrappedMaterialAssembly{}, e
		}
		local := a.ProductBodies[p.DefinitionID]
		if len(items) != len(local) {
			return ColoredWrappedMaterialAssembly{}, errors.New("step: wrapped style geometry count mismatch")
		}
		productIDs[p.DefinitionID] = items
		for i, id := range items {
			bodies[id] = local[i]
		}
	}
	counts := make([]int, len(ids))
	metadata := Model{LengthUnit: a.Assembly.LengthUnit, Bodies: make([]Body, len(ids))}
	if len(ids) != len(bodies) {
		return ColoredWrappedMaterialAssembly{}, errors.New("step: unowned colored wrapped body")
	}
	for i, id := range ids {
		b, ok := bodies[id]
		if !ok {
			return ColoredWrappedMaterialAssembly{}, errors.New("step: unknown colored wrapped body")
		}
		faces, err := bodyFaceIDs(a.Assembly.Source, id)
		if err != nil {
			return ColoredWrappedMaterialAssembly{}, err
		}
		counts[i] = len(faces)
		_ = b
	}
	styles, e := readStylesForBodyCounts(a.Assembly.Source, metadata, ids, contexts, true, counts)
	if e != nil {
		return ColoredWrappedMaterialAssembly{}, e
	}
	colors := map[int64]ColoredWrappedMaterialBody{}
	for i, id := range ids {
		c := styles.Bodies[i]
		named := map[string]RGB{}
		faces, err := bodyFaceIDs(a.Assembly.Source, id)
		if err != nil {
			return ColoredWrappedMaterialAssembly{}, err
		}
		names, err := wrappedColorFaceNames(a.Assembly.Source, faces)
		if err != nil {
			return ColoredWrappedMaterialAssembly{}, err
		}
		for index, color := range c.FaceColors {
			name := names[index]
			if name == "" {
				return ColoredWrappedMaterialAssembly{}, errors.New("step: styled wrapped face needs a unique name")
			}
			for j, n := range names {
				if j != index && n == name {
					return ColoredWrappedMaterialAssembly{}, errors.New("step: ambiguous wrapped face name")
				}
			}
			named[name] = color
		}
		colors[id] = ColoredWrappedMaterialBody{Body: bodies[id], SurfaceColor: c.SurfaceColor, FaceColors: named}
	}
	out := ColoredWrappedMaterialAssembly{Assembly: a.Assembly, ProductBodies: map[int64][]ColoredWrappedMaterialBody{}}
	for _, p := range a.Assembly.Products {
		for _, id := range productIDs[p.DefinitionID] {
			out.ProductBodies[p.DefinitionID] = append(out.ProductBodies[p.DefinitionID], colors[id])
		}
	}
	return out, nil
}

// WriteColoredWrappedMaterialAssembly validates complete local shell geometry and
// emits shared product-local fills. Every face color key must refer to an
// unique regenerated face name. Occurrence matrices do not alter style scope.
func WriteColoredWrappedMaterialAssembly(w io.Writer, m ColoredWrappedMaterialAssemblyModel) error {
	fail := func() error { return errors.New("step: invalid colored wrapped shell assembly") }
	if w == nil {
		return fail()
	}
	base := WrappedMaterialAssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	parts := map[string]ColoredWrappedMaterialAssemblyPart{}
	for _, p := range m.Products {
		part := WrappedMaterialAssemblyPart{ID: p.ID, Name: p.Name}
		parts[p.ID] = p
		for _, b := range p.Bodies {
			if b.SurfaceColor != nil && !validRGB(*b.SurfaceColor) {
				return fail()
			}
			for index, c := range b.FaceColors {
				if index == "" || !validRGB(c) {
					return fail()
				}
			}
			part.Bodies = append(part.Bodies, b.Body)
		}
		base.Products = append(base.Products, part)
	}
	var raw bytes.Buffer
	if e := WriteWrappedMaterialAssembly(&raw, base); e != nil {
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
			if e != nil {
				return fail()
			}
			names, e := wrappedColorFaceNames(a.Source, faces)
			if e != nil {
				return e
			}
			found := map[string]bool{}
			for j, fid := range faces {
				if color, ok := b.FaceColors[names[j]]; ok {
					if found[names[j]] {
						return fail()
					}
					found[names[j]] = true
					targets = append(targets, styleTarget{fid, contexts[id], color})
				}
			}
			if len(found) != len(b.FaceColors) {
				return fail()
			}
		}
	}
	var staged bytes.Buffer
	if e := appendSurfaceStyles(&staged, raw.Bytes(), a.Source, targets); e != nil {
		return e
	}
	if _, e := ReadColoredWrappedMaterialAssembly(bytes.NewReader(staged.Bytes()), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return e
}

func wrappedColorFaceNames(f *File, ids []int64) ([]string, error) {
	out := make([]string, len(ids))
	for i, id := range ids {
		r, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return nil, e
		}
		if r.Args[0].Kind != String {
			return nil, errors.New("step: invalid face name")
		}
		out[i] = r.Args[0].Text
	}
	return out, nil
}
