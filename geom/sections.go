package geom

import "math"

// PlaneSection is an analytic surface section. IntersectionCircle denotes
// either a circle or an ellipse in this result. Lines contains one or two
// infinite generator lines for a plane parallel to a cylinder axis.
// Tangent reports a point or generator tangency within the supplied tolerance.
// Residual records the positional discrepancy used for near-tangent decisions.
type PlaneSection struct {
	Kind     IntersectionKind
	Point    Vec3
	Ellipse  Ellipse
	Lines    []Line
	Tangent  bool
	Residual float64
}

func IntersectPlaneSphere(plane Plane, sphere Sphere, tolerance float64) (PlaneSection, error) {
	r := PlaneSection{}
	if !plane.valid() || !sphere.valid() || !finite(tolerance) || tolerance < 0 {
		return r, ErrInvalid
	}
	distance := plane.SignedDistance(sphere.center)
	if !finite(distance) {
		return r, ErrInvalid
	}
	d := math.Abs(distance)
	if d-sphere.radius > tolerance {
		return r, nil
	}
	center := sphere.center.Sub(plane.normal.Scale(distance))
	if !center.IsFinite() {
		return r, ErrInvalid
	}
	if math.Abs(d-sphere.radius) <= tolerance {
		return PlaneSection{Kind: IntersectionPoint, Point: center, Tangent: true, Residual: math.Abs(d - sphere.radius)}, nil
	}
	radius := math.Sqrt(sphere.radius-d) * math.Sqrt(sphere.radius+d)
	circle, e := NewCircle(center, plane.normal, radius)
	if e != nil {
		return r, e
	}
	return PlaneSection{Kind: IntersectionCircle, Ellipse: circle}, nil
}

// IntersectPlaneCylinder returns an ellipse, two generator lines, a tangent
// generator, or no intersection for an infinite analytic cylinder. A nonzero
// axis-plane dot product inside Angular tolerance is explicitly unresolved
// (ErrConvergence); treating it as parallel would invent incorrect infinite
// lines. Exactly parallel planes are solved analytically.
func IntersectPlaneCylinder(plane Plane, cylinder CylinderSurface, tolerance IntersectionTolerance) (PlaneSection, error) {
	r := PlaneSection{}
	if !plane.valid() || !cylinder.frame.valid() || cylinder.radius <= 0 || !tolerance.valid() {
		return r, ErrInvalid
	}
	axis := cylinder.frame.z
	den := plane.normal.Dot(axis)
	distance := plane.SignedDistance(cylinder.frame.origin)
	if !finite(distance) {
		return r, ErrInvalid
	}
	if den != 0 {
		if math.Abs(den) <= tolerance.Angular {
			return r, ErrConvergence
		}
		center := cylinder.frame.origin.Sub(axis.Scale(distance / den))
		hint := plane.normal.Cross(axis.Cross(plane.normal))
		if hint.Length() == 0 {
			hint = cylinder.frame.x
		}
		frame, e := NewFrame(center, hint, plane.normal)
		if e != nil {
			return r, e
		}
		ellipse, e := NewEllipse(frame, cylinder.radius/math.Abs(den), cylinder.radius)
		if e != nil {
			return r, e
		}
		return PlaneSection{Kind: IntersectionCircle, Ellipse: ellipse}, nil
	}
	d := math.Abs(distance)
	if d-cylinder.radius > tolerance.Linear {
		return r, nil
	}
	center := cylinder.frame.origin.Sub(plane.normal.Scale(distance))
	line, e := NewLine(center, axis)
	if e != nil {
		return r, e
	}
	if math.Abs(d-cylinder.radius) <= tolerance.Linear {
		return PlaneSection{Kind: IntersectionLine, Lines: []Line{line}, Tangent: true, Residual: math.Abs(d - cylinder.radius)}, nil
	}
	offset := math.Sqrt(cylinder.radius-d) * math.Sqrt(cylinder.radius+d)
	direction, e := axis.Cross(plane.normal).Unit()
	if e != nil {
		return r, e
	}
	left, e := NewLine(center.Sub(direction.Scale(offset)), axis)
	if e != nil {
		return r, e
	}
	right, e := NewLine(center.Add(direction.Scale(offset)), axis)
	if e != nil {
		return r, e
	}
	return PlaneSection{Kind: IntersectionLine, Lines: []Line{left, right}}, nil
}
