package boolean

import (
	"math"

	"plainsolid/geom"
)

// cylinder is the carrier of a cylindrical face: the circular cylinder of
// the given radius about the z axis of the frame. Its normal points away
// from the axis.
type cylinder struct {
	frame  geom.Frame
	radius float64
}

func (s cylinder) family() family { return cylinderFamily }
func (s cylinder) axis() line     { return line{s.frame.Origin(), s.frame.ZAxis()} }

// across is the part of a vector perpendicular to the axis.
func (s cylinder) across(v geom.Vec3) geom.Vec3 {
	return v.Sub(s.frame.ZAxis().Scale(v.Dot(s.frame.ZAxis())))
}
func (s cylinder) offset(p geom.Vec3) float64 {
	return s.across(p.Sub(s.frame.Origin())).Length() - s.radius
}
func (s cylinder) normal(p geom.Vec3) geom.Vec3 {
	n, err := s.across(p.Sub(s.frame.Origin())).Unit()
	if err != nil {
		return s.frame.XAxis()
	}
	return n
}
func (s cylinder) chart() chart                 { return cylinderChart(s) }
func (s cylinder) value() (geom.Surface, error) { return geom.NewCylinderSurface(s.frame, s.radius) }
func (s cylinder) stored(q geom.Vec2) geom.Vec2 { return geom.Vec2{X: q.X / s.radius, Y: q.Y} }

// cylinderChart is the chart of a cylinder: arc length around the axis,
// measured from the frame's x axis, and distance along the axis. The
// first coordinate repeats after one turn; a loop is laid out by
// following it, so a face that winds around the axis is cut where its
// own seam runs (I4).
type cylinderChart cylinder

func (c cylinderChart) period() float64 { return 2 * math.Pi * c.radius }
func (c cylinderChart) angle(v geom.Vec3) float64 {
	return math.Atan2(v.Dot(c.frame.YAxis()), v.Dot(c.frame.XAxis()))
}
func (c cylinderChart) locate(p geom.Vec3) geom.Vec2 {
	d := p.Sub(c.frame.Origin())
	return geom.Vec2{X: c.radius * c.angle(d), Y: d.Dot(c.frame.ZAxis())}
}
func (c cylinderChart) lift(q geom.Vec2) geom.Vec3 {
	s, k := math.Sincos(q.X / c.radius)
	return c.frame.ToWorld(geom.Vec3{X: c.radius * k, Y: c.radius * s, Z: q.Y})
}

// chord is a ruling when the two points have one place around the axis,
// and an arc of a section circle when they have one height and are less
// than a turn apart. No other chord of the chart is a line, circle or
// ellipse in space.
func (c cylinderChart) chord(p, q geom.Vec2, eps float64) (leg, bool) {
	switch {
	case math.Abs(q.X-p.X) <= eps && math.Abs(q.Y-p.Y) > eps:
		dir := c.frame.ZAxis().Scale(sign(q.Y - p.Y))
		return leg{line{c.lift(p), dir}, 0, math.Abs(q.Y - p.Y)}, true
	case math.Abs(q.Y-p.Y) <= eps && math.Abs(q.X-p.X) > eps && math.Abs(q.X-p.X) < c.period():
		k := conic{c.frame.Origin().Add(c.frame.ZAxis().Scale(p.Y)), c.frame.XAxis(), c.frame.YAxis(), c.radius, c.radius}
		if q.X < p.X {
			return leg{k.flipped(), -p.X / c.radius, -q.X / c.radius}, true
		}
		return leg{k, p.X / c.radius, q.X / c.radius}, true
	}
	return leg{}, false
}

// image lays a generator, a section circle or a section ellipse of the
// cylinder into the chart. Along the axis the terms of the carrier carry
// over. Around it, a conic that lies on the cylinder turns at the rate of
// its own parameter, and a generator stays where it is.
func (c cylinderChart) image(l leg) path {
	base, lin, cos, sin := l.c.terms()
	z := c.frame.ZAxis()
	p := path{t0: l.t0, t1: l.t1}
	p.base.Y, p.lin.Y, p.cos.Y, p.sin.Y = base.Sub(c.frame.Origin()).Dot(z), lin.Dot(z), cos.Dot(z), sin.Dot(z)
	if cos == (geom.Vec3{}) && sin == (geom.Vec3{}) {
		p.base.X = c.radius * c.angle(base.Sub(c.frame.Origin()))
		return p
	}
	// A section circle keeps one height. Its two axis vectors are across
	// the axis, and their products with it are zero but for rounding,
	// which a frame off the coordinate axes leaves at about 1e-16. Left
	// in, they put the corners of an outline along one rim at heights
	// that differ in the last bits, and trimmed.NewSolid then meets a
	// triangle of no area among them.
	if level := 0x1p-40 * (cos.Length() + sin.Length()); math.Abs(p.cos.Y) <= level && math.Abs(p.sin.Y) <= level {
		p.cos.Y, p.sin.Y = 0, 0
	}
	p.base.X, p.lin.X = c.radius*c.angle(cos), c.radius
	if cos.Cross(sin).Dot(z) < 0 {
		p.lin.X = -c.radius
	}
	return p
}

// flux is the surface integral of x·n / 3 over the region the loops
// bound. On the cylinder x·n is the radius plus the part of the frame
// origin along the normal, which depends on the angle only, so the
// integral reduces to one around the boundary.
func (c cylinderChart) flux(loops [][]leg) float64 {
	ox, oy := c.frame.Origin().Dot(c.frame.XAxis()), c.frame.Origin().Dot(c.frame.YAxis())
	sum := 0.
	for _, loop := range loops {
		for _, p := range chain(c, loop) {
			// The integrand turns with the angle, so a path is cut by
			// the angle it covers as well as by its own shape.
			turn := math.Abs(p.at(p.t1).X-p.at(p.t0).X) / c.radius
			sum += p.integral(p.pieces()+int(turn/(math.Pi/16)), func(q, d geom.Vec2) float64 {
				s, k := math.Sincos(q.X / c.radius)
				return -q.Y * (c.radius + ox*k + oy*s) * d.X
			})
		}
	}
	return sum / 3
}
