// Package meeting holds the closed curves in which two circular cylinders
// whose axes are not parallel meet, other than the two ellipses of two
// cylinders of one radius whose axes cross
// (specs/general-boolean-quartic). The general Boolean cuts along them,
// and STEP exchange rebuilds them from the two cylinders a file names.
package meeting

import (
	"math"

	"plainsolid/geom"
)

// minSine (OD-Q1 of the specification) is the least sine of the angle between the two axes, and of
// each angle at which a curve turns back about an axis, for which Between
// gives curves.
const minSine = 1. / 64

// Curve is one closed curve in which two cylinders meet. The small
// cylinder has radius r and the large one radius R. p is the point of the
// small axis nearest the large axis, n the unit vector along both
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
// greatest x and y on it. Either way the parameter has period 2π.
type Curve struct {
	p, n, m, z geom.Vec3 // m = z × n, and z is the small axis
	pl, zl     geom.Vec3 // the point of the large axis nearest p, and that axis
	r, R, d    float64
	cos, sin   float64
	side       float64 // full pair: the sign of sin ψ; partial pair: 0
	xm, ym     float64 // partial pair: the greatest x and y
	x0, y0     float64 // partial pair: the middle of θ and of ψ
	back       bool    // the parameter runs the other way
}

// Between gives the curves in which two cylinders meet: each cylinder is
// the one of its radius about the z axis of its frame. angular is the
// tolerance of an angle and eps that of a length. Where it gives no
// curve, reason says which bound the pair fails, in words that follow
// "cylinders"; it is empty for two cylinders that are apart, and for two
// of one radius whose axes cross it says that they touch, since their
// curves are ellipses and not these.
func Between(a geom.Frame, ra float64, b geom.Frame, rb float64, angular, eps float64) (curves []Curve, reason string) {
	small, large := a, b
	r, R := ra, rb
	if rb < ra {
		small, large, r, R = b, a, rb, ra
	}
	z, zl := small.ZAxis(), large.ZAxis()
	cross := zl.Cross(z)
	sin := cross.Length()
	if sin < minSine-angular {
		return nil, "whose axes are too nearly parallel"
	}
	// The nearest points of the two axes.
	w := small.Origin().Sub(large.Origin())
	k, e, f := z.Dot(zl), z.Dot(w), zl.Dot(w)
	p := small.Origin().Add(z.Scale((k*f - e) / (1 - k*k)))
	pl := large.Origin().Add(zl.Scale((f - k*e) / (1 - k*k)))
	c := Curve{p: p, n: cross.Scale(1 / sin), z: z, pl: pl, zl: zl, r: r, R: R, cos: k, sin: sin}
	c.m = z.Cross(c.n)
	c.d = p.Sub(pl).Dot(c.n)
	reach := math.Abs(c.d)
	// The sine of the angle whose cosine is k is at least the bound.
	regular := func(k float64) bool { return (1-k)*(1+k) >= minSine*minSine }
	switch {
	case reach+r < R:
		if !regular((reach + r) / R) {
			return nil, "that nearly touch from inside"
		}
		upper, lower := c, c
		upper.side, lower.side = 1, -1
		return []Curve{upper, lower}, ""
	case reach > R-r && reach < R+r:
		kx, ky := (reach-R)/r, (reach-r)/R
		if !regular(kx) || !regular(ky) {
			return nil, "that nearly touch"
		}
		c.xm, c.ym = math.Acos(kx), math.Acos(ky)
		if c.d > 0 {
			c.x0 = math.Pi
		} else {
			c.y0 = math.Pi
		}
		return []Curve{c}, ""
	case reach <= R+r+eps:
		return nil, "that touch"
	}
	return nil, ""
}

// Reversed is the same curve with its parameter running the other way.
func (c Curve) Reversed() Curve {
	c.back = !c.back
	return c
}

