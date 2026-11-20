package boolean

import (
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// quartic is the carrier of a closed curve in which two cylinders with
// axes that are not parallel meet (specs/general-boolean-quartic). The
// small cylinder has radius r and the large one radius R. p is the point
// of the small axis nearest the large axis, n the unit vector along both
// perpendiculars, and d the distance from the large axis to the small one
// along n. θ is the angle of a point about the small axis and ψ its angle
// about the large one, both from n. A point of the small cylinder at
// angle θ and height v above p is on the large one when
//
//	d + r·cos θ = R·cos ψ  and  v = (r·cos·sin θ − R·sin ψ) / sin,
//
// where cos and sin are those of the angle between the axes.
//
// Where every ruling of the small cylinder passes through the large one
// (a full pair) there are two curves, one for each sign of sin ψ, and the
// parameter of each is θ. Otherwise (a partial pair) there is one curve,
// which goes round neither axis. With x and y the angles θ and ψ counted
// from the middle of their ranges it is r·cos x + R·cos y = |d|, an oval
// about the origin that every ray from the origin meets once, and its
// parameter is the polar angle of (x/xm, y/ym), where xm and ym are the
// greatest x and y on it.
type quartic struct {
	small, large cylinder
	p, n, m, z   geom.Vec3 // m = z × n, and z is the small axis
	r, R, d      float64
	cos, sin     float64
	side         float64 // full pair: the sign of sin ψ; partial pair: 0
	xm, ym       float64 // partial pair: the greatest x and y
	x0, y0       float64 // partial pair: the middle of θ and of ψ
	back         bool    // the parameter runs the other way
}

// meetingCurves is the row of two cylinders whose axes are not parallel
// and that are not an equal pair. A pair in scope gives its curves; any
// other is open, with the bound it fails as the reason.
func meetingCurves(a, b cylinder, t tolerances) pairing {
	small, large := a, b
	if b.radius < a.radius {
		small, large = b, a
	}
	z, zl := small.frame.ZAxis(), large.frame.ZAxis()
	cross := zl.Cross(z)
	sin := cross.Length()
	if sin < minCrossingSine-t.angular {
		return pairing{open: true, reason: "whose axes are too nearly parallel"}
	}
	p, pl := nearestOnLines(small.axis(), large.axis())
	q := quartic{small: small, large: large, p: p, n: cross.Scale(1 / sin), z: z, r: small.radius, R: large.radius, cos: z.Dot(zl), sin: sin}
	q.m = z.Cross(q.n)
	q.d = p.Sub(pl).Dot(q.n)
	reach := math.Abs(q.d)
	// The sine of the angle whose cosine is k is at least the bound.
	regular := func(k float64) bool { return (1-k)*(1+k) >= minCrossingSine*minCrossingSine }
	switch {
	case reach+q.r < q.R:
		if !regular((reach + q.r) / q.R) {
			return pairing{open: true, reason: "that nearly touch from inside"}
		}
		upper, lower := q, q
		upper.side, lower.side = 1, -1
		return pairing{curves: []curve{&upper, &lower}, touching: []bool{false, false}}
	case reach > q.R-q.r && reach < q.R+q.r:
		kx, ky := (reach-q.R)/q.r, (reach-q.r)/q.R
		if !regular(kx) || !regular(ky) {
			return pairing{open: true, reason: "that nearly touch"}
		}
		q.xm, q.ym = math.Acos(kx), math.Acos(ky)
		if q.d > 0 {
			q.x0 = math.Pi
		} else {
			q.y0 = math.Pi
		}
		return pairing{curves: []curve{&q}, touching: []bool{false}}
	case reach <= q.R+q.r+t.eps:
		return pairing{open: true, reason: "that touch"}
	}
	return pairing{open: true}
}

func (q *quartic) family() family  { return quarticFamily }
func (q *quartic) period() float64 { return 2 * math.Pi }
func (q *quartic) flipped() curve {
	out := *q
	out.back = !q.back
	return &out
}

// angles gives θ and ψ at parameter t, and their derivatives by it.
func (q *quartic) angles(t float64) (theta, psi, dtheta, dpsi float64) {
	way := 1.
	if q.back {
		t, way = -t, -1
	}
	if q.side != 0 {
		st, ct := math.Sincos(t)
		k := (q.d + q.r*ct) / q.R
		s := q.side * math.Sqrt((1-k)*(1+k))
		return t, math.Atan2(s, k), way, way * q.r * st / (q.R * s)
	}
	// The point of the oval on the ray through (xm·cos t, ym·sin t). Along
	// the ray g falls from r + R − |d| to zero and below, so a Newton
	// step that leaves the bracket is replaced by its middle.
	st, ct := math.Sincos(t)
	a, b := q.xm*ct, q.ym*st
	reach := math.Abs(q.d)
	lo, hi := 0., 1/math.Max(math.Abs(ct), math.Abs(st))
	l := math.Min(1, hi)
	var fx, fy float64
	for range 64 {
		sx, cx := math.Sincos(l * a)
		sy, cy := math.Sincos(l * b)
		g := q.r*cx + q.R*cy - reach
		fx, fy = -q.r*sx, -q.R*sy
		if g > 0 {
			lo = l
		} else {
			hi = l
		}
		next := l - g/(fx*a+fy*b)
		if !(next > lo && next < hi) {
			next = lo + (hi-lo)/2
		}
		done := math.Abs(next-l) <= 0x1p-50*hi
		if l = next; done {
			break
		}
	}
	da, db := -q.xm*st, q.ym*ct
	sx, _ := math.Sincos(l * a)
	sy, _ := math.Sincos(l * b)
	fx, fy = -q.r*sx, -q.R*sy
	dl := -l * (fx*da + fy*db) / (fx*a + fy*b)
	return q.x0 + l*a, q.y0 + l*b, way * (dl*a + l*da), way * (dl*b + l*db)
}

// state gives the point of the carrier at parameter t and its velocity.
func (q *quartic) state(t float64) (at, velocity geom.Vec3) {
	theta, psi, dtheta, dpsi := q.angles(t)
	st, ct := math.Sincos(theta)
	sp, cp := math.Sincos(psi)
	v := (q.r*q.cos*st - q.R*sp) / q.sin
	dv := (q.r*q.cos*ct*dtheta - q.R*cp*dpsi) / q.sin
	at = q.p.Add(q.n.Scale(q.r * ct)).Add(q.m.Scale(q.r * st)).Add(q.z.Scale(v))
	velocity = q.n.Scale(-q.r * st * dtheta).Add(q.m.Scale(q.r * ct * dtheta)).Add(q.z.Scale(dv))
	return at, velocity
}

func (q *quartic) at(t float64) geom.Vec3 {
	p, _ := q.state(t)
	return p
}

func (q *quartic) velocity(t float64) geom.Vec3 {
	_, v := q.state(t)
	return v
}

func (q *quartic) tangent(t float64) geom.Vec3 {
	v := q.velocity(t)
	return v.Scale(1 / v.Length())
}

// about gives the angle of a vector across an axis from n, towards the
// cross product of the axis and n.
func (q *quartic) about(axis, v geom.Vec3) float64 {
	return math.Atan2(v.Dot(axis.Cross(q.n)), v.Dot(q.n))
}

// param inverts a point to its parameter: from the angles of the point
// about the two axes, improved by steps towards the nearest point of the
// carrier.
func (q *quartic) param(p geom.Vec3) float64 {
	t := q.about(q.z, p.Sub(q.p))
	if q.side == 0 {
		large := q.large.axis()
		psi := q.about(large.dir, p.Sub(large.at(large.param(q.p))))
		x, y := math.Remainder(t-q.x0, 2*math.Pi), math.Remainder(psi-q.y0, 2*math.Pi)
		t = math.Atan2(y/q.ym, x/q.xm)
	}
	if q.back {
		t = -t
	}
	for range 3 {
		at, v := q.state(t)
		t -= at.Sub(p).Dot(v) / v.Dot(v)
	}
	if t = math.Mod(t, 2*math.Pi); t < 0 {
		t += 2 * math.Pi
	}
	return t
}

// turn gives the angle of the carrier point at parameter t about the axis
// of one of its two cylinders, from the x axis of that cylinder's frame,
// as a function that is continuous in t: it does not jump where the
// angle a chart reports passes half a turn. s may be either cylinder
// under another frame about the same axis.
func (q *quartic) turn(s cylinder, t float64) float64 {
	theta, psi, _, _ := q.angles(t)
	z, own, angle := s.frame.ZAxis(), q.z, theta
	if large := q.large.frame.ZAxis(); math.Abs(z.Dot(large)) > math.Abs(z.Dot(own)) {
		own, angle = large, psi
	}
	from := math.Atan2(q.n.Dot(s.frame.YAxis()), q.n.Dot(s.frame.XAxis()))
	if z.Dot(own) < 0 {
		return from - angle
	}
	return from + angle
}

// box holds the part of the carrier from t0 to t1: its samples at steps
// of at most a 64th of a turn, grown by what the carrier can leave the
// chord between two of them, which is less than half the length it runs
// between them.
func (q *quartic) box(t0, t1 float64) bounds {
	n := int(math.Ceil(math.Abs(t1-t0)/(math.Pi/32))) + 1
	step := (t1 - t0) / float64(n)
	first, fast := q.state(t0)
	out, speed := around(first), fast.Length()
	for k := 1; k <= n; k++ {
		p, v := q.state(t0 + step*float64(k))
		out, speed = out.with(around(p)), math.Max(speed, v.Length())
	}
	return out.grown(speed * math.Abs(step))
}

func (q *quartic) value(_, _ geom.Vec3, t0, t1 float64) (geom.Curve, parametric.Interval, error) {
	return meetingCurve{q}, parametric.Interval{Min: t0, Max: t1}, nil
}

// meetingCurve is the curve an edge on a quartic carrier stores: the
// carrier by its own parameter.
type meetingCurve struct{ q *quartic }

func (c meetingCurve) Eval(t float64) (geom.Vec3, error)       { return c.q.at(t), nil }
func (c meetingCurve) Derivative(t float64) (geom.Vec3, error) { return c.q.velocity(t), nil }
