package trimmed

import (
	"fmt"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BoreHistory gives the original opening faces and the generated edge and
// wall IDs in positive angular order around the bore axis.
type BoreHistory struct {
	Entry, Exit              int
	EntryEdges, ExitEdges    [4]int
	WallFaces, WallSeamEdges [4]int
}

// CylinderCut specifies one through-tool. Entry and Exit refer to the
// original planar faces. Direction points from Entry to Exit.
type CylinderCut struct {
	Entry, Exit          int
	AxisPoint, Direction geom.Vec3
	Radius               float64
}

// BoreCylinders subtracts multiple through-tools from the same convex solid.
// All finite tool portions must have pairwise separated conservative bounds.
// Overlapping bounds are reported as unresolved even if the tools are disjoint.
// Source indices stay stable and each report identifies its generated entities.
func BoreCylinders(s brep.Solid, cuts []CylinderCut, tolerance float64) (Solid, []BoreHistory, error) {
	if len(cuts) == 0 || len(cuts) > 32 || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, nil, geom.ErrInvalid
	}
	r, e := FromPlanar(s, tolerance)
	if e != nil {
		return Solid{}, nil, e
	}
	v, edges, faces := r.Vertices(), r.Edges(), r.Faces()
	nv, ne, nf := len(v), len(edges), len(faces)
	translate := func(a []Use, remap func(int) int) []Use {
		b := append([]Use(nil), a...)
		for i := range b {
			b[i].Edge = remap(b[i].Edge)
		}
		return b
	}
	var bounds []geom.Box
	history := make([]BoreHistory, 0, len(cuts))
	for i, c := range cuts {
		t, h, e := BoreCylinderAlong(s, c.Entry, c.Exit, c.AxisPoint, c.Direction, c.Radius, tolerance)
		if e != nil {
			return Solid{}, nil, fmt.Errorf("trimmed: tool %d: %w", i, e)
		}
		var box geom.Box
		for j, id := range h.WallFaces {
			f := t.faces[id]
			d, e := faceUVBounds(f)
			if e != nil {
				return Solid{}, nil, e
			}
			b, e := parametric.SurfaceBounds(f.Surface, parametric.Interval{Min: d.UMin, Max: d.UMax}, parametric.Interval{Min: d.VMin, Max: d.VMax})
			if e != nil {
				return Solid{}, nil, e
			}
			if j == 0 {
				box = b
			} else {
				box, e = box.Union(b)
				if e != nil {
					return Solid{}, nil, e
				}
			}
		}
		pad := func(lo, hi float64) (float64, float64) {
			m := tolerance + 128*0x1p-52*max(math.Abs(lo), math.Abs(hi))
			return math.Nextafter(lo-m, math.Inf(-1)), math.Nextafter(hi+m, math.Inf(1))
		}
		box.Min.X, box.Max.X = pad(box.Min.X, box.Max.X)
		box.Min.Y, box.Max.Y = pad(box.Min.Y, box.Max.Y)
		box.Min.Z, box.Max.Z = pad(box.Min.Z, box.Max.Z)
		if !box.Valid() {
			return Solid{}, nil, geom.ErrInvalid
		}
		for j, b := range bounds {
			if _, ok := b.Intersect(box); ok {
				return Solid{}, nil, fmt.Errorf("trimmed: tools %d and %d do not have separated bounds", j, i)
			}
		}
		bounds = append(bounds, box)
		edgeOffset, vertexOffset, faceOffset := len(edges)-ne, len(v)-nv, len(faces)-nf
		remap := func(id int) int {
			if id >= ne {
				return id + edgeOffset
			}
			return id
		}
		v = append(v, t.vertices[nv:]...)
		for _, x := range t.edges[ne:] {
			x.Start += vertexOffset
			x.End += vertexOffset
			edges = append(edges, x)
		}
		for _, f := range []int{h.Entry, h.Exit} {
			added := t.faces[f].Holes[len(t.faces[f].Holes)-1]
			faces[f].Holes = append(faces[f].Holes, translate(added, remap))
		}
		for j := range 4 {
			f := cloneFace(t.faces[h.WallFaces[j]])
			f.Outer = translate(f.Outer, remap)
			faces = append(faces, f)
			h.WallFaces[j] += faceOffset
			h.EntryEdges[j] = remap(h.EntryEdges[j])
			h.ExitEdges[j] = remap(h.ExitEdges[j])
			h.WallSeamEdges[j] = remap(h.WallSeamEdges[j])
		}
		history = append(history, h)
	}
	out, e := NewSolid(v, edges, faces, tolerance)
	if e != nil {
		return Solid{}, nil, e
	}
	return out, history, nil
}