// angles gives θ and ψ at parameter t, and their derivatives by it.
func (c Curve) angles(t float64) (theta, psi, dtheta, dpsi float64) {
	way := 1.
	if c.back {
		t, way = -t, -1
	}
	if c.side != 0 {
		st, ct := math.Sincos(t)
		k := (c.d + c.r*ct) / c.R
		s := c.side * math.Sqrt((1-k)*(1+k))
		return t, math.Atan2(s, k), way, way * c.r * st / (c.R * s)
	}
	// The point of the oval on the ray through (xm·cos t, ym·sin t). Along
	// the ray g falls from r + R − |d| to zero and below, so a Newton
	// step that leaves the bracket is replaced by its middle.
	st, ct := math.Sincos(t)
	a, b := c.xm*ct, c.ym*st
	reach := math.Abs(c.d)
	lo, hi := 0., 1/math.Max(math.Abs(ct), math.Abs(st))
	l := math.Min(1, hi)
	var fx, fy float64
	for range 64 {
		sx, cx := math.Sincos(l * a)
		sy, cy := math.Sincos(l * b)
		g := c.r*cx + c.R*cy - reach
		fx, fy = -c.r*sx, -c.R*sy
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
	da, db := -c.xm*st, c.ym*ct
	sx, _ := math.Sincos(l * a)
	sy, _ := math.Sincos(l * b)
	fx, fy = -c.r*sx, -c.R*sy
	dl := -l * (fx*da + fy*db) / (fx*a + fy*b)
	return c.x0 + l*a, c.y0 + l*b, way * (dl*a + l*da), way * (dl*b + l*db)
}

// State gives the point of the curve at parameter t and its derivative
// by the parameter.
func (c Curve) State(t float64) (at, velocity geom.Vec3) {
	theta, psi, dtheta, dpsi := c.angles(t)
	st, ct := math.Sincos(theta)
	sp, cp := math.Sincos(psi)
	v := (c.r*c.cos*st - c.R*sp) / c.sin
	dv := (c.r*c.cos*ct*dtheta - c.R*cp*dpsi) / c.sin
	at = c.p.Add(c.n.Scale(c.r * ct)).Add(c.m.Scale(c.r * st)).Add(c.z.Scale(v))
	velocity = c.n.Scale(-c.r * st * dtheta).Add(c.m.Scale(c.r * ct * dtheta)).Add(c.z.Scale(dv))
	return at, velocity
}

// Eval and Derivative make a Curve the curve an edge stores.
func (c Curve) Eval(t float64) (geom.Vec3, error) {
	p, _ := c.State(t)
	return p, nil
}

func (c Curve) Derivative(t float64) (geom.Vec3, error) {
	_, v := c.State(t)
	return v, nil
}

// about gives the angle of a vector across an axis from n, towards the
// cross product of the axis and n.
func (c Curve) about(axis, v geom.Vec3) float64 {
	return math.Atan2(v.Dot(axis.Cross(c.n)), v.Dot(c.n))
}

// Param inverts a point to its parameter, in [0, 2π): from the angles of
// the point about the two axes, improved by steps towards the nearest
// point of the curve.
func (c Curve) Param(p geom.Vec3) float64 {
	t := c.about(c.z, p.Sub(c.p))
	if c.side == 0 {
		psi := c.about(c.zl, p.Sub(c.pl))
		x, y := math.Remainder(t-c.x0, 2*math.Pi), math.Remainder(psi-c.y0, 2*math.Pi)
		t = math.Atan2(y/c.ym, x/c.xm)
	}
	if c.back {
		t = -t
	}
	for range 3 {
		at, v := c.State(t)
		t -= at.Sub(p).Dot(v) / v.Dot(v)
	}
	if t = math.Mod(t, 2*math.Pi); t < 0 {
		t += 2 * math.Pi
	}
	return t
}

// Turn gives the angle of the point at parameter t about the axis of one
// of the two cylinders, from the x axis of a frame about that axis, as a
// function that is continuous in t: it does not jump where the angle a
// chart reports passes half a turn. The frame may be any about either
// axis, whichever way its z axis points along it.
func (c Curve) Turn(f geom.Frame, t float64) float64 {
	theta, psi, _, _ := c.angles(t)
	z, own, angle := f.ZAxis(), c.z, theta
	if math.Abs(z.Dot(c.zl)) > math.Abs(z.Dot(own)) {
		own, angle = c.zl, psi
	}
	from := math.Atan2(c.n.Dot(f.YAxis()), c.n.Dot(f.XAxis()))
	if z.Dot(own) < 0 {
		return from - angle
	}
	return from + angle
}
