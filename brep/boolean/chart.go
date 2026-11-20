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
	// chord gives the straight chart segment from p to q as a leg of a
	// carrier in the edge class, when it is one (M6): any chord of a
	// plane, and on a cylinder a chord along a ruling or around a circle.
	chord(p, q geom.Vec2, eps float64) (leg, bool)
	// flux integrates x·n / 3 over the region the loops bound, where n is
	// the carrier normal: the region's share of an enclosed volume.
	flux(loops [][]leg) float64
	// moments gives, in closed form, the area of the region the loops
	// bound and its shares of an enclosed volume and of its first
	// moment, for an outward normal along the carrier normal.
	moments(loops [][]leg) moments
}

// path is the image of a leg in a chart: the point at parameter t is
// base + lin·t + cos·cos t + sin·sin t, for t from t0 to t1. A line or a
// conic in a plane and a generator, a section circle or a section ellipse
// of a cylinder all have this form, and so has the p-curve
// trimmed.EllipseUV. The image of any other leg is free: it gives its
// point and its velocity by evaluation, base is added to the point, and
// the other terms are zero (H1 of specs/general-boolean-quartic).
type path struct {
	base, lin, cos, sin geom.Vec2
	t0, t1              float64
	free                func(t float64) (at, speed geom.Vec2)
}

func (p path) at(t float64) geom.Vec2 {
	if p.free != nil {
		q, _ := p.free(t)
		return geom.Vec2{X: p.base.X + q.X, Y: p.base.Y + q.Y}
	}
	s, c := math.Sincos(t)
	return geom.Vec2{X: p.base.X + p.lin.X*t + p.cos.X*c + p.sin.X*s, Y: p.base.Y + p.lin.Y*t + p.cos.Y*c + p.sin.Y*s}
}

// speed is the derivative of the path by its parameter.
func (p path) speed(t float64) geom.Vec2 {
	if p.free != nil {
		_, v := p.free(t)
		return v
	}
	s, c := math.Sincos(t)
	return geom.Vec2{X: p.lin.X - p.cos.X*s + p.sin.X*c, Y: p.lin.Y - p.cos.Y*s + p.sin.Y*c}
}

// curved reports whether the path is anything but straight.
func (p path) curved() bool {
	return p.free != nil || p.cos != (geom.Vec2{}) || p.sin != (geom.Vec2{})
}

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
	if p.free != nil {
		// The change of the velocity across the start, by a central
		// difference: the carrier runs on either side of the leg.
		const h = 0x1p-17
		before, after := p.speed(p.t0-h), p.speed(p.t0+h)
		a = geom.Vec2{X: (after.X - before.X) / (2 * h), Y: (after.Y - before.Y) / (2 * h)}
	}
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
// and quadrature: one when it is straight, else parts of a 32nd of a
// turn, and of a 64th for a free path.
func (p path) pieces() int {
	switch {
	case !p.curved():
		return 1
	case p.free != nil:
		return int(math.Ceil(math.Abs(p.t1-p.t0)/(math.Pi/32))) + 1
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

// gauss visits the nodes of a Gauss quadrature on n equal parts of the
// path: at each, the point, the velocity and the weight, so that the sum
// of weight·f(point, velocity) is the line integral of f dt.
func (p path) gauss(n int, visit func(q, d geom.Vec2, weight float64)) {
	h := (p.t1 - p.t0) / float64(n)
	for i := 0; i < n; i++ {
		mid := p.t0 + h*(float64(i)+0.5)
		for _, g := range gaussNodes {
			t := mid + g[0]*h/2
			if p.free == nil {
				visit(p.at(t), p.speed(t), g[1]*h/2)
				continue
			}
			q, d := p.free(t)
			visit(geom.Vec2{X: q.X + p.base.X, Y: q.Y + p.base.Y}, d, g[1]*h/2)
		}
	}
}

// integral is the line integral of f(q, dq/dt) dt along the path, by
// Gauss quadrature on n equal parts of it.
func (p path) integral(n int, f func(q, d geom.Vec2) float64) float64 {
	sum := 0.
	p.gauss(n, func(q, d geom.Vec2, weight float64) { sum += weight * f(q, d) })
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

// place lays out the loops of a face: each loop is chained, and on a
// chart that repeats every loop after the first is moved by whole
// periods into the turn of the first, the outer one.
func place(c chart, loops [][]leg) [][]path {
	out := make([][]path, len(loops))
	lo, hi := 0., 0.
	for i, loop := range loops {
		out[i] = chain(c, loop)
		if c.period() == 0 {
			continue
		}
		if i == 0 {
			lo, hi = out[0][0].at(out[0][0].t0).X, out[0][0].at(out[0][0].t0).X
			for _, p := range out[0] {
				for _, q := range p.points() {
					lo, hi = math.Min(lo, q.X), math.Max(hi, q.X)
				}
			}
			continue
		}
		shift := c.period() * math.Round(((lo+hi)/2-out[i][0].at(out[i][0].t0).X)/c.period())
		for k := range out[i] {
			out[i][k] = out[i][k].moved(shift)
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
func (c planeChart) chord(p, q geom.Vec2, _ float64) (leg, bool) {
	from, to := c.lift(p), c.lift(q)
	dir, err := to.Sub(from).Unit()
	return leg{line{from, dir}, 0, to.Distance(from)}, err == nil
}
func (c planeChart) image(l leg) path {
	f, ok := l.c.(termed)
	if !ok {
		return path{t0: l.t0, t1: l.t1, free: func(t float64) (geom.Vec2, geom.Vec2) {
			return c.locate(l.c.at(t)), c.flat(l.c.velocity(t))
		}}
	}
	base, lin, cos, sin := f.terms()
	return path{base: c.locate(base), lin: c.flat(lin), cos: c.flat(cos), sin: c.flat(sin), t0: l.t0, t1: l.t1}
}
func (c planeChart) flux(loops [][]leg) float64 {
	return c.frame.Origin().Dot(c.frame.ZAxis()) * chartArea(c, loops) / 3
}
