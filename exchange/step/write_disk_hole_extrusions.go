package step

import (
	"context"
	"io"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// DiskHoleExtrusionBody describes signed axial extrusion of a circular sketch.
// Eccentric circular through holes and blind pockets must be strictly inside
// the outer disk and disjoint. Blind depths start at the original sketch cap
// or the opposite cap as specified on each bore.
type DiskHoleExtrusionBody struct {
	Name   string
	Frame  geom.Frame
	Outer  trimmed.ExtrusionCircle
	Holes  []trimmed.ExtrusionCircle
	Blind  []trimmed.ExtrusionBlindHole
	Height float64
}

type DiskHoleExtrusionModel struct {
	Bodies     []DiskHoleExtrusionBody
	LengthUnit float64
}

// WriteDiskHoleExtrusions exports native analytic circular stock with optional
// bores and pockets as STEP advanced B-rep. Each full cylinder uses shared
// circular edges and one explicit periodic seam. Each face retains its pcurves.
func WriteDiskHoleExtrusions(w io.Writer, m DiskHoleExtrusionModel) error {
	if w == nil || len(m.Bodies) == 0 || len(m.Bodies) > 32 {
		return geom.ErrInvalid
	}
	bodies := make([]circularExtrusionExport, len(m.Bodies))
	for i, b := range m.Bodies {
		if !utf8.ValidString(b.Name) || len(b.Name) > 4096 {
			return geom.ErrInvalid
		}
		if _, e := trimmed.ExtrudeDiskWithHoles(context.Background(), b.Frame, b.Outer, b.Holes, b.Blind, b.Height); e != nil {
			return e
		}
		disk := b.Outer
		bodies[i] = circularExtrusionExport{
			CircularHoleExtrusionBody: CircularHoleExtrusionBody{
				Name: b.Name, Frame: b.Frame, Height: b.Height,
				Holes: append([]trimmed.ExtrusionCircle(nil), b.Holes...),
				Blind: append([]trimmed.ExtrusionBlindHole(nil), b.Blind...),
			},
			disk: &disk,
		}
	}
	return writeCircularExtrusionSTEP(w, m.LengthUnit, bodies)
}
