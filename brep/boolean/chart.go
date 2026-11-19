package boolean

import (
	"math"

	"plainsolid/geom"
)

// chart is the planar coordinate domain in which the faces of one surface
// carrier are split (section 3). Splitting, region tests and the
// piece-based volumes are written against it, so that a new carrier
// arrives as a chart and not as a second path. Every chart is isometric
// to its surface: chart lengths and angles are model lengths and angles.
type chart interface {
	// locate gives the chart coordinates of a point on the carrier. Where
	// the first coordinate repeats, it is the one in half a period of zero.
	locate(p geom.Vec3) geom.Vec2
	// lift gives the point of the carrier at chart coordinates q.
	lift(q geom.Vec2) geom.Vec3
	// image gives a leg that lies on the carrier as a path in the chart.
	image(l leg) path
	// period is the length after which the first chart coordinate
	// repeats, and zero when it does not.
	period() float64
	// flux integrates x·n / 3 over the region the loops bound, where n is
	// the carrier normal: the region's share of an enclosed volume.
	flux(loops [][]leg) float64
}

// path is the image of a leg in a chart: the point at parameter t is
// base + lin·t + cos·cos t + sin·sin t, for t from t0 to t1. A line or a
// conic in a plane and a generator, a section circle or a section ellipse
// of a cylinder all have this form, and so has the p-curve
// trimmed.EllipseUV.
type path struct {
	base, lin, cos, sin geom.Vec2
	t0, t1              float64
}

func (p path) at(t float64) geom.Vec2 {
	s, c := math.Sincos(t)
	return geom.Vec2{X: p.base.X + p.lin.X*t + p.cos.X*c + p.sin.X*s, Y: p.base.Y + p.lin.Y*t + p.cos.Y*c + p.sin.Y*s}
}

// speed is the derivative of the path by its parameter.
func (p path) speed(t float64) geom.Vec2 {
	s, c := math.Sincos(t)
	return geom.Vec2{X: p.lin.X - p.cos.X*s + p.sin.X*c, Y: p.lin.Y - p.cos.Y*s + p.sin.Y*c}
}

// curved reports whether the path has a periodic part.
func (p path) curved() bool { return p.cos != (geom.Vec2{}) || p.sin != (geom.Vec2{}) }

// moved is the path shifted along the first chart coordinate.
func (p path) moved(d float64) path {
	p.base.X += d
	return p
}

// heading is the unit chart direction in which the path leaves its start,
// and its signed curvature there, positive to the left of travel.
func (p path) heading() (geom.Vec2, float64) {
	v := p.speed(p.t0)
	s, c := math.Sincos(p.t0)
	a := geom.Vec2{X: -p.cos.X*c - p.sin.X*s, Y: -p.cos.Y*c - p.sin.Y*s}
	n := math.Hypot(v.X, v.Y)
	if n == 0 {
		return geom.Vec2{}, 0
	}
	bend := (v.X*a.Y - v.Y*a.X) / (n * n * n)
	if p.t1 < p.t0 {
		return geom.Vec2{X: -v.X / n, Y: -v.Y / n}, -bend
	}
	return geom.Vec2{X: v.X / n, Y: v.Y / n}, bend
}

// pieces is the number of equal parts a path is cut into for sampling
// and quadrature: one when it is straight, else parts of a 32nd of a turn.
func (p path) pieces() int {
	if !p.curved() {
		return 1
	}
	return int(math.Ceil(math.Abs(p.t1-p.t0)/(math.Pi/16))) + 1
}

// points samples the path, both ends included.
func (p path) points() []geom.Vec2 {
	n := p.pieces()
	out := make([]geom.Vec2, n+1)
	for i := range out {
		out[i] = p.at(p.t0 + (p.t1-p.t0)*float64(i)/float64(n))
	}
	return out
}

var gaussNodes = [5][2]float64{{0, 0.5688888888888889}, {-0.5384693101056831, 0.4786286704993665}, {0.5384693101056831, 0.4786286704993665}, {-0.9061798459386640, 0.2369268850561891}, {0.9061798459386640, 0.2369268850561891}}

// integral is the line integral of f(q, dq/dt) dt along the path, by
// Gauss quadrature on n equal parts of it.
func (p path) integral(n int, f func(q, d geom.Vec2) float64) float64 {
	h := (p.t1 - p.t0) / float64(n)
	sum := 0.
	for i := 0; i < n; i++ {
		mid := p.t0 + h*(float64(i)+0.5)
		for _, g := range gaussNodes {
			t := mid + g[0]*h/2
			sum += g[1] * h / 2 * f(p.at(t), p.speed(t))
		}
	}
	return sum
}

// sweep is the path's share of the area of a closed loop it is part of.
func (p path) sweep() float64 {
	return p.integral(p.pieces(), func(q, d geom.Vec2) float64 { return (q.X*d.Y - q.Y*d.X) / 2 })
}

// heading is the unit chart direction in which a leg leaves its start,
// and its signed curvature there.
func heading(c chart, l leg) (geom.Vec2, float64) { return c.image(l).heading() }

// chain gives the images of the legs of a loop, each moved by whole
// periods so that it starts where the one before it ended. A loop that
// closes in the chart then ends where it started.
func chain(c chart, loop []leg) []path {
	out := make([]path, len(loop))
	for i, l := range loop {
		out[i] = c.image(l)
		if i > 0 && c.period() > 0 {
			gap := out[i-1].at(out[i-1].t1).X - out[i].at(out[i].t0).X
			out[i] = out[i].moved(c.period() * math.Round(gap/c.period()))
		}
	}
	return out
}

// ring is a loop as a chart polygon, with curved legs sampled.
func ring(c chart, loop []leg) []geom.Vec2 {
	var out []geom.Vec2
	for _, p := range chain(c, loop) {
		points := p.points()
		out = append(out, points[:len(points)-1]...)
	}
	return out
}

// chartArea is the signed chart area the loops bound: outer loops count
// positive and holes negative.
func chartArea(c chart, loops [][]leg) float64 {
	sum := 0.
	for _, loop := range loops {
		for _, p := range chain(c, loop) {
			sum += p.sweep()
		}
	}
	return sum
}

// planeChart is the chart of a plane: the frame coordinates x and y.
type planeChart plane

func (c planeChart) flat(v geom.Vec3) geom.Vec2 {
	return geom.Vec2{X: v.Dot(c.frame.XAxis()), Y: v.Dot(c.frame.YAxis())}
}
func (c planeChart) locate(p geom.Vec3) geom.Vec2 { return c.flat(p.Sub(c.frame.Origin())) }
func (c planeChart) lift(q geom.Vec2) geom.Vec3   { return c.frame.ToWorld(geom.Vec3{X: q.X, Y: q.Y}) }
func (c planeChart) period() float64              { return 0 }
func (c planeChart) image(l leg) path {
	base, lin, cos, sin := l.c.terms()
	return path{c.locate(base), c.flat(lin), c.flat(cos), c.flat(sin), l.t0, l.t1}
}
func (c planeChart) flux(loops [][]leg) float64 {
	return c.frame.Origin().Dot(c.frame.ZAxis()) * chartArea(c, loops) / 3
}
