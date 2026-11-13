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

// ConeBody specifies a narrowing finite cone or frustum along Frame positive Z.
// BaseRadius must exceed TopRadius >= 0; zero TopRadius produces an apex.
type ConeBody struct {
	Name                          string
	Frame                         geom.Frame
	BaseRadius, TopRadius, Height float64
}
type ConeModel struct {
	Bodies     []ConeBody
	LengthUnit float64
}

// WriteCones emits exact narrowing cones/frusta with shared circular rims, a
// generator seam and full-domain pcurves. An apex has no spurious zero-radius rim.
func WriteCones(w io.Writer, m ConeModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing cylinder writer/model")
	}
	for _, s := range m.Bodies {
		if !utf8.ValidString(s.Name) {
			return errors.New("step: invalid cylinder name")
		}
		if _, e := conicalSolid(s.Frame, s.BaseRadius, s.TopRadius, s.Height); e != nil {
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
			f, r, h := s.Frame, s.BaseRadius, s.Height
			placement := place(f.Origin(), f.XAxis(), f.ZAxis().Scale(-1))
			surface := emit(fmt.Sprintf("CONICAL_SURFACE('',#%d,%s,%s)", placement, real(r), real(math.Atan((r-s.TopRadius)/h))))
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
				p := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(-u), real(-v)))
				d := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(-dx), real(-dy)))
				vector := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
				line := emit(fmt.Sprintf("LINE('',#%d,#%d)", p, vector))
				return pcurve(surface, line)
			}
			var vertices, edges, planes [2]int64
			for k := 0; k < 2; k++ {
				z := float64(k) * h
				center := f.Origin().Add(f.ZAxis().Scale(z))
				rad := r
				if k == 1 {
					rad = s.TopRadius
				}
				if rad == 0 {
					outer := point(center)
					vertices[k] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", outer))
					continue
				}
				pl := place(center, f.XAxis(), f.ZAxis())
				planes[k] = emit(fmt.Sprintf("PLANE('',#%d)", pl))
				outer := point(center.Add(f.XAxis().Scale(rad)))
				vertices[k] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", outer))
				circle := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", pl, real(rad)))
				sidePC := linePC(0, z)
				origin2 := emit("CARTESIAN_POINT('',(0.,0.))")
				x2 := emit("DIRECTION('',(1.,0.))")
				pl2 := emit(fmt.Sprintf("AXIS2_PLACEMENT_2D('',#%d,#%d)", origin2, x2))
				circle2 := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", pl2, real(rad)))
				capPC := pcurve(planes[k], circle2)
				curve := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d,#%d),.PCURVE_S1.)", circle, sidePC, capPC))
				edges[k] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[k], vertices[k], curve))
			}
			base := point(f.Origin().Add(f.XAxis().Scale(r)))
			generator, _ := f.ZAxis().Scale(h).Add(f.XAxis().Scale(s.TopRadius - r)).Unit()
			axis := direction(generator)
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
			sideUses := []int64{oriented(seam, false), oriented(edges[0], true), oriented(seam, true)}
			if s.TopRadius > 0 {
				sideUses = append([]int64{oriented(edges[1], false)}, sideUses...)
			}
			side := face(sideUses, surface, true)
			bottom := face([]int64{oriented(edges[0], false)}, planes[0], false)
			faces := []int64{side, bottom}
			if s.TopRadius > 0 {
				faces = append(faces, face([]int64{oriented(edges[1], true)}, planes[1], true))
			}
			shell := emit("CLOSED_SHELL(''," + references(faces) + ")")
			ids[i] = emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(s.Name), shell))
		}
		return ids
	})
}

func conicalSolid(f geom.Frame, base, top, height float64) (parametric.Solid, error) {
	if !finiteNumber(base) || !finiteNumber(top) || !finiteNumber(height) || base <= top || top < 0 || height <= 0 {
		return parametric.Solid{}, errors.New("step: cone requires base>top>=0 and positive height")
	}
	angle := math.Atan((base - top) / height)
	if angle <= 0 || angle >= math.Pi/2 {
		return parametric.Solid{}, errors.New("step: cone angle outside representable range")
	}
	profile := []geom.Vec2{{}, {X: base}, {X: top, Y: height}}
	if top > 0 {
		profile = append(profile, geom.Vec2{Y: height})
	}
	return parametric.Revolve(f, profile)
}
