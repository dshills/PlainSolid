package step

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"plainsolid/geom"
	"unicode/utf8"
)

// WriteSplineFaces writes a separate one-face OPEN_SHELL per certified natural
// rectangular spline patch, retaining model edges and exact UV isocurves.
func WriteSplineFaces(w io.Writer, m SplineFaceModel) error {
	b := BoundedSplineFaceModel{LengthUnit: m.LengthUnit}
	for _, face := range m.Faces {
		if face.Geometry == nil {
			return errors.New("step: nil spline face")
		}
		u0, u1, v0, v1 := face.Geometry.Domain()
		b.Faces = append(b.Faces, BoundedSplineFace{face.Name, face.Geometry, geom.ParameterRectangle{UMin: u0, UMax: u1, VMin: v0, VMax: v1}})
	}
	return WriteBoundedSplineFaces(w, b)
}

// WriteBoundedSplineFaces preserves the original support and emits exact trim
// edges only when all coefficients and endpoint positions are representable.
func WriteBoundedSplineFaces(w io.Writer, m BoundedSplineFaceModel) error {
	return writeSplineRectangles(w, m, nil)
}

func writeSplineRectangles(w io.Writer, m BoundedSplineFaceModel, holes [][]geom.ParameterRectangle) error {

	if w == nil || len(m.Faces) == 0 {
		return errors.New("step: missing spline face writer/model")
	}
	type staged struct {
		surface *File
		sid     int64
		curves  *File
		cids    []int64
		corners [4]geom.Vec3
	}
	allFaces := append([]BoundedSplineFace(nil), m.Faces...)
	holeParts := make([][]int, len(m.Faces))
	for i, domains := range holes {
		for _, d := range domains {
			holeParts[i] = append(holeParts[i], len(allFaces))
			allFaces = append(allFaces, BoundedSplineFace{Geometry: m.Faces[i].Geometry, Domain: d})
		}
	}
	parts := make([]staged, len(allFaces))
	for i, face := range allFaces {
		if !utf8.ValidString(face.Name) {
			return errors.New("step: invalid spline face name")
		}
		exact, e := boundedSplineBoundaries(face.Geometry, face.Domain)
		if e != nil {
			return e
		}
		var curves [4]*geom.BSpline
		for j, c := range exact {
			curves[j], e = c.ToBSpline()
			if e != nil {
				return e
			}
			lo, hi := c.Domain()
			for k, t := range []float64{lo, hi} {
				p, e := c.ExactPoint(t)
				if e != nil {
					return e
				}
				parts[i].corners[splineFaceEnds[j][k]] = p
			}
		}

		var raw bytes.Buffer
		if e := WriteSurfaces(&raw, SurfaceModel{[]SplineSurface{{Geometry: face.Geometry, ClosedU: "F", ClosedV: "F", SelfIntersect: "F"}}, m.LengthUnit}); e != nil {
			return e
		}
		f, e := Parse(&raw, Limits{})
		if e != nil {
			return e
		}
		parts[i].surface = f
		for _, id := range f.Order {
			if _, ok := component(f.Entities[id], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
				parts[i].sid = id
			}
		}
		cm := CurveModel{LengthUnit: m.LengthUnit}
		for _, c := range curves {
			cm.Curves = append(cm.Curves, SplineCurve{Geometry: c, Closed: "F", SelfIntersect: "F"})
		}
		if e := WriteCurves(&raw, cm); e != nil {
			return e
		}
		f, e = Parse(&raw, Limits{})
		if e != nil {
			return e
		}
		parts[i].curves = f
		for _, id := range f.Order {
			if _, ok := component(f.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS"); ok {
				parts[i].cids = append(parts[i].cids, id)
			}
		}
		if parts[i].sid == 0 || len(parts[i].cids) != 4 {
			return errors.New("step: invalid staged spline geometry")
		}
	}
	return writeStandalone(w, m.LengthUnit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		roots := []int64{}
		for i := range m.Faces {
			p := parts[i]
			sid := copySplineGeometry(emit, p.surface, []int64{p.sid})[0]
			emitBound := func(p staged, d geom.ParameterRectangle, inner bool) int64 {
				curves := copySplineGeometry(emit, p.curves, p.cids)
				var vertices [4]int64
				var edges [4]int64
				u0, u1, v0, v1 := d.UMin, d.UMax, d.VMin, d.VMax
				for j, v := range p.corners {
					point := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(v.X), real(v.Y), real(v.Z)))
					vertices[j] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point))
				}
				for j, ends := range splineFaceEnds {
					u, v, dx, dy := 0., 0., 1., 0.
					switch j {
					case 0:
						u = u0
						dx, dy = 0, 1
					case 1:
						v = v0
					case 2:
						u = u1
						dx, dy = 0, 1
					case 3:
						v = v1
					}
					point := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(u), real(v)))
					dir := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(dx), real(dy)))
					vector := emit(fmt.Sprintf("VECTOR('',#%d,1.)", dir))
					line := emit(fmt.Sprintf("LINE('',#%d,#%d)", point, vector))
					rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", line, ctx))
					pc := emit(fmt.Sprintf("PCURVE('',#%d,#%d)", sid, rep))
					sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d),.PCURVE_S1.)", curves[j], pc))
					edges[j] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[ends[0]], vertices[ends[1]], sc))
				}
				uses := []int64{}
				for j, e := range edges {
					if inner {
						continue
					}
					s := ".F."
					if j == 1 || j == 2 {
						s = ".T."
					}
					uses = append(uses, emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", e, s)))
				}
				if inner {
					uses = nil
					for j := len(edges) - 1; j >= 0; j-- {
						sense := ".T."
						if j == 1 || j == 2 {
							sense = ".F."
						}
						uses = append(uses, emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edges[j], sense)))
					}
				}
				loop := emit("EDGE_LOOP(''," + references(uses) + ")")
				kind := "FACE_OUTER_BOUND"
				if inner {
					kind = "FACE_BOUND"
				}
				return emit(fmt.Sprintf("%s('',#%d,.T.)", kind, loop))
			}
			bounds := []int64{emitBound(p, m.Faces[i].Domain, false)}
			if len(holes) != 0 {
				for j, d := range holes[i] {
					bounds = append(bounds, emitBound(parts[holeParts[i][j]], d, true))
				}
			}
			face := emit(fmt.Sprintf("ADVANCED_FACE(%s,%s,#%d,.T.)", quote(m.Faces[i].Name), references(bounds), sid))
			shell := emit(fmt.Sprintf("OPEN_SHELL('',(#%d))", face))
			roots = append(roots, emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell)))
		}
		return roots
	})
}

func copySplineGeometry(emit func(string) int64, f *File, roots []int64) []int64 {
	out, _ := copySplineGeometryWithIDs(emit, f, roots)
	return out
}
func copySplineGeometryWithIDs(emit func(string) int64, f *File, roots []int64) ([]int64, map[int64]int64) {
	ids := map[int64]int64{}
	var copyOne func(int64) int64
	var walk func(Value)
	walk = func(v Value) {
		if v.Kind == Reference {
			copyOne(v.Ref)
		}
		for _, x := range v.Items {
			walk(x)
		}
	}
	copyOne = func(id int64) int64 {
		if n, ok := ids[id]; ok {
			return n
		}
		ent := f.Entities[id]
		for _, r := range ent.Records {
			for _, v := range r.Args {
				walk(v)
			}
		}
		ids[id] = emit(assemblyEntity(ent, ids))
		return ids[id]
	}
	result := []int64{}
	for _, id := range roots {
		result = append(result, copyOne(id))
	}
	return result, ids

}
