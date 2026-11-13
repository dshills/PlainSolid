package step

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"plainsolid/geom"
	"unicode/utf8"
)

// WriteOrthogonalSplineFaces emits exact restricted model isocurves and source
// UV polygon semantics. Every spline coefficient and vertex must be losslessly
// representable; invalid/unsupported geometry fails before output.
func WriteOrthogonalSplineFaces(w io.Writer, m OrthogonalSplineFaceModel) error {
	sizes := make([]int, len(m.Faces))
	for i := range sizes {
		sizes[i] = 1
	}
	return writeOrthogonalSplineFaces(w, m, sizes)
}

func writeOrthogonalSplineFaces(w io.Writer, m OrthogonalSplineFaceModel, sizes []int) error {
	if w == nil || len(m.Faces) == 0 {
		return errors.New("step: missing orthogonal face writer/model")
	}
	type stage struct {
		surface, curves *File
		sid             int64
		cids            []int64
		vertices        []geom.Vec3
	}
	parts := make([]stage, len(m.Faces))
	for i, face := range m.Faces {
		if !utf8.ValidString(face.Name) {
			return errors.New("step: invalid orthogonal face name")
		}
		exact, e := orthogonalBoundaries(face.Geometry, face.Boundary)
		if e != nil {
			return e
		}
		cm := CurveModel{LengthUnit: m.LengthUnit}
		parts[i].vertices = make([]geom.Vec3, len(exact))
		for j, c := range exact {
			curve, e := c.ToBSpline()
			if e != nil {
				return e
			}
			cm.Curves = append(cm.Curves, SplineCurve{Geometry: curve, Closed: "F", SelfIntersect: "F"})
			a, b := face.Boundary[j], face.Boundary[(j+1)%len(exact)]
			lo, hi := c.Domain()
			parameter := lo
			if a.X > b.X || a.Y > b.Y {
				parameter = hi
			}
			point, e := c.ExactPoint(parameter)
			if e != nil {
				return e
			}
			parts[i].vertices[j] = point
		}
		var raw bytes.Buffer
		if e = WriteSurfaces(&raw, SurfaceModel{LengthUnit: m.LengthUnit, Surfaces: []SplineSurface{{Geometry: face.Geometry, ClosedU: "F", ClosedV: "F", SelfIntersect: "F"}}}); e != nil {
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
		if e = WriteCurves(&raw, cm); e != nil {
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
		if parts[i].sid == 0 || len(parts[i].cids) != len(exact) {
			return errors.New("step: invalid staged orthogonal geometry")
		}
	}
	return writeStandalone(w, m.LengthUnit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		roots := []int64{}
		group, offset := 0, 0
		var sid int64
		var bounds []int64
		var name string
		for i, part := range parts {
			face := m.Faces[i]
			hole := offset > 0
			if !hole {
				sid = copySplineGeometry(emit, part.surface, []int64{part.sid})[0]
				bounds = nil
				name = face.Name
			}
			curves := copySplineGeometry(emit, part.curves, part.cids)
			vertices := make([]int64, len(part.vertices))
			for j, p := range part.vertices {
				point := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
				vertices[j] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point))
			}
			uses := []int64{}
			for j, a := range face.Boundary {
				next := (j + 1) % len(vertices)
				b := face.Boundary[next]
				x, y, dx, dy := 0., a.Y, 1., 0.
				forward := b.X > a.X
				if a.X == b.X {
					x, y, dx, dy = a.X, 0, 0, 1
					forward = b.Y > a.Y
				}
				point := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(x), real(y)))
				dir := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(dx), real(dy)))
				vector := emit(fmt.Sprintf("VECTOR('',#%d,1.)", dir))
				line := emit(fmt.Sprintf("LINE('',#%d,#%d)", point, vector))
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", line, ctx))
				pc := emit(fmt.Sprintf("PCURVE('',#%d,#%d)", sid, rep))
				sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d),.PCURVE_S1.)", curves[j], pc))
				start, end := vertices[j], vertices[next]
				sense := ".T."
				if !forward {
					start, end = end, start
					sense = ".F."
				}
				edge := emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", start, end, sc))
				if hole {
					if sense == ".T." {
						sense = ".F."
					} else {
						sense = ".T."
					}
				}
				uses = append(uses, emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edge, sense)))
			}
			if hole {
				for a, b := 0, len(uses)-1; a < b; a, b = a+1, b-1 {
					uses[a], uses[b] = uses[b], uses[a]
				}
			}
			loop := emit("EDGE_LOOP(''," + references(uses) + ")")
			kind := "FACE_OUTER_BOUND"
			if hole {
				kind = "FACE_BOUND"
			}
			bounds = append(bounds, emit(fmt.Sprintf("%s('',#%d,.T.)", kind, loop)))
			offset++
			if offset == sizes[group] {
				fid := emit(fmt.Sprintf("ADVANCED_FACE(%s,%s,#%d,.T.)", quote(name), references(bounds), sid))
				shell := emit(fmt.Sprintf("OPEN_SHELL('',(#%d))", fid))
				roots = append(roots, emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell)))
				group++
				offset = 0
			}
		}
		return roots
	})
}

// WriteOrthogonalHoledSplineFaces preserves exact original spline and UV data.
// All input loops are counterclockwise; emitted holes traverse clockwise.
func WriteOrthogonalHoledSplineFaces(w io.Writer, m OrthogonalHoledSplineFaceModel) error {
	flat := OrthogonalSplineFaceModel{LengthUnit: m.LengthUnit}
	sizes := make([]int, len(m.Faces))
	for i, face := range m.Faces {
		if e := validateOrthogonalHoles(face.Boundary, face.Holes); e != nil {
			return e
		}
		flat.Faces = append(flat.Faces, OrthogonalSplineFace{Name: face.Name, Geometry: face.Geometry, Boundary: face.Boundary})
		for _, hole := range face.Holes {
			flat.Faces = append(flat.Faces, OrthogonalSplineFace{Geometry: face.Geometry, Boundary: hole})
		}
		sizes[i] = 1 + len(face.Holes)
	}
	return writeOrthogonalSplineFaces(w, flat, sizes)
}
