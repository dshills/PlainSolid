package boolean

import "plainsolid/geom"

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
	// samples (I5).
	alike bool
}

// surfaceRow is one family pair of the table of section 7.4. A surface
// pair is added by adding a row, not by changing a caller (SP5).
type surfaceRow struct {
	a, b family
	meet func(a, b surface, t tolerances) (pairing, error)
}

var surfaceRows = []surfaceRow{
	{planeFamily, planeFamily, planeWithPlane},
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
