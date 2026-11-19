package boolean

import (
	"math"

	"plainsolid/geom"
)

// The closed forms of CC1 that package geom does not have.

// circleMeeting is how two coplanar circles meet.
type circleMeeting struct {
	points   []geom.Vec2 // 0, 1 or 2 common points
	touching bool        // the one point is a tangency
	same     bool        // the circles are one circle within tolerance
}

// circleWithCircle meets two circles of one plane, given in coordinates
// of that plane. Circles within eps of tangency touch at one point.
func circleWithCircle(c1 geom.Vec2, r1 float64, c2 geom.Vec2, r2, eps float64) circleMeeting {
	dx, dy := c2.X-c1.X, c2.Y-c1.Y
	d := math.Hypot(dx, dy)
	switch {
	case d <= eps && math.Abs(r1-r2) <= eps:
		return circleMeeting{same: true}
	case d > r1+r2+eps || d < math.Abs(r1-r2)-eps || d <= eps:
		return circleMeeting{}
	}
	ux, uy := dx/d, dy/d
	if math.Abs(d-(r1+r2)) <= eps || math.Abs(d-math.Abs(r1-r2)) <= eps {
		// Tangent from outside, or one inside the other: the point lies
		// on the line of centres, beyond the smaller centre when inside.
		along := r1
		if d < math.Max(r1, r2) && r1 < r2 {
			along = -r1
		}
		return circleMeeting{points: []geom.Vec2{{X: c1.X + along*ux, Y: c1.Y + along*uy}}, touching: true}
	}
	a := (d*d + r1*r1 - r2*r2) / (2 * d)
	h := math.Sqrt(math.Max(r1*r1-a*a, 0))
	return circleMeeting{points: []geom.Vec2{
		{X: c1.X + a*ux - h*uy, Y: c1.Y + a*uy + h*ux},
		{X: c1.X + a*ux + h*uy, Y: c1.Y + a*uy - h*ux},
	}}
}

// homothetic reports whether two coplanar conics have one shape and one
// attitude (CC3): equal ratios of their radii, and either both circles or
// parallel major axes.
func (t tolerances) homothetic(c, d conic) bool {
	a1, b1, x1 := c.axes()
	a2, b2, x2 := d.axes()
	if math.Abs(a1*b2-a2*b1) > t.eps*math.Max(a1, a2) {
		return false
	}
	return a1-b1 <= t.eps && a2-b2 <= t.eps || t.parallel(x1, x2)
}

// asCircles applies the homothetic reduction of CC3: in the coordinates
// of conic c, stretched along its minor axis by the ratio of its radii,
// c and every conic homothetic to it are circles. It returns the centre
// and radius of d there and the map from those coordinates back to
// space.
func asCircles(c, d conic) (centre geom.Vec2, radius float64, back func(geom.Vec2) geom.Vec3) {
	major, minor, along := c.axes()
	side := c.normal().Cross(along)
	stretch := major / minor
	off := d.center.Sub(c.center)
	a2, _, _ := d.axes()
	centre = geom.Vec2{X: off.Dot(along), Y: off.Dot(side) * stretch}
	return centre, a2, func(q geom.Vec2) geom.Vec3 {
		return c.center.Add(along.Scale(q.X)).Add(side.Scale(q.Y / stretch))
	}
}

// shadow is the projection of a conic along the axis of a cylinder onto
// its cross-section (CC5). The projection is a circle when the projected
// axis vectors have one length within eps and are perpendicular within
// the angular tolerance; its radius is then the mean of the two lengths.
func (t tolerances) shadow(c conic, s cylinder) (centre geom.Vec2, radius float64, round bool) {
	ch := cylinderChart(s)
	flat := func(v geom.Vec3) geom.Vec2 {
		return geom.Vec2{X: v.Dot(ch.frame.XAxis()), Y: v.Dot(ch.frame.YAxis())}
	}
	u, v := flat(c.x.Scale(c.a)), flat(c.y.Scale(c.b))
	lu, lv := math.Hypot(u.X, u.Y), math.Hypot(v.X, v.Y)
	if lu == 0 || lv == 0 {
		return geom.Vec2{}, 0, false
	}
	round = math.Abs(lu-lv) <= t.eps && math.Abs(u.X*v.X+u.Y*v.Y)/(lu*lv) <= t.angular
	return flat(c.center.Sub(s.frame.Origin())), (lu + lv) / 2, round
}

// segmentDistance is the least distance between two segments, and the
// distance from a point to a segment when the first is a single point.
func segmentDistance(p0, p1, q0, q1 geom.Vec3) float64 {
	d1, d2, r := p1.Sub(p0), q1.Sub(q0), p0.Sub(q0)
	a, e, f := d1.Dot(d1), d2.Dot(d2), d2.Dot(r)
	clamp := func(x float64) float64 { return math.Max(0, math.Min(1, x)) }
	var s, u float64
	switch {
	case a == 0 && e == 0:
	case a == 0:
		u = clamp(f / e)
	default:
		c := d1.Dot(r)
		if e == 0 {
			s = clamp(-c / a)
		} else {
			b := d1.Dot(d2)
			if den := a*e - b*b; den != 0 {
				s = clamp((b*f - c*e) / den)
			}
			if u = (b*s + f) / e; u < 0 {
				u, s = 0, clamp(-c/a)
			} else if u > 1 {
				u, s = 1, clamp((b-c)/a)
			}
		}
	}
	return p0.Add(d1.Scale(s)).Distance(q0.Add(d2.Scale(u)))
}

