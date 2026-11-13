package step

import (
	"io"
	"plainsolid/geom"
)

// AxialProfileBody specifies coaxial cylinder sections joined by annular shoulders.
// CylinderSection is one positive-height circular section of an axial profile.
type CylinderSection struct{ Radius, Height float64 }
type AxialProfileBody struct {
	Name     string
	Frame    geom.Frame
	Sections []CylinderSection
}

type AxialProfileModel struct {
	Bodies     []AxialProfileBody
	LengthUnit float64
}

func WriteAxialProfiles(w io.Writer, m AxialProfileModel) error {
	mixed := MixedAxialModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		part := MixedAxialBody{Name: b.Name, Frame: b.Frame}
		for _, s := range b.Sections {
			part.Sections = append(part.Sections, AxialSection{s.Radius, s.Radius, s.Height})
		}
		mixed.Bodies = append(mixed.Bodies, part)
	}
	return WriteMixedAxialProfiles(w, mixed)
}
