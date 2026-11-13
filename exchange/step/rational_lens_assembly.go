package step

import (
	"bytes"
	"io"
	"plainsolid/geom"
)

// WriteRationalSplineLensAssembly retains raw-weight product-local seam recipes,
// body/face RGB fills, and separate exact signed-axis rigid occurrences.
func WriteRationalSplineLensAssembly(w io.Writer, m SplineLensAssemblyModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var raw bytes.Buffer
	if e := writeSplineLensAssembly(&raw, m, true, true, true); e != nil {
		return e
	}
	a, e := ReadRationalSplineLensAssembly(bytes.NewReader(raw.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		return e
	}
	parts := map[string]SplineLensAssemblyPart{}
	for _, p := range m.Products {
		parts[p.ID] = p
	}
	bodies := map[int64]SplineLensBody{}
	styles := map[int64]LensStyle{}
	for _, p := range a.Assembly.Products {
		part, ok := parts[p.Identifier]
		if !ok {
			return geom.ErrInvalid
		}
		for _, b := range a.ProductBodies[p.DefinitionID] {
			bodies[b.SourceBodyID] = b
			styles[b.SourceBodyID] = part.Style
		}
	}
	return appendLensStyles(w, raw.Bytes(), a.Assembly.Source, bodies, styles)
}

// ReadRationalSplineLensAssembly proves local material and source bases, exposing
// raw weights and strict product-local fills without normalizing source weights.
func ReadRationalSplineLensAssembly(r io.Reader, l Limits, o CurvedOvalOptions) (SplineLensAssembly, error) {
	a, e := readSplineLensAssembly(r, l, o, true, true, true)
	if e != nil {
		return a, e
	}
	bodies := map[int64]SplineLensBody{}
	for _, bs := range a.ProductBodies {
		for _, b := range bs {
			bodies[b.SourceBodyID] = b
		}
	}
	styles, e := readLensStyles(a.Assembly.Source, bodies, a.Assembly.LengthUnit)
	if e != nil {
		return SplineLensAssembly{}, e
	}
	a.ProductStyles = map[int64]LensStyle{}
	for pid, bs := range a.ProductBodies {
		for _, b := range bs {
			a.ProductStyles[pid] = styles[b.SourceBodyID]
		}
	}
	return a, nil
}
