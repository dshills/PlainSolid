package boolean

import (
	"math"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// use is one traversal of an edge by a face loop.
type use struct {
	edge     int
	reversed bool
}

// edge is an input edge reduced to its carrier and interval (I3, I5). The
// carrier is oriented so that the edge runs from t0 up to t1.
type edge struct {
	start, end int
	c          curve
	t0, t1     float64
	box        bounds
}

// closed reports whether the edge runs once round a closed carrier. The
// interval of such an edge is t0 to t0 plus the period (I5 step 6), and
// that sum, taken back from t1, can fall short of the period by the
// rounding of t1. The test allows that rounding and nothing more: an open
// arc would have to end within it of a whole turn to be taken for closed.
func (e edge) closed() bool {
	period := e.c.period()
	return period > 0 && period-(e.t1-e.t0) <= 8*0x1p-52*math.Abs(e.t1)
}

// face is an input face reduced to its carrier, its orientation against
// the carrier normal, and its loops. In the chart of the carrier every
// loop keeps the face on its left: the outer loop comes first and runs
// counterclockwise, the others are holes.
type face struct {
	s        surface
	reversed bool
	loops    [][]use
	box      bounds
}

// operand is one input solid after intake.
type operand struct {
	solid trimmed.Solid
	verts []geom.Vec3
	edges []edge
	faces []face
	box   bounds
}

// intake reduces an operand to carriers (I3) and rejects one outside the
// class of section 4.1 with an error that names the first face or edge at
// fault (S5). A collapsed edge is accepted as the pole of a polar plane
// chart and carries nothing into the arrangement (S3): it is left out of
// the edges and of the loops.
//
// onto is the other operand, when it has been taken in already. A planar
// face that lies within ε of the plane of one of its faces is then seated
// on that plane (S7): it takes that carrier, its edges take theirs from
// it, and its vertices are moved onto them.
func intake(index int, s trimmed.Solid, t tolerances, onto *operand) (*operand, error) {
	o := &operand{solid: s, verts: s.Vertices()}
	faces, edges := s.Faces(), s.Edges()
	seated := make([]bool, len(faces))
	owners := table[[]int](edges)
	kept := filled(edges, -1)
	next := 0
	for i, e := range edges {
		if e.Curve != nil {
			kept[i] = next
			next++
		}
	}
	for fi, f := range faces {
		carrier, polar, mirrored, err := surfaceCarrier(f.Surface, t)
		if err != nil {
			return nil, fail(ErrUnsupported, "intake", "operand %s face %d: %v", operandName(index), fi, err)
		}
		// The chart of a carrier under a placement that reflects runs the
		// other way round than the chart the face is stored in, and its
		// normal points the other way (S4). The face is the same point
		// set: its orientation is read against the other normal, and each
		// loop is taken backwards, so that it keeps the face on its left
		// in the chart of the carrier.
		if own, ok := carrier.(plane); ok && onto != nil {
			// The border of the face, at the samples by which I5 will
			// hold each of its edges to the carrier of the face. The
			// distance from a plane is greatest on the border.
			border := func() []geom.Vec3 {
				var points []geom.Vec3
				for _, l := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
					for _, u := range l {
						e := edges[u.Edge]
						if e.Curve == nil {
							points = append(points, o.verts[e.Start])
							continue
						}
						for k := 0; k < edgeSamples; k++ {
							if p, err := e.Curve.Eval(e.Range.Min + (e.Range.Max-e.Range.Min)*float64(k)/float64(edgeSamples-1)); err == nil {
								points = append(points, p)
							}
						}
					}
				}
				return points
			}
			if seat, against, ok := onto.seat(own, o.verts[edges[f.Outer[0].Edge].Start], border, t); ok {
				// A plane that faces the other way has a chart that runs
				// the other way round, as under a reflecting placement.
				carrier, mirrored, seated[fi] = seat, mirrored != against, true
			}
		}
		rec := face{s: carrier, reversed: f.Reversed != mirrored}
		for _, l := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			var loop []use
			for _, u := range l {
				if kept[u.Edge] < 0 {
					if !polar {
						return nil, fail(ErrUnsupported, "intake", "operand %s edge %d is collapsed and not the pole of a polar plane chart", operandName(index), u.Edge)
					}
					continue
				}
				loop = append(loop, use{kept[u.Edge], u.Reversed != mirrored})
				owners[u.Edge] = append(owners[u.Edge], fi)
			}
			if mirrored {
				slices.Reverse(loop)
			}
			rec.loops = append(rec.loops, loop)
		}
		o.faces = append(o.faces, rec)
	}
	for ei, e := range edges {
		if e.Curve == nil {
			continue
		}
		// trimmed.Solid gives every edge with a curve exactly two uses.
		rec, err := edgeCarrier(e, o.faces[owners[ei][0]].s, o.faces[owners[ei][1]].s, t)
		if err != nil {
			return nil, failAt(err, "operand %s edge %d", operandName(index), ei)
		}
		o.edges = append(o.edges, rec)
	}
	o.settle(edges, owners, kept, seated)
	for fi := range o.faces {
		f := &o.faces[fi]
		f.box = o.edges[f.loops[0][0].edge].box
		for _, loop := range f.loops {
			for _, u := range loop {
				f.box = f.box.with(o.edges[u.edge].box)
			}
		}
		if fi == 0 {
			o.box = f.box
		}
		o.box = o.box.with(f.box)
	}
	return o, nil
}

