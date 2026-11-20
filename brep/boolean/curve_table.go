package boolean

import (
	"math"

	"plainsolid/geom"
)

// parallel reports whether two unit directions are parallel under T5.
func (t tolerances) parallel(a, b geom.Vec3) bool { return a.Cross(b).Length() <= t.angular }

// perpendicular reports whether two unit directions are perpendicular
// under T5.
func (t tolerances) perpendicular(a, b geom.Vec3) bool { return math.Abs(a.Dot(b)) <= t.angular }

// carrier is a curve or a surface carrier: an entry of the curve table.
type carrier interface{ family() family }

// hit is an isolated point where a curve meets a curve or a surface.
type hit struct {
	p        geom.Vec3
	t        float64 // parameter on the curve
	s        float64 // parameter on the other curve, for two curves
	gap      float64 // distance between the two carriers at the point
	touching bool    // the curve does not pass through the surface (CC2)
}

// meeting is what the table of section 7.5 gives for a curve with a
// curve or a surface.
type meeting struct {
	hits []hit
	// alike is set when the carriers may coincide: two curves are
	// candidates for an overlap (CC4), or a curve may lie in a surface.
	// The caller then decides on the nodes of the edge.
	alike bool
	// open is set when the table has no closed form for the pair, and
	// the curve must be proved clear of the face (CC5).
	open bool
}

// curveRow is one row of the table of section 7.5: a curve family met
// with a curve or surface family.
type curveRow struct {
	curve, with family
	meet        func(c curve, with carrier, t tolerances) (meeting, error)
}

var curveRows = []curveRow{
	{lineFamily, lineFamily, lineWithLine},
	{lineFamily, planeFamily, lineWithPlane},
	{lineFamily, cylinderFamily, lineWithCylinder},
	{lineFamily, conicFamily, lineWithConic},
	{conicFamily, planeFamily, conicWithPlane},
	{conicFamily, cylinderFamily, conicWithCylinder},
	{conicFamily, conicFamily, conicWithConic},
	{quarticFamily, planeFamily, quarticWithSurface},
	{quarticFamily, cylinderFamily, quarticWithSurface},
	{quarticFamily, lineFamily, quarticWithCurve},
	{quarticFamily, conicFamily, quarticWithCurve},
	{quarticFamily, quarticFamily, quarticWithCurve},
}

// curveMeets resolves a curve with a curve or a surface by the table. Two
// curves may stand in either order in a row. A pair with no row is
// unsupported.
func curveMeets(c curve, with carrier, t tolerances) (meeting, error) {
	for _, r := range curveRows {
		if r.curve == c.family() && r.with == with.family() {
			return r.meet(c, with, t)
		}
	}
	if other, ok := with.(curve); ok {
		for _, r := range curveRows {
			if r.curve == other.family() && r.with == c.family() {
				m, err := r.meet(other, c, t)
				for i := range m.hits {
					m.hits[i].t, m.hits[i].s = m.hits[i].s, m.hits[i].t
				}
				return m, err
			}
		}
	}
	return meeting{}, fail(ErrUnsupported, "curve table", "no row for carrier families %d and %d", c.family(), with.family())
}

// lineWithLine gives the closest points of two lines that are not
// parallel, and marks parallel lines as candidates for an overlap.
func lineWithLine(c curve, with carrier, t tolerances) (meeting, error) {
	a, b := c.(line), with.(line)
	if t.parallel(a.dir, b.dir) {
		return meeting{alike: true}, nil
	}
	w, dot := a.origin.Sub(b.origin), a.dir.Dot(b.dir)
	d, e := a.dir.Dot(w), b.dir.Dot(w)
	u, v := (dot*e-d)/(1-dot*dot), (e-dot*d)/(1-dot*dot)
	p, q := a.at(u), b.at(v)
	return meeting{hits: []hit{{p: p.Add(q).Scale(0.5), t: u, s: v, gap: p.Distance(q)}}}, nil
}