// conicThrough fits the conic of a plane through five points of it: the
// circle through the first three when the other two lie on it within
// eps, and otherwise the ellipse through all five. It returns false when
// the five points lie on no ellipse.
func conicThrough(points [5]geom.Vec3, eps float64) (conic, bool) {
	// Coordinates from the mean of the points, which lies strictly inside
	// any ellipse through them, so the conic below does not pass through
	// the origin.
	var p0 geom.Vec3
	for _, p := range points {
		p0 = p0.Add(p.Scale(0.2))
	}
	u, err := points[1].Sub(points[0]).Unit()
	if err != nil {
		return conic{}, false
	}
	n, err := u.Cross(points[2].Sub(points[0])).Unit()
	if err != nil {
		return conic{}, false
	}
	v := n.Cross(u)
	var q [5]geom.Vec2
	for i, p := range points {
		q[i] = geom.Vec2{X: p.Sub(p0).Dot(u), Y: p.Sub(p0).Dot(v)}
	}
	lift := func(c geom.Vec2) geom.Vec3 { return p0.Add(u.Scale(c.X)).Add(v.Scale(c.Y)) }
	// The circle through the first three points: its centre is equally
	// far from the first and second and from the first and third.
	b, c := geom.Vec2{X: q[1].X - q[0].X, Y: q[1].Y - q[0].Y}, geom.Vec2{X: q[2].X - q[0].X, Y: q[2].Y - q[0].Y}
	if det := 2 * (b.X*c.Y - b.Y*c.X); det != 0 {
		centre := geom.Vec2{X: q[0].X + (c.Y*(b.X*b.X+b.Y*b.Y)-b.Y*(c.X*c.X+c.Y*c.Y))/det, Y: q[0].Y + (b.X*(c.X*c.X+c.Y*c.Y)-c.X*(b.X*b.X+b.Y*b.Y))/det}
		r := math.Hypot(q[0].X-centre.X, q[0].Y-centre.Y)
		if math.Abs(math.Hypot(q[3].X-centre.X, q[3].Y-centre.Y)-r) <= eps && math.Abs(math.Hypot(q[4].X-centre.X, q[4].Y-centre.Y)-r) <= eps {
			return conic{lift(centre), u, v, r, r}, true
		}
	}
	// The general conic A x² + B xy + C y² + D x + E y = 1 through five
	// points, solved by elimination with pivoting.
	var m [5][6]float64
	for i, p := range q {
		m[i] = [6]float64{p.X * p.X, p.X * p.Y, p.Y * p.Y, p.X, p.Y, 1}
	}
	for col := 0; col < 5; col++ {
		pivot := col
		for row := col + 1; row < 5; row++ {
			if math.Abs(m[row][col]) > math.Abs(m[pivot][col]) {
				pivot = row
			}
		}
		if m[pivot][col] == 0 {
			return conic{}, false
		}
		m[col], m[pivot] = m[pivot], m[col]
		for row := 0; row < 5; row++ {
			if row != col {
				f := m[row][col] / m[col][col]
				for k := col; k < 6; k++ {
					m[row][k] -= f * m[col][k]
				}
			}
		}
	}
	A, B, C, D, E := m[0][5]/m[0][0], m[1][5]/m[1][1], m[2][5]/m[2][2], m[3][5]/m[3][3], m[4][5]/m[4][4]
	disc := 4*A*C - B*B
	if !(disc > 0) {
		return conic{}, false
	}
	centre := geom.Vec2{X: (B*E - 2*C*D) / disc, Y: (B*D - 2*A*E) / disc}
	// About the centre the conic is A x² + B xy + C y² = k.
	k := 1 - (A*centre.X*centre.X + B*centre.X*centre.Y + C*centre.Y*centre.Y + D*centre.X + E*centre.Y)
	turn := math.Atan2(B, A-C) / 2
	s, cs := math.Sincos(turn)
	l1, l2 := A*cs*cs+B*cs*s+C*s*s, A*s*s-B*cs*s+C*cs*cs
	if !(k/l1 > 0) || !(k/l2 > 0) {
		return conic{}, false
	}
	x := u.Scale(cs).Add(v.Scale(s))
	return conic{lift(centre), x, n.Cross(x), math.Sqrt(k / l1), math.Sqrt(k / l2)}, true
}

// nearestOnLines gives the points of two lines that are not parallel at
// which they come nearest to each other.
func nearestOnLines(a, b line) (geom.Vec3, geom.Vec3) {
	w := a.origin.Sub(b.origin)
	k, d, e := a.dir.Dot(b.dir), a.dir.Dot(w), b.dir.Dot(w)
	return a.at((k*e - d) / (1 - k*k)), b.at((e - k*d) / (1 - k*k))
}
