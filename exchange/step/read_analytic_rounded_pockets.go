package step

import (
	"context"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ReadAnalyticRoundedPocketExtrusions adds disjoint circular bores and
// common-entry blind pockets to the independently authored rounded-stock
// subset. The outer stock and every original pocket owner/pcurve are verified
// before native footprint containment is checked. Relative 1e-9 analytic
// tolerance applies; the stock-only and exact canonical readers are unchanged.
func ReadAnalyticRoundedPocketExtrusions(r io.Reader, l Limits) (RoundedRectangleExtrusionModel, error) {
	f, id, unit, e := parseAnalyticCircularExtrusion(r, l)
	if e != nil {
		return RoundedRectangleExtrusionModel{}, e
	}
	b, e := recoverAnalyticRoundedPockets(f, id)
	if e != nil {
		return RoundedRectangleExtrusionModel{}, e
	}
	return RoundedRectangleExtrusionModel{LengthUnit: unit, Bodies: []RoundedRectangleExtrusionBody{b}}, nil
}
func recoverAnalyticRoundedPockets(f *File, id int64) (RoundedRectangleExtrusionBody, error) {
	fail := func() (RoundedRectangleExtrusionBody, error) {
		return RoundedRectangleExtrusionBody{}, circularSourceFailure()
	}
	g, e := readCircularSourceGraph(f, id)
	if e != nil {
		return fail()
	}
	inner := []int{}
	floors := map[int]bool{}
	outerLoops := map[int]int{}
	surfaces := map[int64]int{}
	stockFaces := []Value{}
	stock := *f
	stock.Entities = make(map[int64]Entity, len(f.Entities))
	for id, en := range f.Entities {
		stock.Entities[id] = en
	}
	for i, face := range g.faces {
		surfaces[face.surface] = i
		if face.kind == "CYLINDER" {
			if !face.sense {
				if len(face.loops) != 1 || len(face.loops[0]) != 4 {
					return fail()
				}
				inner = append(inner, i)
				continue
			}
			if len(face.loops) != 1 {
				return fail()
			}
		} else {
			outer := -1
			for j, loop := range face.loops {
				if len(loop) >= 3 {
					if outer >= 0 {
						return fail()
					}
					outer = j
				} else if len(loop) != 1 || g.edges[loop[0].id].kind != "CIRCLE" {
					return fail()
				}
			}
			if outer < 0 {
				if len(face.loops) != 1 {
					return fail()
				}
				floors[i] = true
				continue
			}
			outerLoops[i] = outer
			en := f.Entities[face.id]
			r := en.Records[0]
			r.Args = append([]Value(nil), r.Args...)
			r.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: face.bounds[outer]}}}
			en.Records = []Record{r}
			stock.Entities[face.id] = en
		}
		stockFaces = append(stockFaces, Value{Kind: Reference, Ref: face.id})
	}
	if len(inner) > 16 {
		return fail()
	}
	body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	sid, _ := ref(body.Args[1])
	en := f.Entities[sid]
	sr := en.Records[0]
	sr.Args = append([]Value(nil), sr.Args...)
	sr.Args[1] = Value{Kind: Aggregate, Items: stockFaces}
	en.Records = []Record{sr}
	stock.Entities[sid] = en
	out, e := recoverAnalyticRoundedRectangle(&stock, id)
	if e != nil {
		return fail()
	}
	sg, e := readCircularSourceGraph(&stock, id)
	if e != nil {
		return fail()
	}
	frame, height := out.Frame, out.Height
	axis := frame.ZAxis()
	scale := math.Max(height, math.Hypot(out.Profile.Max.X-out.Profile.Min.X, out.Profile.Max.Y-out.Profile.Min.Y))
	tol := scale * 1e-9
	caps := [2]int{-1, -1}
	for i, j := range outerLoops {
		face := g.faces[i]
		n := face.frame.ZAxis()
		if !face.sense {
			n = n.Scale(-1)
		}
		d := n.Dot(axis)
		if math.Abs(math.Abs(d)-1) < 1e-10 {
			k := 0
			if d > 0 {
				k = 1
			}
			if caps[k] >= 0 {
				return fail()
			}
			caps[k] = i
		} else if len(face.loops) != 1 || j != 0 {
			return fail()
		}
	}
	if caps[0] < 0 || caps[1] < 0 {
		return fail()
	}
	for _, i := range inner {
		if math.Abs(math.Abs(g.faces[i].frame.ZAxis().Dot(axis))-1) > 1e-10 {
			return fail()
		}
	}
	for eid, edge := range g.edges {
		if _, ok := sg.edges[eid]; ok {
			continue
		}
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
	common := CircularHoleExtrusionBody{Frame: frame, Height: height}
	if e = collectCircularSourcePockets(g, inner, caps, outerLoops, floors, frame, height, tol, &common); e != nil {
		return fail()
	}
	out.Frame, out.Height = common.Frame, common.Height
	out.Holes, out.Blind = common.Holes, common.Blind
	holes := append([]trimmed.ExtrusionCircle(nil), out.Holes...)
	for i := range holes {
		holes[i].Radius += 32 * tol
	}
	blind := append([]trimmed.ExtrusionBlindHole(nil), out.Blind...)
	for i := range blind {
		blind[i].Radius += 32 * tol
	}
	if _, e = trimmed.ExtrudeRoundedRectangle(context.Background(), out.Frame, out.Profile, holes, blind, out.Height); e != nil {
		return fail()
	}
	return out, nil
}
