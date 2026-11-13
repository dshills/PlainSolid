package iges

import (
	"bytes"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// AffineSplineLensFromSolid recovers original support charts from a canonical
// native lens with an explicit represented conjugate basis. It re-proves the
// material and requires exact native topology, pcurves, model curves and
// tolerance; equivalent reindexed/reparameterized solids are not inferred.
func AffineSplineLensFromSolid(s trimmed.Solid, center, radii geom.Vec2, shear, tilt, unit float64) (SplineLensSolid, error) {
	m := SplineLensSolid{Center: center, Radii: radii, Shear: shear, Tilt: tilt, LengthUnit: unit}
	if unit <= 0 || !finite(unit) {
		return SplineLensSolid{}, geom.ErrInvalid
	}
	faces := s.Faces()
	if len(faces) != 2 || faces[0].Reversed || !faces[1].Reversed {
		return SplineLensSolid{}, geom.ErrInvalid
	}
	var ok bool
	m.Upper, ok = faces[0].Surface.(*geom.BSplineSurface)
	if !ok {
		return SplineLensSolid{}, geom.ErrInvalid
	}
	m.Lower, ok = faces[1].Surface.(*geom.BSplineSurface)
	if !ok {
		return SplineLensSolid{}, geom.ErrInvalid
	}
	proved, e := certifyIGESLens(m)
	if e != nil {
		return SplineLensSolid{}, e
	}
	var a, b bytes.Buffer
	if e = trimmed.WriteJSON(&a, s); e != nil {
		return SplineLensSolid{}, e
	}
	if e = trimmed.WriteJSON(&b, proved); e != nil {
		return SplineLensSolid{}, e
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		return SplineLensSolid{}, geom.ErrConvergence
	}
	m.Solid = s
	return m, nil
}
