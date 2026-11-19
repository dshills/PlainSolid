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
}

// curveMeets resolves a curve with a curve or a surface by the table. A
// pair with no row is unsupported.
func curveMeets(c curve, with carrier, t tolerances) (meeting, error) {
	for _, r := range curveRows {
		if r.curve == c.family() && r.with == with.family() {
			return r.meet(c, with, t)
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