// lineWithPlane gives the point where a line passes through a plane, and
// marks a line parallel to the plane as one that may lie in it.
func lineWithPlane(c curve, with carrier, t tolerances) (meeting, error) {
	l, s := c.(line), with.(plane)
	if t.perpendicular(l.dir, s.frame.ZAxis()) {
		return meeting{alike: true}, nil
	}
	gl, err := geom.NewLine(l.origin, l.dir)
	if err != nil {
		return meeting{}, failWith(ErrInvalid, "curve table", err)
	}
	gp, err := geom.NewPlane(s.frame.Origin(), s.frame.ZAxis())
	if err != nil {
		return meeting{}, failWith(ErrInvalid, "curve table", err)
	}
	r, err := geom.IntersectLinePlane(gl, gp, geom.IntersectionTolerance{Linear: t.eps, Angular: t.angular})
	if err != nil {
		return meeting{}, failWith(ErrInvalid, "curve table", err)
	}
	if r.Kind != geom.IntersectionPoint {
		return meeting{alike: true}, nil
	}
	return meeting{hits: []hit{{p: r.Point, t: r.Parameter}}}, nil
}

// lineInConicPlane meets a line with a conic of its own plane. A line
// within eps of tangency touches at one point. In a hit, t is the
// parameter on the line and s the parameter on the conic.
func lineInConicPlane(l line, c conic, t tolerances) []hit {
	// In the conic's coordinates, each scaled by its radius, the conic is
	// the unit circle.
	rel := l.origin.Sub(c.center)
	ox, oy := rel.Dot(c.x)/c.a, rel.Dot(c.y)/c.b
	dx, dy := l.dir.Dot(c.x)/c.a, l.dir.Dot(c.y)/c.b
	qa, qb := dx*dx+dy*dy, ox*dx+oy*dy
	if qa == 0 {
		return nil
	}
	near := -qb / qa
	fx, fy := ox+dx*near, oy+dy*near
	m := math.Hypot(fx, fy)
	at := func(u float64, touching bool) hit {
		p := l.at(u)
		return hit{p: p, t: u, s: c.param(p), gap: off(c, p), touching: touching}
	}
	if m > 0 {
		// The gap between the foot of the centre on the line and the
		// conic point in its direction.
		if gap := math.Abs(1-1/m) * math.Hypot(c.a*fx/m, c.b*fy/m); gap <= t.eps {
			return []hit{at(near, true)}
		}
	}
	if m >= 1 {
		return nil
	}
	half := math.Sqrt((1 - m*m) / qa)
	return []hit{at(near-half, false), at(near+half, false)}
}

// lineWithConic meets a line with a circle or ellipse: through the
// conic's plane at one point, or within it.
func lineWithConic(c curve, with carrier, t tolerances) (meeting, error) {
	l, k := c.(line), with.(conic)
	n := k.normal()
	height := l.origin.Sub(k.center).Dot(n)
	if t.perpendicular(l.dir, n) {
		if math.Abs(height) > t.eps {
			return meeting{}, nil
		}
		return meeting{hits: lineInConicPlane(l, k, t)}, nil
	}
	u := -height / l.dir.Dot(n)
	p := l.at(u)
	return meeting{hits: []hit{{p: p, t: u, s: k.param(p), gap: off(k, p)}}}, nil
}

// lineWithCylinder meets a line with a cylinder. A line parallel to the
// axis may lie on the surface. Any other passes through at two points,
// touches at one when it runs within eps of the surface, or misses.
func lineWithCylinder(c curve, with carrier, t tolerances) (meeting, error) {
	l, s := c.(line), with.(cylinder)
	if t.parallel(l.dir, s.frame.ZAxis()) {
		return meeting{alike: true}, nil
	}
	o, d := s.across(l.origin.Sub(s.frame.Origin())), s.across(l.dir)
	near := -o.Dot(d) / d.Dot(d)
	reach := o.Add(d.Scale(near)).Length()
	switch {
	case math.Abs(reach-s.radius) <= t.eps:
		return meeting{hits: []hit{{p: l.at(near), t: near, gap: math.Abs(reach - s.radius), touching: true}}}, nil
	case reach > s.radius:
		return meeting{}, nil
	}
	if gl, err := geom.NewLine(l.origin, l.dir); err == nil {
		if gc, err := geom.NewCylinderSurface(s.frame, s.radius); err == nil {
			r, err := geom.IntersectLineCylinder(gl, gc, geom.LineSurfaceOptions{DistanceTolerance: t.eps})
			if found := r.Hits; err == nil && len(found) == 2 {
				return meeting{hits: []hit{{p: found[0].Point, t: found[0].Parameter}, {p: found[1].Point, t: found[1].Parameter}}}, nil
			}
		}
	}
	half := math.Sqrt((s.radius*s.radius - reach*reach) / d.Dot(d))
	return meeting{hits: []hit{{p: l.at(near - half), t: near - half}, {p: l.at(near + half), t: near + half}}}, nil
}

