package step

import (
	"bytes"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"unicode/utf8"
)

// SplineLensFromSolid recovers a canonical two-face native lens without trusting
// cached descriptors. It requires the constructor's exact face/edge order,
// original spline supports, pcurves, model curves and native tolerance. Equivalent
// reindexed or reparameterized assets require a separate explicit conversion.
func SplineLensFromSolid(s trimmed.Solid, center, radii geom.Vec2, name string, unit float64, o CurvedOvalOptions) (SplineLensBody, error) {
	return AffineSplineLensFromSolid(s, center, radii, 0, name, unit, o)
}

// AffineSplineLensFromSolid additionally recovers the exact triangular shear.
func AffineSplineLensFromSolid(s trimmed.Solid, center, radii geom.Vec2, shear float64, name string, unit float64, o CurvedOvalOptions) (SplineLensBody, error) {
	empty := SplineLensBody{}
	if !validLengthUnit(unit) || !utf8.ValidString(name) {
		return empty, geom.ErrInvalid
	}
	var e error
	o, e = o.defaults()
	if e != nil {
		return empty, e
	}
	faces := s.Faces()
	if len(faces) != 2 || faces[0].Reversed || !faces[1].Reversed {
		return empty, geom.ErrInvalid
	}
	m := SplineLensBody{Name: name, Center: center, Radii: radii, Shear: shear, LengthUnit: unit}
	var ok bool
	m.Upper, ok = faces[0].Surface.(*geom.BSplineSurface)
	if !ok {
		return empty, geom.ErrInvalid
	}
	m.Lower, ok = faces[1].Surface.(*geom.BSplineSurface)
	if !ok {
		return empty, geom.ErrInvalid
	}
	proved, e := certifySplineLens(m, o)
	if e != nil {
		return empty, e
	}
	var actual, canonical bytes.Buffer
	if e = trimmed.WriteJSON(&actual, s); e != nil {
		return empty, e
	}
	if e = trimmed.WriteJSON(&canonical, proved); e != nil {
		return empty, e
	}
	if !bytes.Equal(actual.Bytes(), canonical.Bytes()) {
		return empty, geom.ErrConvergence
	}
	m.Solid = s
	return m, nil
}
