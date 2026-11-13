package step

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"unicode/utf8"
)

// SphericalCapBody keeps the sphere above the plane at Radius*sin(Latitude)
// along Frame's Z axis. Latitude is strictly between -pi/2 and pi/2.
type SphericalCapBody struct {
	Name             string
	Frame            geom.Frame
	Radius, Latitude float64
}
type SphericalCapModel struct {
	Bodies     []SphericalCapBody
	LengthUnit float64
}

func WriteSphericalCaps(w io.Writer, m SphericalCapModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing spherical-cap writer/model")
	}
	for _, b := range m.Bodies {
		if !utf8.ValidString(b.Name) || !finiteNumber(b.Latitude) || math.Abs(b.Latitude) >= math.Pi/2 {
			return errors.New("step: invalid cap latitude/name")
		}
		if _, e := geom.NewSphereSurface(b.Frame, b.Radius); e != nil {
			return e
		}
		p, e := geom.NewPlane(b.Frame.ToWorld(geom.Vec3{Z: b.Radius * math.Sin(b.Latitude)}), b.Frame.ZAxis().Scale(-1))
		if e != nil {
			return e
		}
		if _, e = parametric.ClipSphere(parametric.Ball{Center: b.Frame.Origin(), Radius: b.Radius}, p, b.Radius*1e-10); e != nil {
			return e
		}
	}
	var output bytes.Buffer
	e := writeStandalone(&output, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p geom.Vec3, kind string) int64 {
			return emit(fmt.Sprintf("%s('',(%s,%s,%s))", kind, real(p.X), real(p.Y), real(p.Z)))
		}
		place := func(o, x, z geom.Vec3) int64 {
			p, n, r := point(o, "CARTESIAN_POINT"), point(z, "DIRECTION"), point(x, "DIRECTION")
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", p, n, r))
		}
		ids := []int64{}
		for _, b := range m.Bodies {
			f := b.Frame
			rimRadius := b.Radius * math.Cos(b.Latitude)
			center := f.ToWorld(geom.Vec3{Z: b.Radius * math.Sin(b.Latitude)})
			sp := place(f.Origin(), f.XAxis(), f.ZAxis())
			sphere := emit(fmt.Sprintf("SPHERICAL_SURFACE('',#%d,%s)", sp, real(b.Radius)))
			cp := place(center, f.XAxis(), f.ZAxis())
			plane := emit(fmt.Sprintf("PLANE('',#%d)", cp))
			rim := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", cp, real(rimRadius)))
			ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
			pc := func(sid, g int64) int64 {
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", g, ctx))
				return emit(fmt.Sprintf("PCURVE('',#%d,#%d)", sid, rep))
			}
			line := func(u, v, dx, dy float64) int64 {
				p := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(u), real(v)))
				d := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(dx), real(dy)))
				vec := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
				return emit(fmt.Sprintf("LINE('',#%d,#%d)", p, vec))
			}
			rimPC := pc(sphere, line(0, b.Latitude, 1, 0))
			o := emit("CARTESIAN_POINT('',(0.,0.))")
			x := emit("DIRECTION('',(1.,0.))")
			p2 := emit(fmt.Sprintf("AXIS2_PLACEMENT_2D('',#%d,#%d)", o, x))
			c2 := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", p2, real(rimRadius)))
			capPC := pc(plane, c2)
			rimCurve := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d,#%d),.PCURVE_S1.)", rim, rimPC, capPC))
			meridianFrame := place(f.Origin(), f.XAxis(), f.YAxis().Scale(-1))
			meridian := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", meridianFrame, real(b.Radius)))
			a, c := pc(sphere, line(0, 0, 0, 1)), pc(sphere, line(2*math.Pi, 0, 0, 1))
			seamCurve := emit(fmt.Sprintf("SEAM_CURVE('',#%d,(#%d,#%d),.PCURVE_S1.)", meridian, a, c))
			v0p, v1p := point(center.Add(f.XAxis().Scale(rimRadius)), "CARTESIAN_POINT"), point(f.ToWorld(geom.Vec3{Z: b.Radius}), "CARTESIAN_POINT")
			v0 := emit(fmt.Sprintf("VERTEX_POINT('',#%d)", v0p))
			v1 := emit(fmt.Sprintf("VERTEX_POINT('',#%d)", v1p))
			rimEdge := emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", v0, v0, rimCurve))
			seamEdge := emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", v0, v1, seamCurve))
			use := func(edge int64, sense bool) int64 {
				s := "F"
				if sense {
					s = "T"
				}
				return emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,.%s.)", edge, s))
			}
			face := func(sid int64, sense bool, uses []int64) int64 {
				loop := emit("EDGE_LOOP(''," + references(uses) + ")")
				bound := emit(fmt.Sprintf("FACE_BOUND('',#%d,.T.)", loop))
				s := "F"
				if sense {
					s = "T"
				}
				return emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,.%s.)", bound, sid, s))
			}
			side := face(sphere, true, []int64{use(seamEdge, false), use(rimEdge, true), use(seamEdge, true)})
			cap := face(plane, false, []int64{use(rimEdge, false)})
			shell := emit("CLOSED_SHELL(''," + references([]int64{side, cap}) + ")")
			ids = append(ids, emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(b.Name), shell)))
		}
		return ids
	})
	if e != nil {
		return e
	}
	if _, e = ReadAnalyticBodies(bytes.NewReader(output.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(output.Bytes())
	if e == nil && n != output.Len() {
		return io.ErrShortWrite
	}
	return e
}
