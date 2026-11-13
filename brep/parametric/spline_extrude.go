package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// ExtrudeEllipse builds an oblique prism with an elliptical cross section.
// The profile plane is frame XY. Offset must have a nonzero component normal
// to that plane. The side is a rational tensor-product spline.
func ExtrudeEllipse(frame geom.Frame, rx, ry float64, offset geom.Vec3) (Solid, error) {
	ellipse, err := geom.NewEllipse(frame, rx, ry)
	if err != nil {
		return Solid{}, err
	}
	if !offset.IsFinite() {
		return Solid{}, geom.ErrInvalid
	}
	d := offset.Dot(frame.ZAxis())
	scale := math.Max(math.Max(rx, ry), offset.Length())
	if !finite(scale) || math.Abs(d) < scale*1e-12 {
		return Solid{}, fmt.Errorf("parametric: extrusion offset parallel to profile")
	}
	curve, err := ellipse.RationalBSpline()
	if err != nil {
		return Solid{}, err
	}
	return ExtrudeSpline(curve, frame.Origin(), frame.ZAxis(), offset, scale*1e-12)
}

// ExtrudeSpline extrudes a supported closed spline section, preserving
// its rational curve basis. Center lies in its plane and must make the
// profile star-shaped, as in LoftSplines.
func ExtrudeSpline(curve *geom.BSpline, center, normal, offset geom.Vec3, tolerance float64) (Solid, error) {
	if curve == nil || !offset.IsFinite() {
		return Solid{}, geom.ErrInvalid
	}
	p := curve.Poles()
	w, k := curve.Weights(), curve.Knots()
	z, err := normal.Unit()
	if err != nil {
		return Solid{}, err
	}
	if offset.Dot(z) < 0 {
		// Reverse the section and the cap normal together, maintaining its
		// winding relative to the new positive extrusion direction.
		z = z.Scale(-1)
		for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
			w[i], w[j] = w[j], w[i]
		}
		lo, hi := k[0], k[len(k)-1]
		for i, j := 0, len(k)-1; i <= j; i, j = i+1, j-1 {
			a, b := k[i], k[j]
			k[i], k[j] = lo+(hi-b), lo+(hi-a)
		}
		curve, err = geom.NewBSpline(curve.Degree(), p, k, w)
		if err != nil {
			return Solid{}, err
		}
	}
	for i, v := range p {
		p[i] = v.Add(offset)
	}
	top, err := geom.NewBSpline(curve.Degree(), p, k, w)
	if err != nil {
		return Solid{}, err
	}
	return LoftSplines(curve, top, center, center.Add(offset), z, tolerance)
}