// surfaceCarrier recognizes a face by its surface value (S2) and applies
// its placement, which must keep lengths (S4). It tells a plane stored in
// the polar chart, whose normal points against the z axis of its frame
// and whose pole is a collapsed edge, and a placement that reflects.
func surfaceCarrier(s geom.Surface, t tolerances) (carrier surface, polar, mirrored bool, err error) {
	base, m := parametric.SurfaceMapping(s)
	if polar, ok := parametric.PolarPlaneFrame(base); ok {
		against, err := geom.NewFrame(polar.Origin(), polar.XAxis(), polar.ZAxis().Scale(-1))
		if err != nil {
			return nil, false, false, fail(ErrUnsupported, "intake", "the polar chart is degenerate: %v", err)
		}
		frame, mirrored, err := placed(against, m, t)
		return plane{frame}, true, mirrored, err
	}
	switch b := base.(type) {
	case geom.PlaneSurface:
		frame, mirrored, err := placed(b.Frame(), m, t)
		return plane{frame}, false, mirrored, err
	case geom.CylinderSurface:
		own, radius := b.Parameters()
		frame, mirrored, err := placed(own, m, t)
		return cylinder{frame, radius}, false, mirrored, err
	}
	return nil, false, false, fail(ErrUnsupported, "intake", "a surface stored as %T is neither a plane nor a circular cylinder", base)
}

// placed maps a frame by a placement. The placement must keep lengths
// within the bound of S4; the mapped frame is made orthonormal again. It
// is the frame on the images of the origin, the x axis and the z axis. A
// placement that reflects maps a right-handed frame to a left-handed one,
// so the y axis of the frame returned is then against the image of the y
// axis, and placed reports it.
func placed(f geom.Frame, m geom.Affine, t tolerances) (frame geom.Frame, mirrored bool, err error) {
	if m == geom.Identity() {
		return f, false, nil
	}
	worst := 0.
	for i := 0; i < 3; i++ {
		row := 0.
		for j := 0; j < 3; j++ {
			dot := m[0][i]*m[0][j] + m[1][i]*m[1][j] + m[2][i]*m[2][j]
			if i == j {
				dot--
			}
			row += math.Abs(dot)
		}
		worst = math.Max(worst, row)
	}
	if !(worst*t.size <= t.eps) {
		return geom.Frame{}, false, fail(ErrUnsupported, "intake", "the placement does not keep lengths")
	}
	frame, err = geom.NewFrame(m.Apply(f.Origin()), m.ApplyVector(f.XAxis()), m.ApplyVector(f.ZAxis()))
	if err != nil {
		return geom.Frame{}, false, fail(ErrUnsupported, "intake", "the placement is degenerate: %v", err)
	}
	return frame, m.Determinant() < 0, nil
}

// seated bounds how far a face may be from a plane and be seated on it: ε,
// less a part that keeps the samples of its edges within ε of the plane
// after rounding (I5).
const seatedShare = 1 - 0x1p-30

// seat finds the plane of a face of the operand that a planar face of the
// other operand lies on within ε: the planes are parallel as the table of
// section 7.4 judges it, and every point of the border of the face is
// within ε of the plane. One point of the face is tried first, since few
// planes pass it. It reports whether the plane faces against the face's
// own.
func (o *operand) seat(own plane, first geom.Vec3, border func() []geom.Vec3, t tolerances) (plane, bool, bool) {
	var points []geom.Vec3
next:
	for _, f := range o.faces {
		other, ok := f.s.(plane)
		if !ok || other == own || !(math.Abs(other.offset(first)) <= seatedShare*t.eps) {
			continue
		}
		if p, err := planeWithPlane(other, own, t); err != nil || !p.alike {
			continue
		}
		if points == nil {
			points = border()
		}
		for _, c := range points {
			if !(math.Abs(other.offset(c)) <= seatedShare*t.eps) {
				continue next
			}
		}
		return other, other.frame.ZAxis().Dot(own.frame.ZAxis()) < 0, true
	}
	return plane{}, false, false
}

// settle moves the vertices of seated faces onto the carriers their edges
// took from the seat, and has every edge that ends at a moved vertex end
// there. A vertex is moved once, onto the first such edge that ends at
// it; the others pass through the same point within rounding, being cut
// by the same plane.
func (o *operand) settle(edges []parametric.Edge, owners [][]int, kept []int, seated []bool) {
	moved := make([]bool, len(o.verts))
	some := false
	for ei, e := range edges {
		if kept[ei] < 0 || !seated[owners[ei][0]] && !seated[owners[ei][1]] {
			continue
		}
		c := o.edges[kept[ei]].c
		for _, v := range [2]int{e.Start, e.End} {
			if !moved[v] {
				o.verts[v], moved[v], some = c.at(c.param(o.verts[v])), true, true
			}
		}
	}
	if !some {
		return
	}
	for i := range o.edges {
		e := &o.edges[i]
		if !moved[e.start] && !moved[e.end] || e.closed() {
			continue
		}
		near := func(v int, old float64) float64 {
			at := e.c.param(o.verts[v])
			if period := e.c.period(); period > 0 {
				at += period * math.Round((old-at)/period)
			}
			return at
		}
		e.t0, e.t1 = near(e.start, e.t0), near(e.end, e.t1)
	}
}
