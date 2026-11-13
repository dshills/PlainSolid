package step

import (
	"context"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ReadAnalyticRoundedRectangleExtrusions reads one stock-only extrusion with
// four straight sides and one to four circular quarter corners. Independent
// analytic source frames and face ordering are accepted at relative 1e-9
// geometric tolerance. Holes, split rims, non-quarter blends and assemblies are
// unsupported; the canonical reader retains its separate exact graph contract.
func ReadAnalyticRoundedRectangleExtrusions(r io.Reader, l Limits) (RoundedRectangleExtrusionModel, error) {
	f, id, unit, e := parseAnalyticCircularExtrusion(r, l)
	if e != nil {
		return RoundedRectangleExtrusionModel{}, e
	}
	b, e := recoverAnalyticRoundedRectangle(f, id)
	if e != nil {
		return RoundedRectangleExtrusionModel{}, e
	}
	return RoundedRectangleExtrusionModel{LengthUnit: unit, Bodies: []RoundedRectangleExtrusionBody{b}}, nil
}
func recoverAnalyticRoundedRectangle(f *File, id int64) (RoundedRectangleExtrusionBody, error) {
	fail := func() (RoundedRectangleExtrusionBody, error) {
		return RoundedRectangleExtrusionBody{}, circularSourceFailure()
	}
	g, e := readCircularSourceGraph(f, id)
	if e != nil {
		return fail()
	}
	axis := geom.Vec3{}
	curved := []int{}
	surfaces := map[int64]int{}
	for i, face := range g.faces {
		surfaces[face.surface] = i
		if len(face.loops) != 1 {
			return fail()
		}
		if face.kind == "CYLINDER" {
			if !face.sense || len(face.loops[0]) != 4 {
				return fail()
			}
			if len(curved) == 0 {
				axis = face.frame.ZAxis()
			}
			if math.Abs(math.Abs(axis.Dot(face.frame.ZAxis()))-1) > 1e-10 {
				return fail()
			}
			curved = append(curved, i)
		}
	}
	if len(curved) < 1 || len(curved) > 4 {
		return fail()
	}
	caps := [2]int{-1, -1}
	for i, face := range g.faces {
		if face.kind != "PLANE" {
			continue
		}
		n := face.frame.ZAxis()
		if !face.sense {
			n = n.Scale(-1)
		}
		d := n.Dot(axis)
		if math.Abs(math.Abs(d)-1) < 1e-10 {
			j := 0
			if d > 0 {
				j = 1
			}
			if caps[j] >= 0 {
				return fail()
			}
			caps[j] = i
		} else if math.Abs(d) > 1e-10 || len(face.loops[0]) != 4 {
			return fail()
		}
	}
	if caps[0] < 0 || caps[1] < 0 {
		return fail()
	}
	lower, upper := g.faces[caps[0]].loops[0], g.faces[caps[1]].loops[0]
	if len(upper) != 4+len(curved) || len(lower) != len(upper) || len(g.faces) != len(upper)+2 {
		return fail()
	}
	point := func(u analyticUse) (geom.Vec3, error) {
		edge := g.edges[u.id]
		v := edge.a
		if !u.sense {
			v = edge.b
		}
		return analyticVertex(f, v)
	}
	top, e := point(upper[0])
	if e != nil {
		return fail()
	}
	base, e := point(lower[0])
	if e != nil {
		return fail()
	}
	height := top.Sub(base).Dot(axis)
	if height <= 0 {
		return fail()
	}
	x := geom.Vec3{}
	for _, u := range upper {
		edge := g.edges[u.id]
		if edge.kind == "LINE" {
			x = edge.direction
			if !u.sense {
				x = x.Scale(-1)
			}
			break
		}
	}
	frame, e := geom.NewFrame(top.Sub(axis.Scale(height)), x, axis)
	if e != nil {
		return fail()
	}
	scale := height
	vertices := make([]geom.Vec2, len(upper))
	profile := trimmed.RoundedRectangle{}
	for i, u := range upper {
		p, e := point(u)
		if e != nil {
			return fail()
		}
		q := frame.ToLocal(p)
		vertices[i] = geom.Vec2{X: q.X, Y: q.Y}
		scale = math.Max(scale, q.Length())
		if i == 0 {
			profile.Min, profile.Max = vertices[i], vertices[i]
		} else {
			profile.Min.X = math.Min(profile.Min.X, q.X)
			profile.Min.Y = math.Min(profile.Min.Y, q.Y)
			profile.Max.X = math.Max(profile.Max.X, q.X)
			profile.Max.Y = math.Max(profile.Max.Y, q.Y)
		}
	}
	tol := scale * 1e-9
	if !finiteNumber(tol) || tol <= 0 || height <= 64*tol {
		return fail()
	}
	if profile.Max.X-profile.Min.X <= 64*tol || profile.Max.Y-profile.Min.Y <= 64*tol {
		return fail()
	}
	// Assign each exact circle image to its unique cardinal corner. This tests
	// both tangencies, not a sampled curve or a fitted rounded rectangle.
	for i, u := range upper {
		edge := g.edges[u.id]
		if edge.kind != "CIRCLE" {
			continue
		}
		a, b := vertices[i], vertices[(i+1)%len(vertices)]
		center := frame.ToLocal(edge.frame.Origin())
		radius := edge.radius
		matched := -1
		centers := [4]geom.Vec2{{X: profile.Min.X + radius, Y: profile.Min.Y + radius}, {X: profile.Max.X - radius, Y: profile.Min.Y + radius}, {X: profile.Max.X - radius, Y: profile.Max.Y - radius}, {X: profile.Min.X + radius, Y: profile.Max.Y - radius}}
		starts := [4]geom.Vec2{{X: profile.Min.X, Y: profile.Min.Y + radius}, {X: profile.Max.X - radius, Y: profile.Min.Y}, {X: profile.Max.X, Y: profile.Max.Y - radius}, {X: profile.Min.X + radius, Y: profile.Max.Y}}
		ends := [4]geom.Vec2{{X: profile.Min.X + radius, Y: profile.Min.Y}, {X: profile.Max.X, Y: profile.Min.Y + radius}, {X: profile.Max.X - radius, Y: profile.Max.Y}, {X: profile.Min.X, Y: profile.Max.Y - radius}}
		near := func(a, b geom.Vec2) bool { return math.Hypot(a.X-b.X, a.Y-b.Y) <= tol }
		for k := 0; k < 4; k++ {
			if near(a, starts[k]) && near(b, ends[k]) && near(geom.Vec2{X: center.X, Y: center.Y}, centers[k]) {
				if matched >= 0 {
					return fail()
				}
				matched = k
			}
		}
		normal := edge.frame.ZAxis()
		if !u.sense {
			normal = normal.Scale(-1)
		}
		if matched < 0 || profile.Radii[matched] != 0 || radius <= 64*tol || normal.Sub(axis).Length() > 1e-9 {
			return fail()
		}
		profile.Radii[matched] = radius
	}
	native, e := trimmed.ExtrudeRoundedRectangle(context.Background(), frame, profile, nil, nil, height)
	if e != nil {
		return fail()
	}
	outline := native.Faces()[0].Outer
	if len(outline) != len(upper) {
		return fail()
	}
	matched := false
	for shift := range outline {
		ok := true
		for i, u := range upper {
			want := outline[(i+shift)%len(outline)]
			edge := g.edges[u.id]
			if math.Hypot(vertices[i].X-want.UVStart.X, vertices[i].Y-want.UVStart.Y) > tol || (edge.kind == "CIRCLE") != (want.Curve != nil) {
				ok = false
				break
			}
		}
		if ok {
			matched = true
			break
		}
	}
	if !matched {
		return fail()
	}
	// Every lateral face owns exactly one edge on each cap, and their source
	// endpoints are joined by two distinct vertical lines.
	lateral := map[int][2]int64{}
	for j, loop := range [][]analyticUse{lower, upper} {
		for _, u := range loop {
			if g.edges[u.id].kind == "CIRCLE" {
				n := g.edges[u.id].frame.ZAxis()
				if !u.sense {
					n = n.Scale(-1)
				}
				want := axis
				if j == 0 {
					want = axis.Scale(-1)
				}
				if n.Sub(want).Length() > 1e-9 {
					return fail()
				}
			}
			edge := g.edges[u.id]
			owners := g.owners[u.id]
			other := owners[0]
			if other == caps[j] {
				other = owners[1]
			}
			if other == caps[0] || other == caps[1] {
				return fail()
			}
			pair := lateral[other]
			if pair[j] != 0 {
				return fail()
			}
			pair[j] = u.id
			lateral[other] = pair
			if (edge.kind == "CIRCLE") != (g.faces[other].kind == "CYLINDER") {
				return fail()
			}
		}
	}
	if len(lateral) != len(upper) {
		return fail()
	}
	for i, pair := range lateral {
		if pair[0] == 0 || pair[1] == 0 {
			return fail()
		}
		seen := map[int64]bool{}
		vertical := []int64{}
		for _, u := range g.faces[i].loops[0] {
			if seen[u.id] {
				return fail()
			}
			seen[u.id] = true
			if u.id != pair[0] && u.id != pair[1] {
				if g.edges[u.id].kind != "LINE" {
					return fail()
				}
				vertical = append(vertical, u.id)
			}
		}
		if !seen[pair[0]] || !seen[pair[1]] || len(vertical) != 2 {
			return fail()
		}
		if g.faces[i].kind == "CYLINDER" {
			uv := []float64{}
			for _, eid := range vertical {
				edge := g.edges[eid]
				found := false
				for _, pc := range edge.pcs {
					owner, gid, e := circularPCurveGeometry(f, pc)
					if e != nil {
						return fail()
					}
					if owner == g.faces[i].surface {
						p, _, e := lineGeometry(f, gid, 2)
						if e != nil {
							return fail()
						}
						uv = append(uv, p.X)
						found = true
					}
				}
				if !found {
					return fail()
				}
			}
			if len(uv) != 2 || math.Abs(math.Abs(uv[1]-uv[0])-math.Pi/2) > 1e-9 {
				return fail()
			}
		}
	}

	for _, face := range g.faces {
		if face.kind != "PLANE" {
			continue
		}
		loop := face.loops[0]
		p0, e := point(loop[0])
		if e != nil {
			return fail()
		}
		sum := geom.Vec3{}
		for k := 1; k+1 < len(loop); k++ {
			a, e := point(loop[k])
			if e != nil {
				return fail()
			}
			b, e := point(loop[k+1])
			if e != nil {
				return fail()
			}
			sum = sum.Add(a.Sub(p0).Cross(b.Sub(p0)))
		}
		n, e := sum.Unit()
		if e != nil {
			return fail()
		}
		want := face.frame.ZAxis()
		if !face.sense {
			want = want.Scale(-1)
		}
		if n.Sub(want).Length() > 1e-9 {
			return fail()
		}
	}
	for _, edge := range g.edges {
		if edge.wrapper != "SURFACE_CURVE" || edge.a == edge.b {
			return fail()
		}
		a, e := analyticVertex(f, edge.a)
		if e != nil {
			return fail()
		}
		b, e := analyticVertex(f, edge.b)
		if e != nil {
			return fail()
		}
		qa, qb := frame.ToLocal(a), frame.ToLocal(b)
		for _, q := range []geom.Vec3{qa, qb} {
			if math.Min(math.Abs(q.Z), math.Abs(q.Z-height)) > tol {
				return fail()
			}
		}
		if edge.kind == "LINE" {
			if a.Sub(b).Length() <= 64*tol || pointsOnLine(a, b, edge.origin, edge.direction) != nil || b.Sub(a).Dot(edge.direction) <= 0 {
				return fail()
			}
			if math.Abs(qa.Z-qb.Z) > tol && math.Hypot(qa.X-qb.X, qa.Y-qb.Y) > tol {
				return fail()
			}
		} else {
			if math.Abs(qa.Z-qb.Z) > tol {
				return fail()
			}
			for _, p := range []geom.Vec3{a, b} {
				q := edge.frame.ToLocal(p)
				if math.Abs(q.Z) > tol || math.Abs(math.Hypot(q.X, q.Y)-edge.radius) > tol {
					return fail()
				}
			}
		}
		for _, pc := range edge.pcs {
			owner, _, e := circularPCurveGeometry(f, pc)
			if e != nil {
				return fail()
			}
			i, ok := surfaces[owner]
			if !ok {
				return fail()
			}
			if _, e = verifyCircularPCurve(f, pc, g.faces[i], edge, tol); e != nil {
				return fail()
			}
		}
	}
	body, _ := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	return RoundedRectangleExtrusionBody{Name: name, Frame: frame, Profile: profile, Height: height}, nil
}
