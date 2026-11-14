package step

import (
	"context"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ReadAnalyticDiskHoleExtrusions accepts one independently authored full round
// stock with optional eccentric circular through holes and blind pockets from
// either cap. It verifies continuous analytic pcurves, shared rim/seam topology,
// disk/floor orientation, and strictly separated native footprints. As with the
// analytic polygon adapter, geometric comparison uses relative 1e-9 tolerance;
// the separate canonical ReadDiskHoleExtrusions remains exact-graph based.
func ReadAnalyticDiskHoleExtrusions(r io.Reader, l Limits) (DiskHoleExtrusionModel, error) {
	l, e := l.defaults()
	if e != nil {
		return DiskHoleExtrusionModel{}, e
	}
	f, id, unit, e := parseAnalyticCircularExtrusion(r, l)
	if e != nil {
		return DiskHoleExtrusionModel{}, e
	}
	b, e := recoverAnalyticDiskHoles(f, id)
	if e != nil {
		return DiskHoleExtrusionModel{}, e
	}
	return DiskHoleExtrusionModel{LengthUnit: unit, Bodies: []DiskHoleExtrusionBody{b}}, nil
}
func recoverAnalyticDiskHoles(f *File, id int64) (DiskHoleExtrusionBody, error) {
	fail := func() (DiskHoleExtrusionBody, error) { return DiskHoleExtrusionBody{}, circularSourceFailure() }
	g, e := readCircularSourceGraph(f, id)
	if e != nil {
		return fail()
	}
	outer := -1
	inner := []int{}
	surfaces := map[int64]int{}
	for i, face := range g.faces {
		surfaces[face.surface] = i
		if face.kind == "CYLINDER" {
			if len(face.loops) != 1 || len(face.loops[0]) != 4 {
				return fail()
			}
			if face.sense {
				if outer >= 0 {
					return fail()
				}
				outer = i
			} else {
				inner = append(inner, i)
			}
		} else {
			for _, loop := range face.loops {
				if len(loop) != 1 || g.edges[loop[0].id].kind != "CIRCLE" {
					return fail()
				}
			}
		}
	}
	if outer < 0 || len(inner) > 16 {
		return fail()
	}
	wall := g.faces[outer]
	axis := wall.frame.ZAxis()
	var rims []circularSourceEdge
	seam := int64(0)
	seamUses := 0
	for _, u := range wall.loops[0] {
		edge := g.edges[u.id]
		if edge.kind == "CIRCLE" {
			rims = append(rims, edge)
		} else {
			if seam != 0 && seam != edge.id {
				return fail()
			}
			seam = edge.id
			seamUses++
		}
	}
	if len(rims) != 2 || rims[0].id == rims[1].id || seamUses != 2 || g.edges[seam].wrapper != "SEAM_CURVE" {
		return fail()
	}
	if rims[1].frame.Origin().Sub(rims[0].frame.Origin()).Dot(axis) < 0 {
		rims[0], rims[1] = rims[1], rims[0]
	}
	frame, e := geom.NewFrame(rims[0].frame.Origin(), wall.frame.XAxis(), axis)
	if e != nil {
		return fail()
	}
	high := frame.ToLocal(rims[1].frame.Origin())
	height := high.Z
	scale := math.Max(2*wall.radius, height)
	tol := scale * 1e-9
	if !finiteNumber(tol) || tol <= 0 || height <= 64*tol || wall.radius <= 64*tol || math.Hypot(high.X, high.Y) > tol {
		return fail()
	}
	caps := [2]int{-1, -1}
	outerLoops := map[int]int{}
	for j, edge := range rims {
		if math.Abs(edge.radius-wall.radius) > tol {
			return fail()
		}
		owners := g.owners[edge.id]
		owner := owners[0]
		if owner == outer {
			owner = owners[1]
		}
		cap := g.faces[owner]
		if cap.kind != "PLANE" || owner == caps[0] {
			return fail()
		}
		caps[j] = owner
		want := axis
		if j == 0 {
			want = axis.Scale(-1)
		}
		normal := cap.frame.ZAxis()
		if !cap.sense {
			normal = normal.Scale(-1)
		}
		if normal.Sub(want).Length() > 1e-9 {
			return fail()
		}
		found := -1
		for k, loop := range cap.loops {
			if loop[0].id != edge.id {
				continue
			}
			if found >= 0 {
				return fail()
			}
			n := edge.frame.ZAxis()
			if !loop[0].sense {
				n = n.Scale(-1)
			}
			if n.Sub(want).Length() > 1e-9 {
				return fail()
			}
			found = k
		}
		if found < 0 {
			return fail()
		}
		outerLoops[owner] = found
	}
	floors := map[int]bool{}
	for i, face := range g.faces {
		if face.kind == "CYLINDER" {
			if math.Abs(math.Abs(face.frame.ZAxis().Dot(axis))-1) > 1e-10 {
				return fail()
			}
			continue
		}
		if i != caps[0] && i != caps[1] {
			if len(face.loops) != 1 {
				return fail()
			}
			floors[i] = true
		}
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
			if edge.wrapper != "SEAM_CURVE" || edge.a == edge.b || a.Sub(b).Length() <= 64*tol || pointsOnLine(a, b, edge.origin, edge.direction) != nil || b.Sub(a).Dot(edge.direction) <= 0 {
				return fail()
			}
			qa, qb := frame.ToLocal(a), frame.ToLocal(b)
			if math.Hypot(qa.X-qb.X, qa.Y-qb.Y) > tol {
				return fail()
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
			i, ok := surfaces[owner]
			if !ok {
				return fail()
			}
			u, e := verifyCircularPCurve(f, pc, g.faces[i], edge, tol)
			if e != nil {
				return fail()
			}
			us = append(us, u)
		}
		if edge.wrapper == "SEAM_CURVE" && math.Abs(math.Abs(us[1]-us[0])-2*math.Pi) > 1e-9 {
			return fail()
		}
	}
	body, _ := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	common := CircularHoleExtrusionBody{Name: name, Frame: frame, Height: height}
	if e = collectCircularSourcePockets(g, inner, caps, outerLoops, floors, frame, height, tol, &common, true); e != nil {
		return fail()
	}
	out := DiskHoleExtrusionBody{Name: name, Frame: common.Frame, Height: common.Height, Outer: trimmed.ExtrusionCircle{Radius: wall.radius}, Holes: common.Holes, Blind: common.Blind}
	disk := out.Outer
	disk.Radius -= 32 * tol
	holes := append([]trimmed.ExtrusionCircle(nil), out.Holes...)
	for i := range holes {
		holes[i].Radius += 32 * tol
	}
	blind := append([]trimmed.ExtrusionBlindHole(nil), out.Blind...)
	for i := range blind {
		blind[i].Radius += 32 * tol
	}
	if _, e = trimmed.ExtrudeDiskWithHoles(context.Background(), out.Frame, disk, holes, blind, out.Height); e != nil {
		return fail()
	}
	return out, nil
}
