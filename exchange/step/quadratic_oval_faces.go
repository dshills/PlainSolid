package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/exchange/iges"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// QuadraticOval reuses the exact four-quarter UV profile contract of IGES.
type QuadraticOval = iges.QuadraticOval
type QuadraticOvalFace struct {
	Name         string
	Geometry     *geom.BSplineSurface
	Outer        QuadraticOval
	Holes        []QuadraticOval
	SourceFaceID int64
	SourceEdges  []int64
}
type QuadraticOvalFaceModel struct {
	Faces      []QuadraticOvalFace
	LengthUnit float64
}

func WriteQuadraticOvalFaces(w io.Writer, m QuadraticOvalFaceModel) error {
	if w == nil || len(m.Faces) == 0 || len(m.Faces) > 64 {
		return geom.ErrInvalid
	}
	type staged struct {
		model iges.Model
		file  *File
		sid   int64
	}
	parts := make([]staged, len(m.Faces))
	for i, face := range m.Faces {
		if !utf8.ValidString(face.Name) {
			return geom.ErrInvalid
		}
		q, e := iges.NewQuadraticOvalSurfaceWithHoles(face.Geometry, face.Outer, face.Holes, .001)
		if e != nil {
			return e
		}
		parts[i].model = q
		var raw bytes.Buffer
		if e = WriteSurfaces(&raw, SurfaceModel{LengthUnit: m.LengthUnit, Surfaces: []SplineSurface{{Geometry: face.Geometry, ClosedU: "F", ClosedV: "F", SelfIntersect: "F"}}}); e != nil {
			return e
		}
		f, e := Parse(&raw, Limits{})
		if e != nil {
			return e
		}
		parts[i].file = f
		for _, id := range f.Order {
			if _, ok := component(f.Entities[id], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
				parts[i].sid = id
			}
		}
	}
	var out bytes.Buffer
	e := writeStandalone(&out, m.LengthUnit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		roots := []int64{}
		curve := func(c *geom.BSpline, two bool) int64 {
			points := []int64{}
			for _, p := range c.Poles() {
				s := fmt.Sprintf("%s,%s", real(p.X), real(p.Y))
				if !two {
					s += "," + real(p.Z)
				}
				points = append(points, emit("CARTESIAN_POINT('',("+s+"))"))
			}
			weights := []string{}
			for _, v := range c.Weights() {
				weights = append(weights, real(v))
			}
			return emit(fmt.Sprintf("(BOUNDED_CURVE() B_SPLINE_CURVE(2,%s,.UNSPECIFIED.,.T.,.F.) B_SPLINE_CURVE_WITH_KNOTS((3,2,2,2,3),(0.,1.,2.,3.,4.),.UNSPECIFIED.) CURVE() GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_CURVE((%s)) REPRESENTATION_ITEM(''))", references(points), strings.Join(weights, ",")))
		}
		for i, part := range parts {
			sid := copySplineGeometry(emit, part.file, []int64{part.sid})[0]
			bounds := []int64{}
			for j, c := range part.model.CurvesOnSurfaces {
				uv, model := part.model.Entities[c.ParameterCurve].Spline.Curve, part.model.Entities[c.ModelCurve].Spline.Curve
				uid, mid := curve(uv, true), curve(model, false)
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", uid, ctx))
				pc := emit(fmt.Sprintf("PCURVE('',#%d,#%d)", sid, rep))
				sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d),.PCURVE_S1.)", mid, pc))
				p := model.Poles()[0]
				pid := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
				v := emit(fmt.Sprintf("VERTEX_POINT('',#%d)", pid))
				edge := emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", v, v, sc))
				use := emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,.T.)", edge))
				loop := emit(fmt.Sprintf("EDGE_LOOP('',(#%d))", use))
				kind := "FACE_OUTER_BOUND"
				if j > 0 {
					kind = "FACE_BOUND"
				}
				bounds = append(bounds, emit(fmt.Sprintf("%s('',#%d,.T.)", kind, loop)))
			}
			face := emit(fmt.Sprintf("ADVANCED_FACE(%s,%s,#%d,.T.)", quote(m.Faces[i].Name), references(bounds), sid))
			shell := emit(fmt.Sprintf("OPEN_SHELL('',(#%d))", face))
			roots = append(roots, emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell)))
		}
		return roots
	})
	if e != nil {
		return e
	}
	if _, e = ReadQuadraticOvalFaces(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}
