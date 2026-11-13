package step

import (
	"bytes"
	"errors"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WriteNativeSteppedCylinder writes a complete two-radius coaxial solid with
// one annular shoulder. It accepts the canonical native revolution topology,
// including the result of an unequal coaxial cylinder union. The shape is
// validated before writing; samples alone cannot authorize export.
func WriteNativeSteppedCylinder(w io.Writer, s parametric.Solid, name string, lengthUnit float64) error {
	bad := errors.New("step: unsupported native stepped cylinder boundary")
	if w == nil || len(s.Faces()) != 5 {
		return bad
	}
	frame, profile, ok := parametric.AxialSteppedCylinderMeridian(s)
	if !ok {
		return bad
	}
	axis, x := frame.ZAxis(), frame.XAxis()
	lower, upper := profile[1].X, profile[3].X
	h0, h1 := profile[2].Y, profile[4].Y-profile[2].Y
	if lower < upper {
		var err error
		frame, err = geom.NewFrame(frame.ToWorld(geom.Vec3{Z: profile[4].Y}), x, axis.Scale(-1))
		if err != nil {
			return err
		}
		lower, upper, h0, h1 = upper, lower, h1, h0
	}
	var staged bytes.Buffer
	if err := WriteSteppedCylinders(&staged, SteppedCylinderModel{LengthUnit: lengthUnit, Bodies: []SteppedCylinderBody{{Name: name, Frame: frame, LowerRadius: lower, UpperRadius: upper, LowerHeight: h0, UpperHeight: h1}}}); err != nil {
		return err
	}
	n, err := w.Write(staged.Bytes())
	if err == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return err
}
