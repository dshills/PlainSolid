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

// CylinderBody explicitly specifies a complete finite cylinder from Frame's
// origin along its positive Z axis, with two plane caps.
type CylinderBody struct {
	Name           string
	Frame          geom.Frame
	Radius, Height float64
}
type CylinderModel struct {
	Bodies     []CylinderBody
	LengthUnit float64
}

// WriteCylinders emits exact analytic cylinder boundaries with shared circular
// rims, a longitudinal seam, and checked full-domain pcurve representations.
func WriteCylinders(w io.Writer, m CylinderModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing cylinder writer/model")
	}
	for _, s := range m.Bodies {
		if !utf8.ValidString(s.Name) {
			return errors.New("step: invalid cylinder name")
		}
		if _, e := parametric.Cylinder(s.Frame, s.Radius, s.Height); e != nil {
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
			f, r, h := s.Frame, s.Radius, s.Height
			placement := place(f.Origin(), f.XAxis(), f.ZAxis())
			surface := emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", placement, real(r)))
			ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
			pcurve := func(sid, geometry int64) int64 {
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", geometry, ctx))
				return emit(fmt.Sprintf("PCURVE('',#%d,#%d)", sid, rep))
			}
			linePC := func(axis int, offset float64) int64 {
				u, v, dx, dy := 0.0, offset, 1.0, 0.0
				if axis == 1 {
					u, v, dx, dy = offset, 0, 0, 1
				}
				p := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(u), real(v)))
				d := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(dx), real(dy)))
				vector := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
				line := emit(fmt.Sprintf("LINE('',#%d,#%d)", p, vector))
				return pcurve(surface, line)
			}
			var vertices, edges, planes [2]int64
			for k := 0; k < 2; k++ {
				z := float64(k) * h
				center := f.Origin().Add(f.ZAxis().Scale(z))
				pl := place(center, f.XAxis(), f.ZAxis())
				planes[k] = emit(fmt.Sprintf("PLANE('',#%d)", pl))
				outer := point(center.Add(f.XAxis().Scale(r)))
				vertices[k] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", outer))
				circle := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", pl, real(r)))
				sidePC := linePC(0, z)
				origin2 := emit("CARTESIAN_POINT('',(0.,0.))")
				x2 := emit("DIRECTION('',(1.,0.))")
				pl2 := emit(fmt.Sprintf("AXIS2_PLACEMENT_2D('',#%d,#%d)", origin2, x2))
				circle2 := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", pl2, real(r)))
				capPC := pcurve(planes[k], circle2)
				curve := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d,#%d),.PCURVE_S1.)", circle, sidePC, capPC))
				edges[k] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[k], vertices[k], curve))
			}
			base := point(f.Origin().Add(f.XAxis().Scale(r)))
			axis := direction(f.ZAxis())
			vector := emit(fmt.Sprintf("VECTOR('',#%d,1.)", axis))
			line := emit(fmt.Sprintf("LINE('',#%d,#%d)", base, vector))
			a, b := linePC(1, 0), linePC(1, 2*math.Pi)
			seamCurve := emit(fmt.Sprintf("SEAM_CURVE('',#%d,(#%d,#%d),.PCURVE_S1.)", line, a, b))
			seam := emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[0], vertices[1], seamCurve))
			oriented := func(edge int64, sense bool) int64 {
				s := ".F."
				if sense {
					s = ".T."
				}
				return emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edge, s))
			}
			face := func(uses []int64, sid int64, sense bool) int64 {
				loop := emit("EDGE_LOOP(''," + references(uses) + ")")
				bound := emit(fmt.Sprintf("FACE_BOUND('',#%d,.T.)", loop))
				s := ".F."
				if sense {
					s = ".T."
				}
				return emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,%s)", bound, sid, s))
			}
			side := face([]int64{oriented(edges[1], false), oriented(seam, false), oriented(edges[0], true), oriented(seam, true)}, surface, true)
			bottom := face([]int64{oriented(edges[0], false)}, planes[0], false)
			top := face([]int64{oriented(edges[1], true)}, planes[1], true)
			shell := emit("CLOSED_SHELL(''," + references([]int64{side, bottom, top}) + ")")
			ids[i] = emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(s.Name), shell))
		}
		return ids
	})
}
