package step

import (
	"context"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
)

// ReadAnalyticCircularThroughExtrusions reads one convex planar prism with up to
// sixteen full cylindrical through bores. Unlike the canonical source reader,
// this analytic adapter accepts alternate plane frames and face ordering. It
// verifies continuous line/circle pcurves at a relative 1e-9 geometric tolerance,
// source incidence and separated native footprints. Blind floors, split rims,
// assembly placement and detached geometry are unsupported. The recovered frame
// and positive height describe geometry, not original signed feature history.
func ReadAnalyticCircularThroughExtrusions(r io.Reader, l Limits) (CircularHoleExtrusionModel, error) {
	return readAnalyticCircularExtrusions(r, l, false)
}
func readAnalyticCircularExtrusions(r io.Reader, l Limits, allowBlind bool) (CircularHoleExtrusionModel, error) {
	l, e := l.defaults()
	if e != nil {
		return CircularHoleExtrusionModel{}, e
	}
	f, id, unit, e := parseAnalyticCircularExtrusion(r, l)
	if e != nil {
		return CircularHoleExtrusionModel{}, e
	}
	b, e := recoverAnalyticCircularExtrusion(f, id, l, allowBlind)
	if e != nil {
		return CircularHoleExtrusionModel{}, e
	}
	return CircularHoleExtrusionModel{LengthUnit: unit, Bodies: []CircularHoleExtrusionBody{b}}, nil
}
func parseAnalyticCircularExtrusion(r io.Reader, l Limits) (*File, int64, float64, error) {
	fail := func() (*File, int64, float64, error) {
		return nil, 0, 0, circularSourceFailure()
	}
	l, e := l.defaults()
	if e != nil {
		return nil, 0, 0, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return nil, 0, 0, e
	}
	repID := int64(0)
	counts := map[string]int{}
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, r := range en.Records {
			counts[r.Type]++
			if strings.Contains(r.Type, "TRANSFORMATION") || strings.Contains(r.Type, "REPRESENTATION_RELATIONSHIP") || r.Type == "MAPPED_ITEM" || r.Type == "REPRESENTATION_MAP" || r.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE" {
				return fail()
			}
			if strings.HasSuffix(r.Type, "SHAPE_REPRESENTATION") {
				if r.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" || repID != 0 {
					return fail()
				}
				repID = id
			}
		}
	}
	if counts["MANIFOLD_SOLID_BREP"] != 1 || counts["CLOSED_SHELL"] != 1 || counts["ADVANCED_FACE"] > l.MaxFaces || counts["VERTEX_POINT"] > l.MaxVertices || counts["ORIENTED_EDGE"] > l.MaxEdgeUses {
		return fail()
	}
	rep, e := simple(f, repID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	unit, e := contextUnits(f, ctx)
	if e != nil {
		return fail()
	}
	items, e := listRefs(rep.Args[1], 1)
	if e != nil || len(items) > 2 {
		return fail()
	}
	bodyID := int64(0)
	for _, id := range items {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			return fail()
		}
		switch en.Records[0].Type {
		case "MANIFOLD_SOLID_BREP":
			if bodyID != 0 {
				return fail()
			}
			bodyID = id
		case "AXIS2_PLACEMENT_3D":
			p, e := placement(f, id)
			if e != nil || p.Origin() != (geom.Vec3{}) || p.XAxis() != (geom.Vec3{X: 1}) || p.ZAxis() != (geom.Vec3{Z: 1}) {
				return fail()
			}
		default:
			return fail()
		}
	}
	// Every geometric entity must be reachable from the selected representation.
	seen := map[int64]bool{}
	todo := []int64{repID}
	var refs func(Value)
	refs = func(v Value) {
		if v.Kind == Reference {
			todo = append(todo, v.Ref)
		}
		for _, x := range v.Items {
			refs(x)
		}
	}
	for len(todo) > 0 {
		id := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		for _, r := range f.Entities[id].Records {
			for _, v := range r.Args {
				refs(v)
			}
		}
	}
	metadata := map[string]bool{"PRODUCT_RELATED_PRODUCT_CATEGORY": true, "APPLICATION_CONTEXT": true, "APPLICATION_PROTOCOL_DEFINITION": true, "MECHANICAL_CONTEXT": true, "DESIGN_CONTEXT": true, "PRODUCT_CONTEXT": true, "PRODUCT_DEFINITION_CONTEXT": true, "PRODUCT": true, "PRODUCT_DEFINITION_FORMATION": true, "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE": true, "PRODUCT_DEFINITION": true, "PRODUCT_DEFINITION_SHAPE": true, "SHAPE_DEFINITION_REPRESENTATION": true}
	for id, en := range f.Entities {
		if seen[id] {
			continue
		}
		for _, r := range en.Records {
			if !metadata[r.Type] {
				return fail()
			}
		}
	}
	return f, bodyID, unit, nil
}
func recoverAnalyticCircularExtrusion(f *File, id int64, l Limits, allowBlind bool) (CircularHoleExtrusionBody, error) {
	fail := func() (CircularHoleExtrusionBody, error) { return CircularHoleExtrusionBody{}, circularSourceFailure() }
	g, e := readCircularSourceGraph(f, id)
	if e != nil {
		return fail()
	}
	body, _ := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	axis := geom.Vec3{}
	x := geom.Vec3{}
	cylinders := []int{}
	surfaceFace := map[int64]int{}
	for i, face := range g.faces {
		surfaceFace[face.surface] = i
		if face.kind == "CYLINDER" {
			if face.sense || len(face.loops) != 1 || len(face.loops[0]) != 4 {
				return fail()
			}
			if len(cylinders) == 0 {
				axis, x = face.frame.ZAxis(), face.frame.XAxis()
			}
			if math.Abs(math.Abs(axis.Dot(face.frame.ZAxis()))-1) > 1e-10 {
				return fail()
			}
			cylinders = append(cylinders, i)
		}
	}
	if len(cylinders) == 0 || len(cylinders) > 16 {
		return fail()
	}
	caps := [2]int{-1, -1}
	outerLoops := map[int]int{}
	floors := map[int]bool{}
	stockFaces := []Value{}
	for i, face := range g.faces {
		if face.kind != "PLANE" {
			continue
		}
		polygon := -1
		for j, loop := range face.loops {
			if len(loop) >= 3 {
				if polygon >= 0 {
					return fail()
				}
				for _, u := range loop {
					if g.edges[u.id].kind != "LINE" {
						return fail()
					}
				}
				polygon = j
			} else if len(loop) != 1 || g.edges[loop[0].id].kind != "CIRCLE" {
				return fail()
			}
		}
		if polygon < 0 {
			if !allowBlind || len(face.loops) != 1 || len(face.loops[0]) != 1 {
				return fail()
			}
			floors[i] = true
			continue
		}
		outerLoops[i] = polygon
		normal := face.frame.ZAxis()
		if !face.sense {
			normal = normal.Scale(-1)
		}
		d := normal.Dot(axis)
		if math.Abs(math.Abs(d)-1) < 1e-10 {
			j := 0
			if d > 0 {
				j = 1
			}
			if caps[j] >= 0 {
				return fail()
			}
			caps[j] = i
		} else if math.Abs(d) > 1e-10 || len(face.loops) != 1 || len(face.loops[0]) != 4 {
			return fail()
		}
		stockFaces = append(stockFaces, Value{Kind: Reference, Ref: face.id})
	}
	if caps[0] < 0 || caps[1] < 0 {
		return fail()
	}
	vertex := func(u analyticUse) (geom.Vec3, error) {
		edge := g.edges[u.id]
		v := edge.a
		if !u.sense {
			v = edge.b
		}
		return analyticVertex(f, v)
	}
	baseLoop := g.faces[caps[0]].loops[outerLoops[caps[0]]]
	topLoop := g.faces[caps[1]].loops[outerLoops[caps[1]]]
	if len(baseLoop) != len(topLoop) || len(topLoop) < 3 || len(topLoop) > 64 || len(stockFaces) != len(topLoop)+2 {
		return fail()
	}
	origin, e := vertex(baseLoop[0])
	if e != nil {
		return fail()
	}
	frame, e := geom.NewFrame(origin, x, axis)
	if e != nil {
		return fail()
	}
	tp, e := vertex(topLoop[0])
	if e != nil {
		return fail()
	}
	height := frame.ToLocal(tp).Z
	if height <= 0 {
		return fail()
	}
	scale := height
	for _, edge := range g.edges {
		for _, vid := range []int64{edge.a, edge.b} {
			p, e := analyticVertex(f, vid)
			if e != nil {
				return fail()
			}
			scale = math.Max(scale, p.Sub(origin).Length())
		}
	}
	tol := scale * 1e-9
	if math.IsInf(tol, 0) || tol <= 0 || height <= 64*tol {
		return fail()
	}
	out := CircularHoleExtrusionBody{Name: name, Frame: frame, Height: height}
	for _, u := range topLoop {
		p, e := vertex(u)
		if e != nil {
			return fail()
		}
		q := frame.ToLocal(p)
		if math.Abs(q.Z-height) > tol {
			return fail()
		}
		out.Outer = append(out.Outer, geom.Vec2{X: q.X, Y: q.Y})
	}
	// Strip only the proven bore faces and cap inner bounds, then independently
	// validate the remaining complete planar stock graph.
	stock := *f
	stock.Entities = make(map[int64]Entity, len(f.Entities))
	for id, en := range f.Entities {
		stock.Entities[id] = en
	}
	for i, j := range outerLoops {
		face := g.faces[i]
		en := f.Entities[face.id]
		r := en.Records[0]
		r.Args = append([]Value(nil), r.Args...)
		r.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: face.bounds[j]}}}
		en.Records = []Record{r}
		stock.Entities[face.id] = en
	}
	sid, _ := ref(body.Args[1])
	en := f.Entities[sid]
	sr := en.Records[0]
	sr.Args = append([]Value(nil), sr.Args...)
	sr.Args[1] = Value{Kind: Aggregate, Items: stockFaces}
	en.Records = []Record{sr}
	stock.Entities[sid] = en
	if _, e = readBody(&stock, id, l.MaxVertices, l.MaxFaces, l.MaxEdgeUses, true); e != nil {
		return CircularHoleExtrusionBody{}, e
	}
	for _, edge := range g.edges {
		a, e := analyticVertex(f, edge.a)
		if e != nil {
			return fail()
		}
		b, e := analyticVertex(f, edge.b)
		if e != nil {
			return fail()
		}
		if edge.kind == "LINE" {
			if edge.a == edge.b || a.Sub(b).Length() <= 64*tol || pointsOnLine(a, b, edge.origin, edge.direction) != nil || b.Sub(a).Dot(edge.direction) <= 0 {
				return fail()
			}
			qa, qb := frame.ToLocal(a), frame.ToLocal(b)
			sameZ := math.Abs(qa.Z-qb.Z) <= tol
			sameXY := math.Hypot(qa.X-qb.X, qa.Y-qb.Y) <= tol
			if !sameZ && !sameXY {
				return fail()
			}
			for _, q := range []geom.Vec3{qa, qb} {
				if edge.wrapper != "SEAM_CURVE" && math.Min(math.Abs(q.Z), math.Abs(q.Z-height)) > tol {
					return fail()
				}
			}
		} else if edge.a != edge.b || a.Sub(edge.frame.ToWorld(geom.Vec3{X: edge.radius})).Length() > tol {
			return fail()
		}
		us := []float64{}
		for _, pc := range edge.pcs {
			owner, _, e := circularPCurveGeometry(f, pc)
			if e != nil {
				return fail()
			}
			i, ok := surfaceFace[owner]
			if !ok {
				return fail()
			}
			u, e := verifyCircularPCurve(f, pc, g.faces[i], edge, tol)
			if e != nil {
				return fail()
			}
			us = append(us, u)
		}
		if edge.wrapper == "SEAM_CURVE" && (edge.kind != "LINE" || math.Abs(math.Abs(us[1]-us[0])-2*math.Pi) > 1e-9) {
			return fail()
		}
	}
	if e = collectCircularSourcePockets(g, cylinders, caps, outerLoops, floors, frame, height, tol, &out, false); e != nil {
		return fail()
	}
	inflated := append([]trimmed.ExtrusionCircle(nil), out.Holes...)
	for i := range inflated {
		inflated[i].Radius += 32 * tol
	}
	blind := append([]trimmed.ExtrusionBlindHole(nil), out.Blind...)
	for i := range blind {
		blind[i].Radius += 32 * tol
	}
	if _, e = trimmed.ExtrudePolygonWithHoles(context.Background(), out.Frame, out.Outer, inflated, blind, out.Height); e != nil {
		return fail()
	}

	return out, nil
}
