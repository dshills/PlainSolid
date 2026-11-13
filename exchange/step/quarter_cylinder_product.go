package step

import (
	"io"
	"plainsolid/brep/parametric"
)

// QuarterCylinderProduct is the explicit six-face dialect with one canonical
// AP203 product chain. Product identity is 'quarter-cylinder'; name is bound to
// the solid. Source roles and native geometry remain separately represented.
type QuarterCylinderProduct struct {
	Model                                QuarterCylinderModel
	Product, Definition, ShapeDefinition int64
}

// WriteQuarterCylinderProduct stages exact quarter topology and its strict
// rooted envelope before writing. It does not widen the half-split dialect.
func WriteQuarterCylinderProduct(w io.Writer, d SplitCylinderDescriptor) error {
	return writeCylinderProduct(w, d, 4)
}

// ReadQuarterCylinderProduct requires both the quarter product identity and
// complete six-face/twelve-edge coverage; relabeling another graph is invalid.
func ReadQuarterCylinderProduct(r io.Reader, l Limits) (QuarterCylinderProduct, error) {
	p, e := readCylinderProduct(r, l, 4)
	if e != nil {
		return QuarterCylinderProduct{}, e
	}
	m := p.Model
	model := QuarterCylinderModel{Frame: m.Frame, Name: m.Name, LengthUnit: m.LengthUnit, Radius: m.Radius, Height: m.Height, Solid: m.Solid, Body: m.Body, Shell: m.Shell}
	copy(model.Faces[:], m.Faces)
	copy(model.Edges[:], m.Edges)
	return QuarterCylinderProduct{Model: model, Product: p.Product, Definition: p.Definition, ShapeDefinition: p.ShapeDefinition}, nil
}

// WriteNativeQuarterCylinderProduct requires the same full canonical native
// cylinder descriptor classification as the half-split native export route.
func WriteNativeQuarterCylinderProduct(w io.Writer, s parametric.Solid, name string, lengthUnit float64) error {
	if w == nil {
		return splitFailure()
	}
	d, e := NativeSplitCylinderDescriptor(s, name, lengthUnit)
	if e != nil {
		return e
	}
	return WriteQuarterCylinderProduct(w, d)
}