// planeOf is the plane of a conic as a carrier.
func planeOf(k conic) (plane, error) {
	frame, err := geom.NewFrame(k.center, k.x, k.normal())
	return plane{frame}, err
}

// conicWithPlane meets a circle or ellipse with a plane: along the line
// of the two planes, where the conic passes through, touches or misses.
// A conic in a parallel plane may lie in the plane.
func conicWithPlane(c curve, with carrier, t tolerances) (meeting, error) {
	k, s := c.(conic), with.(plane)
	own, err := planeOf(k)
	if err != nil {
		return meeting{}, failWith(ErrInvalid, "curve table", err)
	}
	pair, err := planeWithPlane(own, s, t)
	if err != nil || pair.alike {
		return meeting{alike: pair.alike}, err
	}
	var m meeting
	for _, h := range lineInConicPlane(pair.curves[0].(line), k, t) {
		h.t, h.s = h.s, 0
		m.hits = append(m.hits, h)
	}
	return m, nil
}

// conicWithCylinder meets a circle or ellipse with a cylinder. When the
// conic's plane is parallel to the axis, the plane cuts generators and
// each is met with the conic. When the conic projects to a circle along
// the axis, two circles are met in the cross-section. No other case has
// a closed form here.
func conicWithCylinder(c curve, with carrier, t tolerances) (meeting, error) {
	k, s := c.(conic), with.(cylinder)
	if t.perpendicular(k.normal(), s.frame.ZAxis()) {
		own, err := planeOf(k)
		if err != nil {
			return meeting{}, failWith(ErrInvalid, "curve table", err)
		}
		pair, err := planeWithCylinder(own, s, t)
		if err != nil {
			return meeting{}, err
		}
		var m meeting
		for i, generator := range pair.curves {
			for _, h := range lineInConicPlane(generator.(line), k, t) {
				h.t, h.s, h.touching = h.s, 0, h.touching || pair.touching[i]
				m.hits = append(m.hits, h)
			}
		}
		return m, nil
	}
	centre, radius, round := t.shadow(k, s)
	if !round {
		return conicAcrossCylinder(k, s, t), nil
	}
	circles := circleWithCircle(centre, radius, geom.Vec2{}, s.radius, t.eps)
	m := meeting{alike: circles.same}
	for _, q := range circles.points {
		// The conic point above q: its parameter is the angle of q in the
		// projected axis vectors.
		rel := s.frame.XAxis().Scale(q.X - centre.X).Add(s.frame.YAxis().Scale(q.Y - centre.Y))
		at := math.Atan2(rel.Dot(k.y)*k.b, rel.Dot(k.x)*k.a)
		p := k.at(at)
		m.hits = append(m.hits, hit{p: p, t: k.param(p), gap: math.Abs(s.offset(p)), touching: circles.touching})
	}
	return m, nil
}

// conicWithConic meets two circles or ellipses. In different planes they
// can only meet on the line of the two planes. In one plane, homothetic
// conics reduce to two circles (CC3), and may be one conic; any other
// coplanar pair is unsupported.
func conicWithConic(c curve, with carrier, t tolerances) (meeting, error) {
	k, d := c.(conic), with.(conic)
	own, err := planeOf(k)
	if err != nil {
		return meeting{}, failWith(ErrInvalid, "curve table", err)
	}
	other, err := planeOf(d)
	if err != nil {
		return meeting{}, failWith(ErrInvalid, "curve table", err)
	}
	pair, err := planeWithPlane(own, other, t)
	if err != nil {
		return meeting{}, err
	}
	var m meeting
	if !pair.alike {
		for _, h := range lineInConicPlane(pair.curves[0].(line), k, t) {
			m.hits = append(m.hits, hit{p: h.p, t: h.s, s: d.param(h.p), gap: off(d, h.p)})
		}
		return m, nil
	}
	if math.Abs(own.offset(d.center)) > t.eps {
		return meeting{}, nil
	}
	if !t.homothetic(k, d) {
		return meeting{}, fail(ErrUnsupported, "curve table", "two coplanar conics that are not homothetic")
	}
	major, _, _ := k.axes()
	centre, radius, back := asCircles(k, d)
	circles := circleWithCircle(geom.Vec2{}, major, centre, radius, t.eps)
	m.alike = circles.same
	for _, q := range circles.points {
		p := back(q)
		m.hits = append(m.hits, hit{p: p, t: k.param(p), s: d.param(p), gap: off(d, p), touching: circles.touching})
	}
	return m, nil
}

