package step

import (
	"io"
	"plainsolid/brep/trimmed"
)

// WrappedShoulderBody has two outward cylindrical walls of unequal radii,
// adjoining exactly at one height. LargerCapNames indexes the larger wall's
// lower/upper cap (one exterior disk and one annulus). SmallerEndName labels
// the remaining exterior disk. Neither wall descriptor is material-reversed.
type WrappedShoulderBody struct {
	Name            string
	Larger, Smaller WrappedSplineFace
	LargerCapNames  [2]string
	SmallerEndName  string
	Solid           trimmed.Solid
	LengthUnit      float64
}

func ReadWrappedShoulder(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedShoulderBody, error) {
	m, e := readWrappedArrangement(r, l, o, true, true)
	if e != nil {
		return WrappedShoulderBody{}, e
	}
	return WrappedShoulderBody{Name: m.Name, Larger: m.Outer, Smaller: m.Inner, LargerCapNames: m.CapNames, SmallerEndName: m.FloorName, Solid: m.Solid, LengthUnit: m.LengthUnit}, nil
}
