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

// TorusElbowBody specifies a circular pipe swept from frame X toward Y.
type TorusElbowBody struct {
	Name                            string
	Frame                           geom.Frame
	MajorRadius, MinorRadius, Angle float64
}
type TorusElbowModel struct {
	Bodies     []TorusElbowBody
	LengthUnit float64
}

// WriteTorusElbows emits an exact bounded torus with two shared disk caps.
func WriteTorusElbows(w io.Writer, m TorusElbowModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing torus-elbow writer/model")
	}
	for _, s := range m.Bodies {
		if !utf8.ValidString(s.Name) {
			return errors.New("step: invalid torus-elbow name")
		}
		if _, e := parametric.TorusElbow(s.Frame, s.MajorRadius, s.MinorRadius, s.Angle); e != nil {
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
			f, major, r := s.Frame, s.MajorRadius, s.MinorRadius
			placement := place(f.Origin(), f.XAxis(), f.ZAxis())
			surface := emit(fmt.Sprintf("TOROIDAL_SURFACE('',#%d,%s,%s)", placement, real(major), real(r)))
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
				angle := 0.
				if k == 1 {
					angle = s.Angle
				}
				radial := f.XAxis().Scale(math.Cos(angle)).Add(f.YAxis().Scale(math.Sin(angle)))
				center := f.Origin().Add(radial.Scale(major))
				rimRadius := r

				pl := place(center, radial, radial.Cross(f.ZAxis()))
				planes[k] = emit(fmt.Sprintf("PLANE('',#%d)", pl))
				outer := point(center.Add(radial.Scale(rimRadius)))
				vertices[k] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", outer))
				circle := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", pl, real(rimRadius)))
				sidePC := linePC(1, angle)
				origin2 := emit("CARTESIAN_POINT('',(0.,0.))")
				x2 := emit("DIRECTION('',(1.,0.))")
				pl2 := emit(fmt.Sprintf("AXIS2_PLACEMENT_2D('',#%d,#%d)", origin2, x2))
				circle2 := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", pl2, real(rimRadius)))
				capPC := pcurve(planes[k], circle2)
				curve := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d,#%d),.PCURVE_S1.)", circle, sidePC, capPC))
				edges[k] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[k], vertices[k], curve))
			}
			meridianPlacement := place(f.Origin(), f.XAxis(), f.ZAxis())
			meridian := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", meridianPlacement, real(major+r)))

			a, b := linePC(0, 0), linePC(0, 2*math.Pi)
			seamCurve := emit(fmt.Sprintf("SEAM_CURVE('',#%d,(#%d,#%d),.PCURVE_S1.)", meridian, a, b))
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
			side := face([]int64{oriented(seam, false), oriented(edges[0], false), oriented(seam, true), oriented(edges[1], true)}, surface, true)
			bottom := face([]int64{oriented(edges[0], true)}, planes[0], true)
			top := face([]int64{oriented(edges[1], false)}, planes[1], false)
			shell := emit("CLOSED_SHELL(''," + references([]int64{side, bottom, top}) + ")")
			ids[i] = emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(s.Name), shell))
		}
		return ids
	})
}
