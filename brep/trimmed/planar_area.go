package trimmed

import (
	"fmt"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PlanarFaceArea measures a planar face with straight and elliptical trims
// using the boundary integral in its UV chart.
func (s Solid) PlanarFaceArea(id int) (float64, error) {
	if id < 0 || id >= len(s.faces) {
		return 0, geom.ErrDomain
	}
	f := s.faces[id]
	surface, m := parametric.SurfaceMapping(f.Surface)
	plane, ok := surface.(geom.PlaneSurface)
	if !ok {
		return 0, fmt.Errorf("unsupported planar face chart or trim: %w", geom.ErrDomain)
	}
	x := m.ApplyVector(plane.Frame().XAxis())
	y := m.ApplyVector(plane.Frame().YAxis())
	cross := func(a, b geom.Vec2) float64 { return a.X*b.Y - a.Y*b.X }
	var area float64
	for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
		if len(loop) == 0 {
			return 0, geom.ErrInvalid
		}
		anchor := loop[0].UVStart
		for _, u := range loop {
			switch c := u.Curve.(type) {
			case nil:
				area += cross(sub2(u.UVStart, anchor), sub2(u.UVEnd, anchor))
			case anchoredCircularUV:
				area += cross(sub2(c.Center, anchor), sub2(c.End, c.Start)) + cross(c.Cos, c.Sin)*(c.To-c.From)
			case EllipseUV:
				if c.Linear != (geom.Vec2{}) {
					return 0, fmt.Errorf("unsupported planar face chart or trim: %w", geom.ErrDomain)
				}
				a, err := c.EvalUV(0)
				if err != nil {
					return 0, err
				}
				b, err := c.EvalUV(1)
				if err != nil {
					return 0, err
				}
				area += cross(sub2(c.Center, anchor), sub2(b, a)) + cross(c.Cos, c.Sin)*(c.To-c.From)
			default:
				return 0, fmt.Errorf("unsupported planar face chart or trim: %w", geom.ErrDomain)
			}
		}
	}
	area = math.Abs(area) * x.Cross(y).Length() / 2
	if !finite(area) || area <= 0 {
		return 0, fmt.Errorf("invalid planar face area: %w", geom.ErrInvalid)
	}
	return area, nil
}

// CircularPlanarWire proves a single complete circular loop from its analytic
// arcs in a rigid planar chart. The chart map supplies model coordinates.
func CircularPlanarWire(wire []Use, chart geom.Affine, center geom.Vec3, radius, tolerance float64) bool {
	if len(wire) == 0 || !finite(radius) || radius <= 0 || !finite(tolerance) || tolerance <= 0 {
		return false
	}
	var turn float64
	for _, use := range wire {
		var a EllipseUV
		switch c := use.Curve.(type) {
		case EllipseUV:
			a = c
		case anchoredCircularUV:
			a = c.EllipseUV
		default:
			return false
		}
		if a.Linear != (geom.Vec2{}) || chart.Apply(geom.Vec3{X: a.Center.X, Y: a.Center.Y}).Sub(center).Length() > tolerance {
			return false
		}
		x := chart.ApplyVector(geom.Vec3{X: a.Cos.X, Y: a.Cos.Y})
		y := chart.ApplyVector(geom.Vec3{X: a.Sin.X, Y: a.Sin.Y})
		if math.Abs(x.Length()-radius) > tolerance || math.Abs(y.Length()-radius) > tolerance ||
			math.Abs(x.Dot(y)) > tolerance*radius {
			return false
		}
		turn += math.Abs(a.To - a.From)
	}
	return math.Abs(turn-2*math.Pi) <= 1e-8
}
