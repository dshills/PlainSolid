package step

import (
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// QuarterCylinderModel retains six source face roles and twelve edge roles.
// Faces are the four increasing angular quadrants followed by bottom/top caps;
// edges are bottom arcs, top arcs and axial lines, each ordered +X,+Y,-X,-Y
// in the exact source frame. Solid is the equivalent canonical native cylinder.
type QuarterCylinderModel struct {
	Frame                      geom.Frame
	Name                       string
	LengthUnit, Radius, Height float64
	Solid                      parametric.Solid
	Body, Shell                int64
	Faces                      [6]int64
	Edges                      [12]int64
}

// ReadQuarterCylinderModel proves full four-quarter model-only coverage with
// exact cardinal rim vertices, increasing circle arcs and opposite coedges.
// It does not admit the half-split dialect or approximate source coordinates.
func ReadQuarterCylinderModel(r io.Reader, l Limits) (QuarterCylinderModel, error) {
	m, e := readCylinderCells(r, l, 4)
	if e != nil {
		return QuarterCylinderModel{}, e
	}
	out := QuarterCylinderModel{Frame: m.Frame, Name: m.Name, LengthUnit: m.LengthUnit, Radius: m.Radius, Height: m.Height, Solid: m.Solid, Body: m.Body, Shell: m.Shell}
	copy(out.Faces[:], m.Faces)
	copy(out.Edges[:], m.Edges)
	return out, nil
}

// WriteQuarterCylinderModel constructs exact directed quarter-cell topology.
// Descriptor dimensions/frames share the strict split-cylinder numeric domain;
// this is a distinct six-face dialect with no implicit product envelope.
func WriteQuarterCylinderModel(w io.Writer, d SplitCylinderDescriptor) error {
	return writeCylinderCells(w, d, 4)
}
