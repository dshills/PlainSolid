package step

import (
	"bytes"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
)

// NativeSplitCylinderDescriptor recovers only the original canonical
// parametric.Cylinder representation. It compares all built-in surface/curve
// descriptors, vertices, ranges, face senses, topology and constructor tolerance
// with a reconstructed cylinder. Custom evaluators, reordered topology and
// approximately equivalent geometry are deliberately unsupported. The recovered
// descriptor is then checked against the exact rooted STEP dialect.
func NativeSplitCylinderDescriptor(s parametric.Solid, name string, lengthUnit float64) (SplitCylinderDescriptor, error) {
	faces := s.Faces()
	if len(faces) != 3 {
		return SplitCylinderDescriptor{}, splitFailure()
	}
	side, ok := faces[0].Surface.(geom.CylinderSurface)
	if !ok {
		return SplitCylinderDescriptor{}, splitFailure()
	}
	frame, radius := side.Parameters()
	if faces[0].V.Min != 0 {
		return SplitCylinderDescriptor{}, splitFailure()
	}
	height := faces[0].V.Max
	canonical, e := parametric.Cylinder(frame, radius, height)
	if e != nil {
		return SplitCylinderDescriptor{}, e
	}
	// Constructor-owned geometry is immutable. DeepEqual compares descriptor
	// data and concrete built-in types, never calls potentially untrusted Eval.
	if s.Tolerance() != canonical.Tolerance() || !reflect.DeepEqual(s.Vertices(), canonical.Vertices()) || !reflect.DeepEqual(s.Edges(), canonical.Edges()) || !reflect.DeepEqual(faces, canonical.Faces()) {
		return SplitCylinderDescriptor{}, splitFailure()
	}
	d := SplitCylinderDescriptor{Name: name, Frame: frame, Radius: radius, Height: height, LengthUnit: lengthUnit}
	var staged bytes.Buffer
	if e = WriteSplitCylinderProduct(&staged, d); e != nil {
		return SplitCylinderDescriptor{}, e
	}
	return d, nil
}

// WriteNativeSplitCylinderProduct exports a single canonical native cylinder
// through the explicit exact split-cylinder product dialect. It does not infer
// a cylinder from bounding boxes, samples or arbitrary surface implementations.
func WriteNativeSplitCylinderProduct(w io.Writer, s parametric.Solid, name string, lengthUnit float64) error {
	if w == nil {
		return splitFailure()
	}
	d, e := NativeSplitCylinderDescriptor(s, name, lengthUnit)
	if e != nil {
		return e
	}
	return WriteSplitCylinderProduct(w, d)
}
