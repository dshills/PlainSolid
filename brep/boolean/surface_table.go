package boolean

import (
	"math"

	"plainsolid/geom"
)

// pairing is what the table of section 7.4 gives for two surface carriers.
type pairing struct {
	// curves are the carrier curves along which the surfaces meet, and
	// touching tells for each whether the surfaces touch there without
	// crossing.
	curves   []curve
	touching []bool
	// alike is set when the carriers may be one surface. Whether two
	// faces on them are a same-surface pair is then decided node by node
	// (SP4), and an edge between them takes its carrier from its own
	// samples (I5). When the nodes say they are not, curves decides.
	alike bool
	// open is set when the table has no closed form for the pair and the
	// faces must be proved separate (SP2, SP3).
	open bool
}

// surfaceRow is one family pair of the table of section 7.4. A surface
// pair is added by adding a row, not by changing a caller (SP5).
type surfaceRow struct {
	a, b family
	meet func(a, b surface, t tolerances) (pairing, error)
}

var surfaceRows = []surfaceRow{
	{planeFamily, planeFamily, planeWithPlane},
	{planeFamily, cylinderFamily, planeWithCylinder},
	{cylinderFamily, cylinderFamily, cylinderWithCylinder},
}

// surfacePair resolves two carriers by the table. A pair with no row is
// unsupported.
func surfacePair(a, b surface, t tolerances) (pairing, error) {
	for _, r := range surfaceRows {
		if r.a == a.family() && r.b == b.family() {
			return r.meet(a, b, t)
		}
		if r.a == b.family() && r.b == a.family() {
			return r.meet(b, a, t)
		}
	}
	return pairing{}, fail(ErrUnsupported, "surface pair", "no row for carrier families %d and %d", a.family(), b.family())
}

// planeWithPlane holds the plane rows: planes that are not parallel meet
// in one line, and parallel planes are alike.
func planeWithPlane(a, b surface, t tolerances) (pairing, error) {
	p, q := a.(plane), b.(plane)
	pa, err := geom.NewPlane(p.frame.Origin(), p.frame.ZAxis())
	if err != nil {
		return pairing{}, fail(ErrInvalid, "surface pair", "%v", err)
	}
	pb, err := geom.NewPlane(q.frame.Origin(), q.frame.ZAxis())
	if err != nil {
		return pairing{}, fail(ErrInvalid, "surface pair", "%v", err)
	}
	r, err := geom.IntersectPlanes(pa, pb, geom.IntersectionTolerance{Linear: t.eps, Angular: t.angular})
	if err != nil {
		return pairing{}, fail(ErrInvalid, "surface pair", "%v", err)
	}
	if r.Kind != geom.IntersectionLine {
		return pairing{alike: true}, nil
	}
	return pairing{curves: []curve{line{r.Line.Origin(), r.Line.Direction()}}, touching: []bool{false}}, nil
}

// sameSurface is the node-by-node test of SP4 for two faces whose
// carriers are alike. A node of one face passes when it lies within its
// radius of the other face's carrier. All pass: the faces are a
// same-surface pair. None passes: the remaining rows decide, which for
// parallel planes means no contact. Some pass and some do not: the pair
// is neither coincident nor separated at this tolerance.
func sameSurface(carriers [2]surface, nodes [2][]point) (bool, error) {
	pass, miss := 0, 0
	for i, list := range nodes {
		for _, n := range list {
			if d := carriers[1-i].offset(n.pos); d <= n.radius && d >= -n.radius {
				pass++
			} else {
				miss++
			}
		}
	}
	if pass > 0 && miss > 0 {
		return false, fail(ErrTolerance, "SP4", "%d nodes of two parallel faces are on the other carrier and %d are not", pass, miss)
	}
	return miss == 0, nil
}

// planeWithCylinder holds the plane and cylinder rows (SP1). A plane that
// is not parallel to the axis cuts one circle or ellipse. A plane
// parallel to it cuts two generators, touches along one, or misses.
func planeWithCylinder(a, b surface, t tolerances) (pairing, error) {
	p, c := a.(plane), b.(cylinder)
	tol := geom.IntersectionTolerance{Linear: t.eps, Angular: t.angular}
	if !t.perpendicular(p.frame.ZAxis(), c.frame.ZAxis()) {
		gp, err := geom.NewPlane(p.frame.Origin(), p.frame.ZAxis())
		if err != nil {
			return pairing{}, failWith(ErrInvalid, "surface pair", err)
		}
		gc, err := geom.NewCylinderSurface(c.frame, c.radius)
		if err != nil {
			return pairing{}, failWith(ErrInvalid, "surface pair", err)
		}
		section, err := geom.IntersectPlaneCylinder(gp, gc, tol)
		if err != nil {
			return pairing{}, failWith(ErrInvalid, "surface pair", err)
		}
		return pairing{curves: []curve{conicOf(section.Ellipse)}, touching: []bool{false}}, nil
	}
	// Parallel under T5. The helper wants the normal exactly
	// perpendicular to the axis, so it is asked in the coordinates of the
	// cylinder, with the part of the normal along the axis dropped.
	local := geom.Vec3{X: p.frame.ZAxis().Dot(c.frame.XAxis()), Y: p.frame.ZAxis().Dot(c.frame.YAxis())}
	n, err := local.Unit()
	if err != nil {
		return pairing{}, failWith(ErrInvalid, "surface pair", err)
	}
	gp, err := geom.NewPlane(n.Scale(p.offset(c.frame.Origin())*-1), n)
	if err != nil {
		return pairing{}, failWith(ErrInvalid, "surface pair", err)
	}
	upright, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		return pairing{}, failWith(ErrInvalid, "surface pair", err)
	}
	gc, err := geom.NewCylinderSurface(upright, c.radius)
	if err != nil {
		return pairing{}, failWith(ErrInvalid, "surface pair", err)
	}
	section, err := geom.IntersectPlaneCylinder(gp, gc, tol)
	if err != nil {
		return pairing{}, failWith(ErrInvalid, "surface pair", err)
	}
	var out pairing
	for _, l := range section.Lines {
		out.curves = append(out.curves, line{c.frame.ToWorld(l.Origin()), c.frame.ZAxis()})
		out.touching = append(out.touching, section.Tangent)
	}
	return out, nil
}

// cylinderWithCylinder holds the rows for two cylinders. Parallel
// cylinders meet in two generators, touch along one from outside or from
// inside, or miss; coaxial ones do not meet. A pair whose surfaces are
// nowhere farther apart than the tolerance scale is alike (SP4). Axes
// that are not parallel have no closed form here (SP3).
func cylinderWithCylinder(a, b surface, t tolerances) (pairing, error) {
	c, d := a.(cylinder), b.(cylinder)
	if !t.parallel(c.frame.ZAxis(), d.frame.ZAxis()) {
		return pairing{open: true}, nil
	}
	ch := cylinderChart(c)
	off := d.frame.Origin().Sub(c.frame.Origin())
	centre := geom.Vec2{X: off.Dot(c.frame.XAxis()), Y: off.Dot(c.frame.YAxis())}
	apart := math.Hypot(centre.X, centre.Y)
	out := pairing{alike: apart+math.Abs(c.radius-d.radius) < 4*t.eps}
	if apart <= t.eps {
		return out, nil // coaxial
	}
	m := circleWithCircle(geom.Vec2{}, c.radius, centre, d.radius, t.eps)
	for _, q := range m.points {
		out.curves = append(out.curves, line{ch.frame.ToWorld(geom.Vec3{X: q.X, Y: q.Y}), c.frame.ZAxis()})
		out.touching = append(out.touching, m.touching)
	}
	return out, nil
}
