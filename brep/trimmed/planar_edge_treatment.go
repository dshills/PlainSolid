package trimmed

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// TreatPlanarOrthogonalEdge rounds or bevels one complete convex right-angle
// edge of planar stock. Three-valent endpoints and strict separation of the
// local treatment box from every nonincident boundary prove local clearance.
// The selected and endpoint faces may be nonrectangular.
func TreatPlanarOrthogonalEdge(ctx context.Context, base brep.Solid, start, end geom.Vec3, size float64, bevel bool, tolerance float64) (Solid, error) {
	fail := func() (Solid, error) { return Solid{}, geom.ErrDomain }
	if ctx == nil || !finite(size) || !finite(tolerance) || tolerance <= 0 || size <= 8*tolerance {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	source, e := CoalesceCardinalPlanar(ctx, base, tolerance)
	if e != nil {
		return Solid{}, e
	}
	source, e = collapseTreatmentEdges(ctx, source, source.Faces())
	if e != nil {
		return Solid{}, e
	}
	vertices, edges, faces := source.Vertices(), source.Edges(), source.Faces()
	edgeID := -1
	for i, ed := range edges {
		a, b := vertices[ed.Start], vertices[ed.End]
		if a == start && b == end || a == end && b == start {
			edgeID = i
			break
		}
	}
	if edgeID < 0 {
		return fail()
	}
	owners := make([][]int, len(edges))
	for i, f := range faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				owners[u.Edge] = append(owners[u.Edge], i)
			}
		}
	}
	if len(owners[edgeID]) != 2 {
		return fail()
	}
	f1, f2 := owners[edgeID][0], owners[edgeID][1]
	n1 := faces[f1].Surface.(geom.PlaneSurface).Frame().ZAxis()
	n2 := faces[f2].Surface.(geom.PlaneSurface).Frame().ZAxis()
	axis := n1.Cross(n2)
	cardinal := func(v geom.Vec3) bool {
		return math.Abs(v.X)+math.Abs(v.Y)+math.Abs(v.Z) == 1 && (v.X == 0 || math.Abs(v.X) == 1) && (v.Y == 0 || math.Abs(v.Y) == 1) && (v.Z == 0 || math.Abs(v.Z) == 1)
	}
	if !cardinal(n1) || !cardinal(n2) || !cardinal(axis) {
		return fail()
	}
	a, b := edges[edgeID].Start, edges[edgeID].End
	if vertices[b].Sub(vertices[a]).Dot(axis) < 0 {
		a, b = b, a
	}
	length := vertices[b].Sub(vertices[a]).Dot(axis)
	if length <= 8*tolerance || vertices[a].Add(axis.Scale(length)) != vertices[b] {
		return fail()
	}
	ends := [2]int{a, b}
	var endpointFaces [2]int
	var incident [2][2]int
	for k, id := range ends {
		inc := []int{}
		for i, ed := range edges {
			if i != edgeID && len(owners[i]) > 0 && (ed.Start == id || ed.End == id) {
				inc = append(inc, i)
			}
		}
		if len(inc) != 2 {
			return fail()
		}
		endpointFaces[k] = -1
		for _, eid := range inc {
			own := owners[eid]
			if len(own) != 2 {
				return fail()
			}
			which := -1
			other := -1
			for _, f := range own {
				if f == f1 {
					which = 0
				} else if f == f2 {
					which = 1
				} else {
					other = f
				}
			}
			if which < 0 || other < 0 {
				return fail()
			}
			if endpointFaces[k] >= 0 && endpointFaces[k] != other {
				return fail()
			}
			endpointFaces[k] = other
			incident[k][which] = eid
			ed := edges[eid]
			v := ed.Start
			if v == id {
				v = ed.End
			}
			inward := n2.Scale(-1)
			if which == 1 {
				inward = n1.Scale(-1)
			}
			delta := vertices[v].Sub(vertices[id])
			distance := delta.Dot(inward)
			if distance <= size+8*tolerance || delta != inward.Scale(distance) {
				return fail()
			}
		}
		en := faces[endpointFaces[k]].Surface.(geom.PlaneSurface).Frame().ZAxis()
		want := axis
		if k == 0 {
			want = axis.Scale(-1)
		}
		if en != want {
			return fail()
		}
	}
	// All four incident faces bound the treatment box. Other boundaries must
	// have disjoint conservative boxes; this also excludes hidden cavity faces.
	var boxPoints []geom.Vec3
	for _, id := range ends {
		for _, u := range []float64{0, size} {
			for _, v := range []float64{0, size} {
				boxPoints = append(boxPoints, vertices[id].Sub(n1.Scale(u)).Sub(n2.Scale(v)))
			}
		}
	}
	box, e := geom.Bounds(boxPoints)
	if e != nil {
		return Solid{}, e
	}
	separated := func(lo, hi geom.Vec3) bool {
		return hi.X < box.Min.X || lo.X > box.Max.X || hi.Y < box.Min.Y || lo.Y > box.Max.Y || hi.Z < box.Min.Z || lo.Z > box.Max.Z
	}
	exempt := map[int]bool{f1: true, f2: true, endpointFaces[0]: true, endpointFaces[1]: true}
	for i, f := range faces {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		var points []geom.Vec3
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				ed := edges[u.Edge]
				if exempt[i] && (u.Edge == edgeID || u.Edge == incident[0][0] || u.Edge == incident[0][1] || u.Edge == incident[1][0] || u.Edge == incident[1][1]) {
					continue
				}
				bounds, _ := geom.Bounds([]geom.Vec3{vertices[ed.Start], vertices[ed.End]})
				if exempt[i] && !separated(bounds.Min, bounds.Max) {
					return fail()
				}
				points = append(points, vertices[ed.Start], vertices[ed.End])
			}
		}
		if !exempt[i] {
			bounds, e := geom.Bounds(points)
			if e != nil || !separated(bounds.Min, bounds.Max) {
				return fail()
			}
		}
	}
	var tangents [2][2]int
	for k, id := range ends {
		for side, n := range []geom.Vec3{n2, n1} {
			p := vertices[id].Sub(n.Scale(size))
			if !p.IsFinite() || p.Add(n.Scale(size)) != vertices[id] {
				return fail()
			}
			tangents[k][side] = len(vertices)
			vertices = append(vertices, p)
			eid := incident[k][side]
			if edges[eid].Start == id {
				edges[eid].Start = tangents[k][side]
			} else {
				edges[eid].End = tangents[k][side]
			}
		}
	}
	line := func(start, end int) (parametric.Edge, error) {
		p, q := vertices[start], vertices[end]
		c, e := geom.NewLine(p, q.Sub(p))
		return parametric.Edge{Start: start, End: end, Curve: c, Range: parametric.Interval{Max: q.Sub(p).Length()}}, e
	}
	for _, ids := range incident {
		for _, id := range ids {
			ed, e := line(edges[id].Start, edges[id].End)
			if e != nil {
				return Solid{}, e
			}
			edges[id] = ed
		}
	}
	var rails [2]int
	for i := 0; i < 2; i++ {
		ed, e := line(tangents[0][i], tangents[1][i])
		if e != nil {
			return Solid{}, e
		}
		rails[i] = len(edges)
		edges = append(edges, ed)
	}
	center := vertices[a].Sub(n1.Scale(size)).Sub(n2.Scale(size))
	frame, e := geom.NewFrame(center, n1, axis)
	if e != nil {
		return Solid{}, e
	}
	cylinder, e := geom.NewCylinderSurface(frame, size)
	if e != nil {
		return Solid{}, e
	}
	var arcs [2]int
	for k := 0; k < 2; k++ {
		arcs[k] = len(edges)
		if bevel {
			ed, e := line(tangents[k][0], tangents[k][1])
			if e != nil {
				return Solid{}, e
			}
			edges = append(edges, ed)
		} else {
			z := float64(k) * length
			curve := surfaceTrimCurve{surface: cylinder, a: geom.Vec2{Y: z}, b: geom.Vec2{X: math.Pi / 2, Y: z}}
			edges = append(edges, parametric.Edge{Start: tangents[k][0], End: tangents[k][1], Curve: curve, Range: parametric.Interval{Max: 1}})
		}
	}
	for fi := range faces {
		plane := faces[fi].Surface.(geom.PlaneSurface)
		loops := append([][]Use{faces[fi].Outer}, faces[fi].Holes...)
		for li, loop := range loops {
			var out []Use
			for _, old := range loop {
				u := old
				if u.Edge == edgeID {
					side := 0
					if fi == f2 {
						side = 1
					}
					u.Edge = rails[side]
					u.Reversed = edges[edgeID].Start != a != old.Reversed
				}
				ed := edges[u.Edge]
				s, t := ed.Start, ed.End
				if u.Reversed {
					s, t = t, s
				}
				p, q := plane.Frame().ToLocal(vertices[s]), plane.Frame().ToLocal(vertices[t])
				u.UVStart, u.UVEnd = geom.Vec2{X: p.X, Y: p.Y}, geom.Vec2{X: q.X, Y: q.Y}
				u.Curve = nil
				out = append(out, u)
			}
			var sewn []Use
			for i, u := range out {
				sewn = append(sewn, u)
				ed := edges[u.Edge]
				end := ed.End
				if u.Reversed {
					end = ed.Start
				}
				next := out[(i+1)%len(out)]
				nd := edges[next.Edge]
				start := nd.Start
				if next.Reversed {
					start = nd.End
				}
				if end == start {
					continue
				}
				k := -1
				for j, id := range endpointFaces {
					if fi == id {
						k = j
					}
				}
				if k < 0 {
					return fail()
				}
				ae := edges[arcs[k]]
				reverse := end == ae.End
				if !(end == ae.Start && start == ae.End || end == ae.End && start == ae.Start) {
					return fail()
				}
				p, q := plane.Frame().ToLocal(vertices[end]), plane.Frame().ToLocal(vertices[start])
				use := Use{Edge: arcs[k], Reversed: reverse, UVStart: geom.Vec2{X: p.X, Y: p.Y}, UVEnd: geom.Vec2{X: q.X, Y: q.Y}}
				if !bevel {
					c := plane.Frame().ToLocal(center.Add(axis.Scale(float64(k) * length)))
					x, y := n1.Scale(size), n2.Scale(size)
					from, to := 0., math.Pi/2
					if reverse {
						from, to = to, from
					}
					use.Curve = unionArcUV{EllipseUV: EllipseUV{Center: geom.Vec2{X: c.X, Y: c.Y}, Cos: geom.Vec2{X: x.Dot(plane.Frame().XAxis()), Y: x.Dot(plane.Frame().YAxis())}, Sin: geom.Vec2{X: y.Dot(plane.Frame().XAxis()), Y: y.Dot(plane.Frame().YAxis())}, From: from, To: to}, Start: use.UVStart, End: use.UVEnd}
				}
				sewn = append(sewn, use)
			}
			if li == 0 {
				faces[fi].Outer = sewn
			} else {
				faces[fi].Holes[li-1] = sewn
			}
		}
	}
	if bevel {
		normal, _ := n1.Add(n2).Unit()
		f, e := geom.NewFrame(vertices[tangents[0][0]], axis, normal)
		if e != nil {
			return Solid{}, e
		}
		plane, e := geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, e
		}
		ids := []int{arcs[0], rails[1], arcs[1], rails[0]}
		rev := []bool{false, false, true, true}
		var loop []Use
		for i, id := range ids {
			ed := edges[id]
			a, b := ed.Start, ed.End
			if rev[i] {
				a, b = b, a
			}
			p, q := f.ToLocal(vertices[a]), f.ToLocal(vertices[b])
			loop = append(loop, Use{Edge: id, Reversed: rev[i], UVStart: geom.Vec2{X: p.X, Y: p.Y}, UVEnd: geom.Vec2{X: q.X, Y: q.Y}})
		}
		faces = append(faces, Face{Surface: plane, Outer: loop})
	} else {
		faces = append(faces, Face{Surface: cylinder, Outer: []Use{{Edge: arcs[0], UVStart: geom.Vec2{}, UVEnd: geom.Vec2{X: math.Pi / 2}}, {Edge: rails[1], UVStart: geom.Vec2{X: math.Pi / 2}, UVEnd: geom.Vec2{X: math.Pi / 2, Y: length}}, {Edge: arcs[1], Reversed: true, UVStart: geom.Vec2{X: math.Pi / 2, Y: length}, UVEnd: geom.Vec2{Y: length}}, {Edge: rails[0], Reversed: true, UVStart: geom.Vec2{Y: length}, UVEnd: geom.Vec2{}}}})
	}
	return compactSteppedTopology(ctx, vertices, edges, faces, tolerance)
}
