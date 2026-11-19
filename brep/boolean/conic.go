package boolean

import (
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// conic is the carrier of a circular or elliptic edge. Its point at
// parameter t is center + a·cos t·x + b·sin t·y, with x and y unit and
// perpendicular. A circle has a = b.
type conic struct {
	center, x, y geom.Vec3
	a, b         float64
}

// conicOf takes a conic from an ellipse value of package geom.
func conicOf(e geom.Ellipse) conic {
	frame, rx, ry := e.Parameters()
	return conic{frame.Origin(), frame.XAxis(), frame.YAxis(), rx, ry}
}

func (c conic) family() family    { return conicFamily }
func (c conic) normal() geom.Vec3 { return c.x.Cross(c.y) }
func (c conic) period() float64   { return 2 * math.Pi }
func (c conic) flipped() curve    { return conic{c.center, c.x, c.y.Scale(-1), c.a, c.b} }
func (c conic) at(t float64) geom.Vec3 {
	s, k := math.Sincos(t)
	return c.center.Add(c.x.Scale(c.a * k)).Add(c.y.Scale(c.b * s))
}
func (c conic) velocity(t float64) geom.Vec3 {
	s, k := math.Sincos(t)
	return c.x.Scale(-c.a * s).Add(c.y.Scale(c.b * k))
}
func (c conic) tangent(t float64) geom.Vec3 {
	v := c.velocity(t)
	return v.Scale(1 / v.Length())
}
func (c conic) terms() (base, lin, cos, sin geom.Vec3) {
	return c.center, geom.Vec3{}, c.x.Scale(c.a), c.y.Scale(c.b)
}

// param inverts a point to its parameter (CC1): the angle of the point in
// the conic's own coordinates, improved by Newton steps towards the
// nearest point of the conic.
func (c conic) param(p geom.Vec3) float64 {
	d := p.Sub(c.center)
	t := math.Atan2(d.Dot(c.y)/c.b, d.Dot(c.x)/c.a)
	for i := 0; i < 3; i++ {
		s, k := math.Sincos(t)
		off := c.at(t).Sub(p)
		v := c.velocity(t)
		slope := v.Dot(v) - off.Dot(c.x.Scale(c.a*k).Add(c.y.Scale(c.b*s)))
		if slope <= 0 {
			break
		}
		t -= off.Dot(v) / slope
	}
	if t = math.Mod(t, 2*math.Pi); t < 0 {
		t += 2 * math.Pi
	}
	return t
}

func (c conic) value(_, _ geom.Vec3, t0, t1 float64) (geom.Curve, parametric.Interval, error) {
	frame, err := geom.NewFrame(c.center, c.x, c.normal())
	if err != nil {
		return nil, parametric.Interval{}, err
	}
	e, err := geom.NewEllipse(frame, c.a, c.b)
	return e, parametric.Interval{Min: t0, Max: t1}, err
}

// axes gives the major radius, the minor radius and the unit direction
// of the major axis.
func (c conic) axes() (major, minor float64, along geom.Vec3) {
	if c.a >= c.b {
		return c.a, c.b, c.x
	}
	return c.b, c.a, c.y
}