// BoreCylinder subtracts a circular through-hole normal to two opposite
// faces of a convex planar solid. axisPoint lies anywhere along the bore
// axis. The two complete openings must clear all other supporting planes
// by tolerance. Tangent, partial and multi-shell cuts are rejected.
// The returned solid preserves the original face, edge and vertex IDs.
func BoreCylinder(s brep.Solid, entry, exit int, axisPoint geom.Vec3, radius, tolerance float64) (Solid, BoreHistory, error) {
	a, e := s.FacePlane(entry)
	if e != nil {
		return Solid{}, BoreHistory{}, e
	}
	b, e := s.FacePlane(exit)
	if e != nil {
		return Solid{}, BoreHistory{}, e
	}
	if a.Normal().Dot(b.Normal()) >= 0 || a.Normal().Cross(b.Normal()).Length() > 64*0x1p-52 {
		return Solid{}, BoreHistory{}, fmt.Errorf("trimmed: opening faces are not opposite parallel planes")
	}
	return BoreCylinderAlong(s, entry, exit, axisPoint, a.Normal().Scale(-1), radius, tolerance)
}

// BoreCylinderAlong subtracts a cylindrical tool along the specified direction
// from entry to exit. The two opening planes may have different orientations,
// but each complete ellipse must lie strictly inside its planar face and
// every tool generator must enter through entry and leave through exit.
// The cap holes and side trims use analytic parameter curves.
func BoreCylinderAlong(s brep.Solid, entry, exit int, axisPoint, direction geom.Vec3, radius, tolerance float64) (Solid, BoreHistory, error) {
	fail := func(reason string) (Solid, BoreHistory, error) {
		return Solid{}, BoreHistory{}, fmt.Errorf("trimmed: cylinder bore: %s", reason)
	}
	if !axisPoint.IsFinite() || !finite(radius) || !finite(tolerance) || radius <= tolerance || tolerance <= 0 {
		return fail("invalid axis, radius or tolerance")
	}
	topology := s.Topology()
	if topology.Shells != 1 || topology.EulerCharacteristic != 2 || entry == exit ||
		entry < 0 || exit < 0 || entry >= topology.Faces || exit >= topology.Faces {
		return fail("expected one convex shell and two distinct cap faces")
	}
	if topology.Vertices > (1<<20)/topology.Faces {
		return fail("convexity work limit exceeded")
	}
	v := s.Vertices()
	m, ancestry := s.Tessellate(), s.TriangleFaces()
	first := make([][3]int, topology.Faces)
	seen := make([]bool, topology.Faces)
	for j, f := range ancestry {
		if !seen[f] {
			first[f], seen[f] = m.Triangles[j], true
		}
	}
	for f, t := range first {
		if !seen[f] {
			return fail("missing face triangle")
		}
		for _, p := range v {
			side, e := geom.Orient3D(v[t[0]], v[t[1]], v[t[2]], p)
			if e != nil || side < 0 {
				return fail("input is not an outward convex shell")
			}
		}
	}
	r, e := FromPlanar(s, tolerance)
	if e != nil {
		return Solid{}, BoreHistory{}, e
	}
	v, edges, faces := r.Vertices(), r.Edges(), r.Faces()
	plane := func(i int) geom.Frame { return faces[i].Surface.(geom.PlaneSurface).Frame() }
	a, b := plane(entry), plane(exit)
	n, e := direction.Unit()
	if e != nil {
		return fail("invalid axis direction")
	}
	da, db := n.Dot(a.ZAxis()), n.Dot(b.ZAxis())
	if da >= -64*0x1p-52 || db <= 64*0x1p-52 {
		return fail("axis must point from entry to exit through both faces")
	}
	base := axisPoint.Add(n.Scale(a.Origin().Sub(axisPoint).Dot(a.ZAxis()) / da))
	h := b.Origin().Sub(base).Dot(b.ZAxis()) / db
	if !base.IsFinite() || !finite(h) || h <= 2*tolerance {
		return fail("invalid axial span")
	}
	frame, e := geom.FrameFromNormal(base, n)
	if e != nil {
		return Solid{}, BoreHistory{}, e
	}
	type opening struct {
		center, x, y geom.Vec3
		v, xV, yV    float64
	}
	makeOpening := func(normal geom.Vec3, denominator, height float64) opening {
		x, y := frame.XAxis().Scale(radius), frame.YAxis().Scale(radius)
		dx, dy := -x.Dot(normal)/denominator, -y.Dot(normal)/denominator
		return opening{frame.ToWorld(geom.Vec3{Z: height}), x.Add(n.Scale(dx)), y.Add(n.Scale(dy)), height, dx, dy}
	}
	openings := [2]opening{makeOpening(a.ZAxis(), da, 0), makeOpening(b.ZAxis(), db, h)}
	if h-math.Hypot(openings[1].xV-openings[0].xV, openings[1].yV-openings[0].yV) <= 2*tolerance {
		return fail("opening ellipses are not separated along every generator")
	}
	for i := range faces {
		if i == entry || i == exit {
			continue
		}
		p := plane(i)
		normal := p.ZAxis()
		for _, o := range openings {
			reach := math.Hypot(normal.Dot(o.x), normal.Dot(o.y))
			d := o.center.Sub(p.Origin())
			margin := -d.Dot(normal) - reach
			roundoff := 64 * 0x1p-52 * max(d.Length(), o.x.Length(), o.y.Length())
			if !finite(margin) || margin <= tolerance+roundoff {
				return fail("bore touches or crosses a non-opening face")
			}
		}
	}
	wall, e := geom.NewCylinderSurface(frame, radius)
	if e != nil {
		return Solid{}, BoreHistory{}, e
	}
	history := BoreHistory{Entry: entry, Exit: exit}
	var pointIDs [2][4]int
	var sideUV [2][5]geom.Vec2
	sideArc := func(o opening, from, to float64) EllipseUV {
		return EllipseUV{Center: geom.Vec2{Y: o.v}, Cos: geom.Vec2{Y: o.xV}, Sin: geom.Vec2{Y: o.yV}, Linear: geom.Vec2{X: 1}, From: from, To: to}
	}
	for k, f := range [2]int{entry, exit} {
		ids := &history.EntryEdges
		o := openings[k]
		if k == 1 {
			ids = &history.ExitEdges
		}
		for i := range 5 {
			angle := float64(i) * math.Pi / 2
			uv, e := sideArc(o, angle, angle+math.Pi/2).EvalUV(0)
			if e != nil {
				return Solid{}, BoreHistory{}, e
			}
			sideUV[k][i] = uv
			if i == 4 {
				break
			}
			p, e := wall.Eval(uv.X, uv.Y)
			if e != nil {
				return Solid{}, BoreHistory{}, e
			}
			pointIDs[k][i] = len(v)
			v = append(v, p)
		}
		p := plane(f)
		q := p.ToLocal(o.center)
		c, x, y := geom.Vec2{X: q.X, Y: q.Y},
			geom.Vec2{X: o.x.Dot(p.XAxis()), Y: o.x.Dot(p.YAxis())},
			geom.Vec2{X: o.y.Dot(p.XAxis()), Y: o.y.Dot(p.YAxis())}
		loop := make([]Use, 4)
		for i := range 4 {
			a, b := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
			modelArc := EllipseUV{Center: c, Cos: x, Sin: y, From: a, To: b}
			ids[i] = len(edges)
			edges = append(edges, parametric.Edge{Start: pointIDs[k][i], End: pointIDs[k][(i+1)%4],
				Curve: surfaceTrimCurve{surface: faces[f].Surface, curve: modelArc}, Range: parametric.Interval{Max: 1}})
			j := i
			if k == 1 {
				j, a, b = 3-i, b, a
			}
			arc := EllipseUV{Center: c, Cos: x, Sin: y, From: a, To: b}
			start, e := arc.EvalUV(0)
			if e != nil {
				return Solid{}, BoreHistory{}, e
			}
			end, e := arc.EvalUV(1)
			if e != nil {
				return Solid{}, BoreHistory{}, e
			}
			loop[j] = Use{Edge: ids[i], Reversed: k == 1, UVStart: start, UVEnd: end, Curve: arc}
		}
		faces[f].Holes = append(faces[f].Holes, loop)
	}
	for i := range 4 {
		l, e := geom.NewLine(v[pointIDs[0][i]], n)
		if e != nil {
			return Solid{}, BoreHistory{}, e
		}
		history.WallSeamEdges[i] = len(edges)
		edges = append(edges, parametric.Edge{Start: pointIDs[0][i], End: pointIDs[1][i], Curve: l,
			Range: parametric.Interval{Max: sideUV[1][i].Y - sideUV[0][i].Y}})
	}
	for i := range 4 {
		a, b := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
		p := [4]geom.Vec2{sideUV[0][i], sideUV[0][i+1], sideUV[1][i+1], sideUV[1][i]}
		ids := [4]int{history.EntryEdges[i], history.WallSeamEdges[(i+1)%4], history.ExitEdges[i], history.WallSeamEdges[i]}
		l := make([]Use, 4)
		for j, id := range ids {
			l[j] = Use{Edge: id, Reversed: j >= 2, UVStart: p[j], UVEnd: p[(j+1)%4]}
		}
		if o := openings[0]; o.xV != 0 || o.yV != 0 {
			l[0].Curve = sideArc(o, a, b)
		}
		if o := openings[1]; o.xV != 0 || o.yV != 0 {
			l[2].Curve = sideArc(o, b, a)
		}
		history.WallFaces[i] = len(faces)
		faces = append(faces, Face{Surface: wall, Outer: l, Reversed: true})
	}
	out, e := NewSolid(v, edges, faces, tolerance)
	if e != nil {
		return Solid{}, BoreHistory{}, e
	}
	return out, history, nil
}
