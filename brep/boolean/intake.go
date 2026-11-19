package boolean

import (
	"math"

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
// fault (S5).
func intake(index int, s trimmed.Solid, t tolerances) (*operand, error) {
	o := &operand{solid: s, verts: s.Vertices()}
	faces, edges := s.Faces(), s.Edges()
	owners := table[[]int](edges)
	for fi, f := range faces {
		carrier, err := surfaceCarrier(f.Surface, t)
		if err != nil {
			return nil, fail(ErrUnsupported, "intake", "operand %s face %d: %v", operandName(index), fi, err)
		}
		rec := face{s: carrier, reversed: f.Reversed}
		for _, l := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			loop := make([]use, len(l))
			for i, u := range l {
				loop[i] = use{u.Edge, u.Reversed}
				owners[u.Edge] = append(owners[u.Edge], fi)
			}
			rec.loops = append(rec.loops, loop)
		}
		o.faces = append(o.faces, rec)
	}
	for ei, e := range edges {
		if e.Curve == nil {
			return nil, fail(ErrUnsupported, "intake", "operand %s edge %d is collapsed", operandName(index), ei)
		}
		// trimmed.Solid gives every edge with a curve exactly two uses.
		rec, err := edgeCarrier(e, o.faces[owners[ei][0]].s, o.faces[owners[ei][1]].s, t)
		if err != nil {
			return nil, failAt(err, "operand %s edge %d", operandName(index), ei)
		}
		o.edges = append(o.edges, rec)
	}
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
// its placement, which must be rigid (S4).
func surfaceCarrier(s geom.Surface, t tolerances) (surface, error) {
	base, m := parametric.SurfaceMapping(s)
	switch b := base.(type) {
	case geom.PlaneSurface:
		frame, err := placed(b.Frame(), m, t)
		if err != nil {
			return nil, err
		}
		return plane{frame}, nil
	}
	return nil, fail(ErrUnsupported, "intake", "a surface stored as %T is not a plane; cylinder faces are not accepted before Part 2.1", base)
}

// placed maps a frame by a placement. The placement must be rigid within
// the bound of S4; the mapped frame is made orthonormal again.
func placed(f geom.Frame, m geom.Affine, t tolerances) (geom.Frame, error) {
	if m == geom.Identity() {
		return f, nil
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
	if !(worst*t.size <= t.eps) || !(m.Determinant() > 0) {
		return geom.Frame{}, fail(ErrUnsupported, "intake", "the placement is not rigid")
	}
	frame, err := geom.NewFrame(m.Apply(f.Origin()), m.ApplyVector(f.XAxis()), m.ApplyVector(f.ZAxis()))
	if err != nil {
		return geom.Frame{}, fail(ErrUnsupported, "intake", "the placement is degenerate: %v", err)
	}
	return frame, nil
}