func ReadQuadraticOvalFaces(r io.Reader, l Limits) (QuadraticOvalFaceModel, error) {
	return readQuadraticOvalFaces(r, l, false)
}

// ReadPartitionedQuadraticOvalFaces additionally accepts four source quadratic
// quarter edges per loop, retaining their IDs while exactly joining coefficients.
func ReadPartitionedQuadraticOvalFaces(r io.Reader, l Limits) (QuadraticOvalFaceModel, error) {
	return readQuadraticOvalFaces(r, l, true)
}
func readQuadraticOvalFaces(r io.Reader, l Limits, quarters bool) (QuadraticOvalFaceModel, error) {
	var out QuadraticOvalFaceModel
	var e error
	l, e = l.defaults()
	if e != nil {
		return out, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return out, e
	}
	owned := map[int64]bool{}
	uses := 0
	points := 0
	for _, ent := range f.Entities {
		for _, rec := range ent.Records {
			if rec.Type == "CARTESIAN_POINT" {
				points++
			}
		}
	}
	if points > l.MaxVertices {
		return out, geom.ErrInvalid
	}
	claim := func(id int64) error {
		if id <= 0 || owned[id] {
			return geom.ErrInvalid
		}
		owned[id] = true
		return nil
	}
	b, e := readSplineFaceGraph(f, l, func(f *File, id int64, maxPoints int) (BoundedSplineFace, error) {
		fail := func() (BoundedSplineFace, error) {
			return BoundedSplineFace{}, fmt.Errorf("step: unsupported quadratic oval face graph")
		}
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sense, e := boolean(face.Args[3])
		if e != nil || !sense {
			return fail()
		}
		name, e := textValue(face.Args[0])
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		if e = exactSplineWeightNormalization(f, sid, "RATIONAL_B_SPLINE_SURFACE"); e != nil {
			return BoundedSplineFace{}, e
		}
		surface, e := DecodeBSplineSurface(f, sid, maxPoints)
		if e != nil {
			return fail()
		}
		model := iges.Model{LengthUnit: .001, Entities: []iges.Entity{{Surface: &iges.Surface{Geometry: surface.Geometry, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}}}}
		trim := iges.TrimmedSurface{Surface: 0, Outer: -1}
		result := QuadraticOvalFace{Name: name, Geometry: surface.Geometry, SourceFaceID: id}
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil || len(bounds) > 17 {
			return fail()
		}
		for _, bid := range bounds {
			if e = claim(bid); e != nil {
				return fail()
			}
			bound, e := simple(f, bid, "FACE_OUTER_BOUND", 3)
			outer := e == nil
			if !outer {
				bound, e = simple(f, bid, "FACE_BOUND", 3)
			}
			if e != nil {
				return fail()
			}
			sense, e = boolean(bound.Args[2])
			if e != nil || !sense {
				return fail()
			}
			lid, e := ref(bound.Args[1])
			if e != nil || claim(lid) != nil {
				return fail()
			}
			loop, e := simple(f, lid, "EDGE_LOOP", 2)
			if e != nil {
				return fail()
			}
			edges, e := listRefs(loop.Args[1], 1)
			if e != nil || (len(edges) != 1 && (!quarters || len(edges) != 4)) {
				return fail()
			}
			var uvParts, modelParts []*geom.BSpline
			var firstVertex, previousEnd int64
			for edgeIndex, edgeUse := range edges {
				uses++
				if uses > l.MaxEdgeUses || claim(edgeUse) != nil {
					return fail()
				}
				oe, e := simple(f, edgeUse, "ORIENTED_EDGE", 5)
				if e != nil || oe.Args[1].Kind != Derived || oe.Args[2].Kind != Derived {
					return fail()
				}
				sense, e = boolean(oe.Args[4])
				if e != nil || !sense {
					return fail()
				}
				eid, e := ref(oe.Args[3])
				if e != nil || claim(eid) != nil {
					return fail()
				}
				ec, e := simple(f, eid, "EDGE_CURVE", 5)
				if e != nil {
					return fail()
				}
				sense, e = boolean(ec.Args[4])
				if e != nil || !sense {
					return fail()
				}
				v, e := ref(ec.Args[1])
				end, ee := ref(ec.Args[2])
				if e != nil || ee != nil || (len(edges) == 1 && v != end) || claim(v) != nil {
					return fail()
				}
				if edgeIndex == 0 {
					firstVertex = v
				} else if v != previousEnd {
					return fail()
				}
				previousEnd = end
				scid, e := ref(ec.Args[3])
				if e != nil || claim(scid) != nil {
					return fail()
				}
				sc, e := simple(f, scid, "SURFACE_CURVE", 4)
				if e != nil || sc.Args[3].Kind != Enumeration || sc.Args[3].Text != "PCURVE_S1" {
					return fail()
				}
				mid, e := ref(sc.Args[1])
				if e != nil || claim(mid) != nil {
					return fail()
				}
				pcs, e := listRefs(sc.Args[2], 1)
				if e != nil || len(pcs) != 1 || claim(pcs[0]) != nil {
					return fail()
				}
				pc, e := simple(f, pcs[0], "PCURVE", 3)
				if e != nil {
					return fail()
				}
				ps, _ := ref(pc.Args[1])
				rid, _ := ref(pc.Args[2])
				if ps != sid || claim(rid) != nil {
					return fail()
				}
				rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
				if e != nil {
					return fail()
				}
				ctx, _ := ref(rep.Args[2])
				if e = validatePCurveContext(f, ctx); e != nil {
					return fail()
				}
				curves, e := listRefs(rep.Args[1], 1)
				if e != nil || len(curves) != 1 || claim(curves[0]) != nil {
					return fail()
				}
				uid := curves[0]
				uv, e := decodeOvalUV(f, uid, maxPoints)
				if e != nil {
					return BoundedSplineFace{}, e
				}
				if e = exactSplineWeightNormalization(f, mid, "RATIONAL_B_SPLINE_CURVE"); e != nil {
					return BoundedSplineFace{}, e
				}
				mc, e := DecodeBSplineCurve(f, mid, maxPoints)
				if e != nil {
					return fail()
				}
				vp, e := simple(f, v, "VERTEX_POINT", 2)
				if e != nil {
					return fail()
				}
				pid, _ := ref(vp.Args[1])
				pt, e := coordinate(f, pid, "CARTESIAN_POINT", 3)
				if e != nil || pt != mc.Geometry.Poles()[0] {
					return fail()
				}
				ep, e := simple(f, end, "VERTEX_POINT", 2)
				if e != nil {
					return fail()
				}
				epid, _ := ref(ep.Args[1])
				endPoint, e := coordinate(f, epid, "CARTESIAN_POINT", 3)
				mp := mc.Geometry.Poles()
				if e != nil || endPoint != mp[len(mp)-1] {
					return fail()
				}
				uvParts = append(uvParts, uv)
				modelParts = append(modelParts, mc.Geometry)
				result.SourceEdges = append(result.SourceEdges, eid)
			}
			if previousEnd != firstVertex {
				return fail()
			}
			uv, e := joinOvalQuarters(uvParts)
			if e != nil {
				return BoundedSplineFace{}, e
			}
			modelCurve, e := joinOvalQuarters(modelParts)
			if e != nil {
				return BoundedSplineFace{}, e
			}
			if len(edges) == 4 {
				positive := uv.Poles()[2].Y > uv.Poles()[0].Y
				if outer && !positive {
					return fail()
				}
				outer = positive
			}
			index := len(model.Entities)
			model.Entities = append(model.Entities, iges.Entity{Spline: &iges.Spline{Curve: uv, Start: 0, End: 4}}, iges.Entity{Spline: &iges.Spline{Curve: modelCurve, Start: 0, End: 4}})
			ci := len(model.CurvesOnSurfaces)
			model.CurvesOnSurfaces = append(model.CurvesOnSurfaces, iges.CurveOnSurface{Surface: 0, ParameterCurve: index, ModelCurve: index + 1, Preference: 3})
			if outer {
				if trim.Outer >= 0 {
					return fail()
				}
				trim.Outer = ci
			} else {
				trim.Inner = append(trim.Inner, ci)
			}
		}
		result.Outer, result.Holes, e = model.QuadraticOvalBoundaries(trim)
		if e != nil {
			return BoundedSplineFace{}, e
		}
		out.Faces = append(out.Faces, result)
		return BoundedSplineFace{Geometry: surface.Geometry}, nil
	}, 1)
	if e != nil {
		return QuadraticOvalFaceModel{}, e
	}
	out.LengthUnit = b.LengthUnit
	for id, ent := range f.Entities {
		for _, rec := range ent.Records {
			switch rec.Type {
			case "EDGE_CURVE", "VERTEX_POINT", "ORIENTED_EDGE", "EDGE_LOOP", "FACE_BOUND", "FACE_OUTER_BOUND", "SURFACE_CURVE", "PCURVE", "B_SPLINE_CURVE_WITH_KNOTS":
				if !owned[id] {
					return QuadraticOvalFaceModel{}, geom.ErrInvalid
				}
			}
		}
	}
	return out, nil
}
func decodeOvalUV(f *File, id int64, maxPoints int) (*geom.BSpline, error) {
	if e := exactSplineWeightNormalization(f, id, "RATIONAL_B_SPLINE_CURVE"); e != nil {
		return nil, e
	}
	r, ok := component(f.Entities[id], "B_SPLINE_CURVE")
	if !ok {
		r, ok = component(f.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS")
		if !ok || len(r.Args) < 3 {
			return nil, geom.ErrInvalid
		}
		r.Args = r.Args[1:]
	}
	if len(r.Args) < 2 {
		return nil, geom.ErrInvalid
	}
	ids, e := listRefs(r.Args[1], 2)
	if e != nil || len(ids) > maxPoints {
		return nil, geom.ErrInvalid
	}
	local := *f
	local.Entities = map[int64]Entity{}
	for k, v := range f.Entities {
		local.Entities[k] = v
	}
	for _, pid := range ids {
		p, e := simple(f, pid, "CARTESIAN_POINT", 2)
		if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 2 {
			return nil, geom.ErrInvalid
		}
		p.Args = append([]Value(nil), p.Args...)
		p.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), p.Args[1].Items...), Value{Kind: Number, Text: "0."})}
		local.Entities[pid] = Entity{ID: pid, Records: []Record{p}}
	}
	c, e := DecodeBSplineCurve(&local, id, maxPoints)
	if e != nil {
		return nil, e
	}
	return c.Geometry, nil
}

