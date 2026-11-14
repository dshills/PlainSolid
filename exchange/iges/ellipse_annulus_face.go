package iges

import (
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// EllipseFaceHole retains positive authored model/parameter curves; type144's
// inner-boundary role supplies subtraction, represented by reversed native use.
type EllipseFaceHole struct {
	Model                               EllipseConicModel
	ParameterMetadata, BoundaryMetadata CylinderDatumMetadata
	SourceBoundaryID, SourceParameterID int
}
type ellipseFaceHole = EllipseFaceHole

// EllipseAnnulusFace is a single face with one strictly contained concentric
// aligned ellipse. Outer is the disk's support/outer boundary, not another face.
type EllipseAnnulusFace struct {
	Outer EllipseDiskFace
	Inner EllipseFaceHole
}

func (m EllipseAnnulusFace) NativeFace() (trimmed.Face, geom.Ellipse, geom.Ellipse, error) {
	bad := func() (trimmed.Face, geom.Ellipse, geom.Ellipse, error) {
		return trimmed.Face{}, geom.Ellipse{}, geom.Ellipse{}, geom.ErrInvalid
	}
	f, c, e := m.Outer.NativeFace()
	if e != nil {
		return bad()
	}
	h := m.Inner
	if h.Model.LengthUnit != m.Outer.Model.LengthUnit || h.Model.Curve.Z != m.Outer.Model.Curve.Z || h.Model.Curve.Placement != nil || h.Model.Curve.PlacementMetadata != (CylinderDatumMetadata{}) || cylinderMetadata(h.ParameterMetadata) != nil || cylinderMetadata(h.BoundaryMetadata) != nil {
		return bad()
	}
	inner, e := h.Model.Curve.Geometry()
	if e != nil {
		return bad()
	}
	a, b, e := ellipseConicRadii(h.Model.Curve)
	if e != nil {
		return bad()
	}
	oa, ob, e := ellipseConicRadii(m.Outer.Model.Curve)
	if e != nil || a >= oa || b >= ob {
		return bad()
	}
	uv := trimmed.EllipseUV{Cos: geom.Vec2{X: a}, Sin: geom.Vec2{Y: b}, From: 2 * math.Pi}
	f.Holes = [][]trimmed.Use{{{Edge: 1, Reversed: true, UVStart: geom.Vec2{X: a}, UVEnd: geom.Vec2{X: a}, Curve: uv}}}
	return f, c, inner, nil
}
func ReadEllipseAnnulusFace(r io.Reader, l Limits) (EllipseAnnulusFace, error) {
	var h ellipseFaceHole
	o, e := readEllipseDiskFace(r, l, &h)
	if e != nil {
		return EllipseAnnulusFace{}, e
	}
	return EllipseAnnulusFace{o, h}, nil
}
func WriteEllipseAnnulusFace(w io.Writer, m EllipseAnnulusFace) error {
	if _, _, _, e := m.NativeFace(); e != nil {
		return e
	}
	return writeEllipseDiskFace(w, m.Outer, &m.Inner)
}
