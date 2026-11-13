package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
	"unicode/utf8"
)

// WriteCurvedOvalFace emits one exact open face with four independently paired
// quadratic pcurves and composed model splines. Unrepresentable coefficients
// fail instead of approximation. A complete reader proof precedes output.
func WriteCurvedOvalFace(w io.Writer, m CurvedOvalFace, o CurvedOvalOptions) error {
	return writeCurvedOvalFace(w, m, o, false)
}
func writeCurvedOvalFace(w io.Writer, m CurvedOvalFace, o CurvedOvalOptions, ellipse bool) error {
	if w == nil || !utf8.ValidString(m.Name) || !validLengthUnit(m.LengthUnit) {
		return geom.ErrInvalid
	}
	var e error
	o, e = o.defaults()
	if e != nil {
		return e
	}
	var uv [][4]*geom.BSpline
	var models [][4]*geom.ExactSplineCurve
	if ellipse {
		if len(m.Holes) != 0 {
			return geom.ErrInvalid
		}
		u, c, err := certifyCurvedOvalMode(m, o, true)
		e = err
		uv = [][4]*geom.BSpline{u}
		models = [][4]*geom.ExactSplineCurve{c}
	} else {
		uv, models, e = certifyCurvedOvalHoles(m, o)
	}
	if e != nil {
		return e
	}
	poles := make([][4][]geom.Vec3, len(models))
	weights := make([][4][]float64, len(models))
	endpoints := make([][4]geom.Vec3, len(models))
	for j, loop := range models {
		for i, c := range loop {
			poles[j][i], weights[j][i], e = exactBezierFloats(c, o.patch())
			if e != nil {
				return e
			}
			endpoints[j][i], e = c.ExactPoint(0)
			if e != nil {
				return e
			}
		}
	}
	var support bytes.Buffer
	if e = WriteSurfaces(&support, SurfaceModel{LengthUnit: m.LengthUnit, Surfaces: []SplineSurface{{Geometry: m.Geometry, ClosedU: "F", ClosedV: "F", SelfIntersect: "F"}}}); e != nil {
		return e
	}
	f, e := Parse(&support, Limits{})
	if e != nil {
		return e
	}
	sid := int64(0)
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
			sid = id
		}
	}
	if sid == 0 {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		surface := copySplineGeometry(emit, f, []int64{sid})[0]
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		bounds := []int64{}
		for j := range models {
			vertices := [4]int64{}
			for i, p := range endpoints[j] {
				point := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
				vertices[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point))
			}
			uses := []int64{}
			for i := range models[j] {
				uid := emitCurvedOvalBezier(emit, uv[j][i].Poles(), uv[j][i].Weights(), true)
				mid := emitCurvedOvalBezier(emit, poles[j][i], weights[j][i], false)
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", uid, ctx))
				pc := emit(fmt.Sprintf("PCURVE('',#%d,#%d)", surface, rep))
				sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d),.PCURVE_S1.)", mid, pc))
				edge := emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[i], vertices[(i+1)%4], sc))
				uses = append(uses, emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,.T.)", edge)))
			}
			loop := emit("EDGE_LOOP(''," + references(uses) + ")")
			kind := "FACE_OUTER_BOUND"
			if j > 0 {
				kind = "FACE_BOUND"
			}
			bounds = append(bounds, emit(fmt.Sprintf("%s('',#%d,.T.)", kind, loop)))
		}
		face := emit(fmt.Sprintf("ADVANCED_FACE(%s,%s,#%d,.T.)", quote(m.Name), references(bounds), surface))
		shell := emit(fmt.Sprintf("OPEN_SHELL('',(#%d))", face))
		return []int64{emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell))}
	})
	if e != nil {
		return e
	}
	if _, e = readCurvedOvalFace(bytes.NewReader(out.Bytes()), Limits{}, o, ellipse); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}