func validatePCurveContext(f *File, id int64) error {
	seen := map[string]bool{}
	for _, r := range f.Entities[id].Records {
		if seen[r.Type] {
			return geom.ErrInvalid
		}
		seen[r.Type] = true
		switch r.Type {
		case "GEOMETRIC_REPRESENTATION_CONTEXT":
			if len(r.Args) != 1 {
				return geom.ErrInvalid
			}
			n, e := integer(r.Args[0])
			if e != nil || n != 2 {
				return geom.ErrInvalid
			}
		case "PARAMETRIC_REPRESENTATION_CONTEXT":
			if len(r.Args) != 0 {
				return geom.ErrInvalid
			}
		case "REPRESENTATION_CONTEXT":
			if len(r.Args) != 2 || r.Args[0].Kind != String || r.Args[1].Kind != String {
				return geom.ErrInvalid
			}
		default:
			return geom.ErrInvalid
		}
	}
	if len(seen) != 3 {
		return geom.ErrInvalid
	}
	return nil
}

func joinOvalQuarters(parts []*geom.BSpline) (*geom.BSpline, error) {
	if len(parts) == 1 {
		return parts[0], nil
	}
	if len(parts) != 4 {
		return nil, geom.ErrInvalid
	}
	var poles []geom.Vec3
	var weights []float64
	for partIndex, c := range parts {
		p, w, k := c.Poles(), c.Weights(), c.Knots()
		if c.Degree() != 2 || len(p) != 3 || len(w) != 3 || len(k) != 6 || w[0] != 1 || w[2] != 1 {
			return nil, geom.ErrInvalid
		}
		for i, want := range []float64{float64(partIndex), float64(partIndex), float64(partIndex), float64(partIndex + 1), float64(partIndex + 1), float64(partIndex + 1)} {
			if k[i] != want {
				return nil, geom.ErrInvalid
			}
		}
		if len(poles) > 0 {
			if poles[len(poles)-1] != p[0] {
				return nil, geom.ErrInvalid
			}
			p = p[1:]
			w = w[1:]
		}
		poles = append(poles, p...)
		weights = append(weights, w...)
	}
	return geom.NewBSpline(2, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, weights)
}
