package trimmed

import (
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// stopCrossedBoreCap acts only on a freshly constructed, fully qualified
// crossed-bore cap beyond its intersection. It retains every curve
// exactly; only circular rim supports and straight axial segments are moved.
func stopCrossedBoreCap(s Solid, cap int, shift geom.Vec3) (Solid, int, error) {
	fail := func(e error) (Solid, int, error) { return Solid{}, 0, e }
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if cap < 0 || cap >= 6 || len(faces[cap].Holes) != 1 || (len(faces[cap].Holes[0]) != 4 && len(faces[cap].Holes[0]) != 6) {
		return fail(geom.ErrDomain)
	}
	plane, ok := faces[cap].Surface.(geom.PlaneSurface)
	if !ok {
		return fail(geom.ErrDomain)
	}
	pf := plane.Frame()
	ff, e := geom.NewFrame(pf.Origin().Add(shift), pf.XAxis(), pf.ZAxis())
	if e != nil {
		return fail(e)
	}
	fs, e := geom.NewPlaneSurface(ff)
	if e != nil {
		return fail(e)
	}
	ring := faces[cap].Holes[0]
	rim, moved := map[int]bool{}, map[int]bool{}
	for _, u := range ring {
		if _, ok := u.Curve.(unionArcUV); !ok {
			return fail(geom.ErrDomain)
		}
		rim[u.Edge] = true
		moved[edges[u.Edge].Start] = true
		moved[edges[u.Edge].End] = true
	}
	for id := range moved {
		vertices[id] = vertices[id].Add(shift)
	}
	for id, edge := range edges {
		if !moved[edge.Start] && !moved[edge.End] {
			continue
		}
		c, ok := edge.Curve.(surfaceTrimCurve)
		if !ok || edge.Range != (parametric.Interval{Max: 1}) {
			return fail(geom.ErrDomain)
		}
		if rim[id] {
			c.surface = fs
		} else {
			cylinder, ok := c.surface.(geom.CylinderSurface)
			if !ok || c.curve != nil {
				return fail(geom.ErrDomain)
			}
			f, _ := cylinder.Parameters()
			delta := shift.Dot(f.ZAxis())
			if moved[edge.Start] {
				c.a.Y += delta
			}
			if moved[edge.End] {
				c.b.Y += delta
			}
		}
		edges[id].Curve = c
	}
	for i := range faces {
		if i == cap {
			continue
		}
		for _, loop := range append([][]Use{faces[i].Outer}, faces[i].Holes...) {
			for j := range loop {
				u := &loop[j]
				edge := edges[u.Edge]
				start, end := edge.Start, edge.End
				if u.Reversed {
					start, end = end, start
				}
				if !moved[start] && !moved[end] {
					continue
				}
				cylinder, ok := faces[i].Surface.(geom.CylinderSurface)
				if !ok || u.Curve != nil {
					return fail(geom.ErrDomain)
				}
				f, _ := cylinder.Parameters()
				delta := shift.Dot(f.ZAxis())
				if moved[start] {
					u.UVStart.Y += delta
				}
				if moved[end] {
					u.UVEnd.Y += delta
				}
			}
		}
	}
	faces[cap].Holes = nil
	floorID := len(faces)
	faces = append(faces, Face{Surface: fs, Reversed: true, Outer: reverseUnionLoop(ring)})
	out, e := NewSolid(vertices, edges, faces, s.Tolerance())
	return out, floorID, e
}
