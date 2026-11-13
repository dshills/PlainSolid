package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// ClipSphere keeps the part of a ball on the negative side of the plane.
// The returned boundary has a spherical patch and a planar disk, both exact.
// Tangent cuts return the regularized empty or unchanged ball. Non-tangent
// cuts closer than tolerance to tangency report an unresolved operation.
func ClipSphere(ball Ball, plane geom.Plane, tolerance float64) (Solid, error) {
	if !ball.Center.IsFinite() || ball.Radius <= 0 || !finite(ball.Radius) ||
		tolerance <= 0 || !finite(tolerance) || plane.Normal().Length() == 0 {
		return Solid{}, fmt.Errorf("parametric: invalid sphere clip parameters")
	}
	h := -plane.SignedDistance(ball.Center)
	if !finite(h) {
		return Solid{}, geom.ErrInvalid
	}
	r := ball.Radius
	switch {
	case h == r || h > r && h-r > tolerance:
		return Sphere(ball.Center, r)
	case h == -r || h < -r && -r-h > tolerance:
		return Solid{}, nil
	case math.Abs(math.Abs(h)-r) <= tolerance:
		return Solid{}, fmt.Errorf("parametric: unresolved near-tangent sphere clip")
	}
	n := plane.Normal()
	frame, err := geom.FrameFromNormal(ball.Center, n)
	if err != nil {
		return Solid{}, err
	}
	surface, err := geom.NewSphereSurface(frame, r)
	if err != nil {
		return Solid{}, err
	}
	latitude := math.Asin(h / r)
	radius := r * math.Cos(latitude)
	center := ball.Center.Add(n.Scale(h))
	diskFrame, err := geom.NewFrame(center, frame.XAxis(), n)
	if err != nil {
		return Solid{}, err
	}
	side := Face{Surface: surface, U: circleDomain, V: Interval{-math.Pi / 2, latitude}, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}}
	cap := Face{Surface: disk{diskFrame}, U: circleDomain, V: Interval{0, radius}, Boundary: [4]Use{{3, false}, {4, false}, {2, false}, {4, true}}, Reversed: true}
	pole, err := side.sidePoint(0, 0)
	if err != nil {
		return Solid{}, err
	}
	rim, err := side.sidePoint(2, 0)
	if err != nil {
		return Solid{}, err
	}
	vertices := []geom.Vec3{pole, rim, center}
	edges := []Edge{
		{Start: 0, End: 0},
		{Start: 0, End: 1, Curve: iso{side, 1}, Range: normalized},
		{Start: 1, End: 1, Curve: iso{side, 2}, Range: normalized},
		{Start: 2, End: 2},
		{Start: 2, End: 1, Curve: iso{cap, 1}, Range: normalized},
	}
	return NewSolid(vertices, edges, []Face{side, cap}, tolerance)
}

// SplitSphere returns regularized negative and positive pieces cut by plane.
// Both parts use the same analytic section circle and opposite cap normals.
func SplitSphere(ball Ball, plane geom.Plane, tolerance float64) (negative, positive Solid, err error) {
	negative, err = ClipSphere(ball, plane, tolerance)
	if err != nil {
		return Solid{}, Solid{}, err
	}
	reversed, err := geom.NewPlane(plane.Origin(), plane.Normal().Scale(-1))
	if err != nil {
		return Solid{}, Solid{}, err
	}
	positive, err = ClipSphere(ball, reversed, tolerance)
	if err != nil {
		return Solid{}, Solid{}, err
	}
	return negative, positive, nil
}

// HollowSphere constructs the material between two concentric spherical
// boundaries. Thickness must be positive and smaller than Radius.
func HollowSphere(ball Ball, thickness float64) (Solid, error) {
	if !ball.valid() || thickness <= 0 || thickness >= ball.Radius || !finite(thickness) {
		return Solid{}, geom.ErrInvalid
	}
	inner := ball.Radius - thickness
	if inner == ball.Radius || inner <= 0 {
		return Solid{}, fmt.Errorf("parametric: unresolved shell thickness")
	}
	return CombineSpheres(ball, Ball{Center: ball.Center, Radius: inner}, Difference, ball.Radius*1e-12)
}

// SplitCylinder returns the negative and positive pieces of a supported
// transverse plane cut. Both parts retain exact cylindrical walls and the
// same geometric section ellipse.
func SplitCylinder(frame geom.Frame, radius, height float64, plane geom.Plane, tolerance float64) (negative, positive Solid, err error) {
	negative, err = ClipCylinder(frame, radius, height, plane, tolerance)
	if err != nil {
		return Solid{}, Solid{}, err
	}
	reverse, err := geom.NewPlane(plane.Origin(), plane.Normal().Scale(-1))
	if err != nil {
		return Solid{}, Solid{}, err
	}
	positive, err = ClipCylinder(frame, radius, height, reverse, tolerance)
	if err != nil {
		return Solid{}, Solid{}, err
	}
	return negative, positive, nil
}
