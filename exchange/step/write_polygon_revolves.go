package step

import (
	"context"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"unicode/utf8"
)

// PolygonRevolveBody sweeps a simple radius/height meridian about Frame Z.
// This export subset requires 3–32 strictly positive-radius vertices and a
// nonzero signed partial angle of magnitude less than one full turn.
// Original concave meridians and planar/cylindrical/conical sides are retained.
type PolygonRevolveBody struct {
	Name    string
	Frame   geom.Frame
	Profile []geom.Vec2
	Angle   float64
}
type PolygonRevolveModel struct {
	Bodies     []PolygonRevolveBody
	LengthUnit float64
}
type revolveSTEPFace struct {
	frame               geom.Frame
	kind                string
	radius, angle, sign float64
	sense               bool
}

// WritePolygonRevolves writes closed analytic ADVANCED_FACE graphs with shared
// line/circle edges and explicit owner pcurves. It validates the native solid
// before output. This is an export API, not a generic revolve STEP decoder.
func WritePolygonRevolves(w io.Writer, m PolygonRevolveModel) error {
	if w == nil || len(m.Bodies) == 0 || len(m.Bodies) > 64 {
		return geom.ErrInvalid
	}
	bodies := make([]PolygonRevolveBody, len(m.Bodies))
	specs := make([][]revolveSTEPFace, len(m.Bodies))
	for k, b := range m.Bodies {
		if !utf8.ValidString(b.Name) || len(b.Name) > 4096 || len(b.Profile) < 3 || len(b.Profile) > 32 || !finiteNumber(b.Angle) || b.Angle == 0 || math.Abs(b.Angle) >= 2*math.Pi {
			return geom.ErrInvalid
		}
		for _, p := range b.Profile {
			if p.X <= 0 {
				return geom.ErrInvalid
			}
		}
		p, _, e := parametric.PrepareRevolutionMeridian(b.Frame, b.Profile)
		if e != nil {
			return e
		}
		b.Profile = p
		if _, e = trimmed.RevolvePolygon(context.Background(), b.Frame, p, b.Angle); e != nil {
			return e
		}
		fs := make([]revolveSTEPFace, len(p)+2)
		for i, a := range p {
			z := p[(i+1)%len(p)]
			dr, dz := z.X-a.X, z.Y-a.Y
			f, e := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{Z: a.Y}), b.Frame.XAxis(), b.Frame.ZAxis())
			if e != nil {
				return e
			}
			s := revolveSTEPFace{frame: f, radius: a.X, sign: 1, sense: dz > 0}
			switch {
			case dz == 0:
				s.kind = "PLANE"
				s.sense = dr < 0
			case dr == 0:
				s.kind = "CYLINDRICAL_SURFACE"
			default:
				s.kind = "CONICAL_SURFACE"
				s.sign = math.Copysign(1, dr/dz)
				s.angle = math.Atan(math.Abs(dr / dz))
				if !finiteNumber(s.angle) || s.angle <= 0 || s.angle >= math.Pi/2 {
					return geom.ErrUnrepresentable
				}
				s.frame, e = geom.NewFrame(f.Origin(), f.XAxis(), f.ZAxis().Scale(s.sign))
				if e != nil {
					return e
				}
			}
			fs[i] = s
		}
		for j, u := range []float64{math.Min(0, b.Angle), math.Max(0, b.Angle)} {
			radial := b.Frame.XAxis().Scale(math.Cos(u)).Add(b.Frame.YAxis().Scale(math.Sin(u)))
			f, e := geom.NewFrame(b.Frame.Origin(), radial, radial.Cross(b.Frame.ZAxis()))
			if e != nil {
				return e
			}
			fs[len(p)+j] = revolveSTEPFace{frame: f, kind: "PLANE", sense: j == 0}
		}
		bodies[k], specs[k] = b, fs
	}
	return writeStandalone(w, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		direction := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		place := func(f geom.Frame) int64 {
			p, x, z := point(f.Origin()), direction(f.XAxis()), direction(f.ZAxis())
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", p, z, x))
		}
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		line2 := func(p, d geom.Vec2) int64 {
			pt := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(p.X), real(p.Y)))
			di := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(d.X), real(d.Y)))
			v := emit(fmt.Sprintf("VECTOR('',#%d,%s)", di, real(math.Hypot(d.X, d.Y))))
			return emit(fmt.Sprintf("LINE('',#%d,#%d)", pt, v))
		}
		circle2 := func(r float64) int64 {
			p := emit("CARTESIAN_POINT('',(0.,0.))")
			d := emit("DIRECTION('',(1.,0.))")
			a := emit(fmt.Sprintf("AXIS2_PLACEMENT_2D('',#%d,#%d)", p, d))
			return emit(fmt.Sprintf("CIRCLE('',#%d,%s)", a, real(r)))
		}
		pc := func(surface, curve int64) int64 {
			r := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", curve, ctx))
			return emit(fmt.Sprintf("PCURVE('',#%d,#%d)", surface, r))
		}
		roots := []int64{}
		for body, b := range bodies {
			n := len(b.Profile)
			lo, hi := math.Min(0, b.Angle), math.Max(0, b.Angle)
			fs := specs[body]
			points := make([]geom.Vec3, 2*n)
			vertices := make([]int64, 2*n)
			surfaces := make([]int64, n+2)
			edges := make([]int64, 3*n)
			for j, u := range []float64{lo, hi} {
				for i, p := range b.Profile {
					v := b.Frame.ToWorld(geom.Vec3{X: p.X * math.Cos(u), Y: p.X * math.Sin(u), Z: p.Y})
					points[j*n+i] = v
					pt := point(v)
					vertices[j*n+i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", pt))
				}
			}
			for i, s := range fs {
				p := place(s.frame)
				switch s.kind {
				case "PLANE":
					surfaces[i] = emit(fmt.Sprintf("PLANE('',#%d)", p))
				case "CYLINDRICAL_SURFACE":
					surfaces[i] = emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", p, real(s.radius)))
				default:
					surfaces[i] = emit(fmt.Sprintf("CONICAL_SURFACE('',#%d,%s,%s)", p, real(s.radius), real(s.angle)))
				}
			}
			for i, p := range b.Profile {
				f, _ := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{Z: p.Y}), b.Frame.XAxis(), b.Frame.ZAxis())
				a := place(f)
				circle := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", a, real(p.X)))
				pcs := []int64{}
				for _, owner := range []int{(i + n - 1) % n, i} {
					s := fs[owner]
					var c int64
					if s.kind == "PLANE" {
						c = circle2(p.X)
					} else {
						height := p.Y - b.Profile[owner].Y
						if s.kind == "CONICAL_SURFACE" {
							height /= s.sign * math.Cos(s.angle)
						}
						c = line2(geom.Vec2{Y: height}, geom.Vec2{X: s.sign})
					}
					pcs = append(pcs, pc(surfaces[owner], c))
				}
				sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,%s,.CURVE_3D.)", circle, references(pcs)))
				edges[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[i], vertices[n+i], sc))
			}
			for j, u := range []float64{lo, hi} {
				for i, p := range b.Profile {
					next := (i + 1) % n
					origin := points[j*n+i]
					delta := points[j*n+next].Sub(origin)
					unit, _ := delta.Unit()
					pt, di := point(origin), direction(unit)
					v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", di))
					line := emit(fmt.Sprintf("LINE('',#%d,#%d)", pt, v))
					pcs := []int64{}
					s := fs[i]
					if s.kind == "PLANE" {
						q := s.frame.ToLocal(origin)
						pcs = append(pcs, pc(surfaces[i], line2(geom.Vec2{X: q.X, Y: q.Y}, geom.Vec2{X: unit.Dot(s.frame.XAxis()), Y: unit.Dot(s.frame.YAxis())})))
					} else {
						v := unit.Dot(b.Frame.ZAxis())
						if s.kind == "CONICAL_SURFACE" {
							v /= s.sign * math.Cos(s.angle)
						}
						pcs = append(pcs, pc(surfaces[i], line2(geom.Vec2{X: s.sign * u}, geom.Vec2{Y: v})))
					}
					d := geom.Vec2{X: (b.Profile[next].X - p.X) / delta.Length(), Y: (b.Profile[next].Y - p.Y) / delta.Length()}
					pcs = append(pcs, pc(surfaces[n+j], line2(p, d)))
					sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,%s,.CURVE_3D.)", line, references(pcs)))
					edges[n+j*n+i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[j*n+i], vertices[j*n+next], sc))
				}
			}
			faces := []int64{}
			addFace := func(index int, ids []int, forward []bool) {
				uses := []int64{}
				for j, id := range ids {
					sense := ".F."
					if forward[j] {
						sense = ".T."
					}
					uses = append(uses, emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edges[id], sense)))
				}
				loop := emit("EDGE_LOOP(''," + references(uses) + ")")
				bound := emit(fmt.Sprintf("FACE_BOUND('',#%d,.T.)", loop))
				sense := ".F."
				if fs[index].sense {
					sense = ".T."
				}
				faces = append(faces, emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,%s)", bound, surfaces[index], sense)))
			}
			for i := range b.Profile {
				addFace(i, []int{i, 2*n + i, (i + 1) % n, n + i}, []bool{true, true, false, false})
			}
			ids := make([]int, n)
			forward := make([]bool, n)
			for i := range ids {
				ids[i] = n + i
				forward[i] = true
			}
			addFace(n, ids, forward)
			for i := range ids {
				ids[i] = 2*n + n - 1 - i
				forward[i] = false
			}
			addFace(n+1, ids, forward)
			shell := emit("CLOSED_SHELL(''," + references(faces) + ")")
			roots = append(roots, emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(b.Name), shell)))
		}
		return roots
	})
}
