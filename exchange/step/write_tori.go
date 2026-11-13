package step

import (
	"errors"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"unicode/utf8"
)

// TorusBody explicitly specifies an outward complete ring torus in Frame.
type TorusBody struct {
	Name                     string
	Frame                    geom.Frame
	MajorRadius, MinorRadius float64
}
type TorusModel struct {
	Bodies     []TorusBody
	LengthUnit float64
}

// WriteTori emits exact ring tori with two circular seams and four explicit UV
// pcurves delimiting [0,2*pi] squared. Horn/spindle tori and trims are rejected.
func WriteTori(w io.Writer, m TorusModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing torus writer/model")
	}
	for _, s := range m.Bodies {
		if !utf8.ValidString(s.Name) {
			return errors.New("step: invalid torus name")
		}
		if _, e := parametric.Torus(s.Frame, s.MajorRadius, s.MinorRadius); e != nil {
			return e
		}
	}
	return writeStandalone(w, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		direction := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		place := func(origin, x, z geom.Vec3) int64 {
			p, n, r := point(origin), direction(z), direction(x)
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", p, n, r))
		}
		ids := make([]int64, len(m.Bodies))
		for i, s := range m.Bodies {
			f, R, r := s.Frame, s.MajorRadius, s.MinorRadius
			placement := place(f.Origin(), f.XAxis(), f.ZAxis())
			surface := emit(fmt.Sprintf("TOROIDAL_SURFACE('',#%d,%s,%s)", placement, real(R), real(r)))
			p := point(f.Origin().Add(f.XAxis().Scale(R + r)))
			vertex := emit(fmt.Sprintf("VERTEX_POINT('',#%d)", p))
			ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
			pcurve := func(axis int, offset float64) int64 {
				u, v, dx, dy := 0.0, offset, 1.0, 0.0
				if axis == 1 {
					u, v, dx, dy = offset, 0, 0, 1
				}
				p := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(u), real(v)))
				d := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(dx), real(dy)))
				vector := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
				line := emit(fmt.Sprintf("LINE('',#%d,#%d)", p, vector))
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", line, ctx))
				return emit(fmt.Sprintf("PCURVE('',#%d,#%d)", surface, rep))
			}
			var edges [2]int64
			for axis := 0; axis < 2; axis++ {
				origin, normal, radius := f.Origin(), f.ZAxis(), R+r
				if axis == 1 {
					origin = f.Origin().Add(f.XAxis().Scale(R))
					normal = f.YAxis().Scale(-1)
					radius = r
				}
				pl := place(origin, f.XAxis(), normal)
				circle := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", pl, real(radius)))
				a, b := pcurve(axis, 0), pcurve(axis, 2*math.Pi)
				curve := emit(fmt.Sprintf("SEAM_CURVE('',#%d,(#%d,#%d),.PCURVE_S1.)", circle, a, b))
				edges[axis] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertex, vertex, curve))
			}
			uses := make([]int64, 4)
			for j := range uses {
				sense := ".F."
				if j >= 2 {
					sense = ".T."
				}
				uses[j] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edges[j%2], sense))
			}
			loop := emit("EDGE_LOOP(''," + references(uses) + ")")
			bound := emit(fmt.Sprintf("FACE_BOUND('',#%d,.T.)", loop))
			face := emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,.T.)", bound, surface))
			shell := emit(fmt.Sprintf("CLOSED_SHELL('',(#%d))", face))
			ids[i] = emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(s.Name), shell))
		}
		return ids
	})
}
