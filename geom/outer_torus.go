package geom

import "math"

// OuterTorusSurface is the regular outward half of a circular rolling band.
// Longitude U is in radians and meridian V is in [-pi/2, pi/2]. The radial
// distance from the axis is at least the positive major radius, even when the
// extension to a complete horn or spindle torus would have singularities.
type OuterTorusSurface struct{ torus TorusSurface }

func NewOuterTorusSurface(frame Frame, major, minor float64) (OuterTorusSurface, error) {
	if !frame.valid() || !finite(major) || !finite(minor) || major <= 0 || minor <= 0 || !finite(major+minor) {
		return OuterTorusSurface{}, ErrInvalid
	}
	return OuterTorusSurface{TorusSurface{frame, major, minor}}, nil
}

func (s OuterTorusSurface) Parameters() (Frame, float64, float64) { return s.torus.Parameters() }

func (s OuterTorusSurface) Eval(u, v float64) (Vec3, error) {
	if v < -math.Pi/2 || v > math.Pi/2 {
		return Vec3{}, ErrDomain
	}
	return s.torus.Eval(u, v)
}

func (s OuterTorusSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if v < -math.Pi/2 || v > math.Pi/2 {
		return Vec3{}, Vec3{}, ErrDomain
	}
	return s.torus.Partials(u, v)
}
