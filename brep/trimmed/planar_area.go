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