// axisDistance is the square of the distance from the point of a conic at
// parameter t to the axis of a cylinder, as a polynomial in t: across the
// axis the conic is c + u·cos t + v·sin t, whose square is of degree two.
func axisDistance(k conic, s cylinder) trig {
	c, u, v := s.across(k.center.Sub(s.frame.Origin())), s.across(k.x.Scale(k.a)), s.across(k.y.Scale(k.b))
	return trig{c.Dot(c) + (u.Dot(u)+v.Dot(v))/2, 2 * c.Dot(u), 2 * c.Dot(v), (u.Dot(u) - v.Dot(v)) / 2, u.Dot(v)}
}

// conicAcrossCylinder meets a circle or an ellipse with a cylinder where
// the conic's plane is not along the axis and the conic does not project
// to a circle (section 6.4 of specs/general-boolean-crossing). The conic
// passes through the cylinder where the square of its distance to the
// axis passes the square of the radius, and touches it where the distance
// comes within the tolerance of the radius and turns back. A distance
// within ε of the radius r is a square within about 2rε of r². There are
// at most four hits. A conic within the tolerance of the cylinder all the
// way round may lie in it.
func conicAcrossCylinder(k conic, s cylinder, t tolerances) meeting {
	f := axisDistance(k, s)
	f[0] -= s.radius * s.radius
	found, everywhere := f.hits(2 * s.radius * t.eps)
	m := meeting{alike: everywhere}
	for _, h := range found {
		p := k.at(h.t)
		m.hits = append(m.hits, hit{p: p, t: k.param(p), gap: math.Abs(s.offset(p)), touching: h.touching})
	}
	return m
}

// quarticWithSurface meets the meeting curve of two cylinders with a
// plane or a cylinder (T2 of specs/general-boolean-quartic): where the
// signed distance from its point to the surface is zero within the
// tolerance. A curve within the tolerance of the surface all the way
// round may lie in it, as it does in each of its own two cylinders.
func quarticWithSurface(c curve, with carrier, t tolerances) (meeting, error) {
	q, s := c.(*quartic), with.(surface)
	found, everywhere := zeros(func(u float64) float64 { return s.offset(q.at(u)) }, q.period(), t.eps)
	m := meeting{alike: everywhere}
	for _, h := range found {
		p := q.at(h.t)
		m.hits = append(m.hits, hit{p: p, t: h.t, gap: math.Abs(s.offset(p)), touching: h.touching})
	}
	return m, nil
}

// quarticWithCurve meets the meeting curve of two cylinders with a line,
// a circle, an ellipse or another meeting curve (T3): where the distance
// from its point to the other curve is least and at most four times the
// tolerance. The passes hold each hit to their own bounds by its gap.
// Two meeting curves within the tolerance all the way round are
// candidates for an overlap.
func quarticWithCurve(c curve, with carrier, t tolerances) (meeting, error) {
	q, other := c.(*quartic), with.(curve)
	fastest := 0.
	for k := range 16 {
		fastest = math.Max(fastest, q.velocity(q.period()*float64(k)/16).Length())
	}
	found, everywhere := minima(func(u float64) float64 { return off(other, q.at(u)) }, q.period(), 4*t.eps, t.eps, 2*fastest)
	m := meeting{alike: everywhere}
	for _, u := range found {
		p := q.at(u)
		m.hits = append(m.hits, hit{p: p, t: u, s: other.param(p), gap: off(other, p)})
	}
	return m, nil
}
