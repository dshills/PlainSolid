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

// CylinderSectorBody is a solid circular sector swept from Frame X toward Y.
type CylinderSectorBody struct {
	Name                  string
	Frame                 geom.Frame
	Radius, Height, Angle float64
}
type CylinderSectorModel struct {
	Bodies     []CylinderSectorBody
	LengthUnit float64
}

func WriteCylinderSectors(w io.Writer, m CylinderSectorModel) error {
	converted := ConicalSectorModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		converted.Bodies = append(converted.Bodies, ConicalSectorBody{b.Name, b.Frame, b.Radius, b.Radius, b.Height, b.Angle})
	}
	return writeSectorFrusta(w, converted)
}
func writeSectorFrusta(w io.Writer, m ConicalSectorModel) error {

	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing cylinder sector model/writer")
	}
	for _, b := range m.Bodies {
		if !utf8.ValidString(b.Name) {
			return errors.New("step: invalid sector name")
		}
		if b.BaseRadius == b.TopRadius {
			if _, e := parametric.CylinderSector(b.Frame, b.BaseRadius, b.Height, b.Angle); e != nil {
				return e
			}
		} else {
			if _, e := parametric.ConicalSector(b.Frame, b.BaseRadius, b.TopRadius, b.Height, b.Angle); e != nil {
				return e
			}
		}
	}
	return writeStandalone(w, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		dir := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		place := func(f geom.Frame) int64 {
			p, x, z := point(f.Origin()), dir(f.XAxis()), dir(f.ZAxis())
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", p, z, x))
		}
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		pcurve := func(sid, g int64) int64 {
			rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", g, ctx))
			return emit(fmt.Sprintf("PCURVE('',#%d,#%d)", sid, rep))
		}
		line2 := func(p, d geom.Vec3) int64 {
			pid := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(p.X), real(p.Y)))
			did := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(d.X), real(d.Y)))
			v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", did))
			return emit(fmt.Sprintf("LINE('',#%d,#%d)", pid, v))
		}
		circle2 := func(r float64) int64 {
			p := emit("CARTESIAN_POINT('',(0.,0.))")
			d := emit("DIRECTION('',(1.,0.))")
			a := emit(fmt.Sprintf("AXIS2_PLACEMENT_2D('',#%d,#%d)", p, d))
			return emit(fmt.Sprintf("CIRCLE('',#%d,%s)", a, real(r)))
		}
		roots := []int64{}
		for _, b := range m.Bodies {
			points, planes := sectorFrustumGeometry(b.Frame, b.BaseRadius, b.TopRadius, b.Height, b.Angle)
			var vertices [6]int64
			var surfaces [5]int64
			var edges [9]int64
			for i, p := range points {
				v := point(p)
				vertices[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", v))
			}
			for i, f := range planes {
				p := place(f)
				if i == 0 {
					if b.BaseRadius == b.TopRadius {
						surfaces[i] = emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", p, real(b.BaseRadius)))
					} else {
						cf, _ := geom.NewFrame(f.Origin(), f.XAxis(), f.ZAxis().Scale(-1))
						p = place(cf)
						surfaces[i] = emit(fmt.Sprintf("CONICAL_SURFACE('',#%d,%s,%s)", p, real(b.BaseRadius), real(math.Atan((b.BaseRadius-b.TopRadius)/b.Height))))
					}
				} else {
					surfaces[i] = emit(fmt.Sprintf("PLANE('',#%d)", p))
				}
			}
			for i, ends := range sectorEnds {
				var geometry int64
				origin := points[ends[0]]
				direction := points[ends[1]].Sub(origin)
				direction, _ = direction.Unit()
				if i < 2 {
					p := place(planes[i+1])
					radius := b.BaseRadius
					if i == 1 {
						radius = b.TopRadius
					}
					geometry = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", p, real(radius)))
				} else {
					p, d := point(origin), dir(direction)
					v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
					geometry = emit(fmt.Sprintf("LINE('',#%d,#%d)", p, v))
				}
				pcs := []int64{}
				for face, loop := range sectorLoops {
					for _, u := range loop {
						if u.edge != i {
							continue
						}
						var g int64
						if face == 0 {
							p, d := geom.Vec3{}, geom.Vec3{X: 1}
							if i == 1 {
								p.Y = b.Height
							}
							if i >= 2 {
								d = geom.Vec3{Y: 1}
								if i == 3 {
									p.X = b.Angle
								}
							}
							if b.BaseRadius != b.TopRadius {
								p = p.Scale(-1)
								d = d.Scale(-1)
							}
							g = line2(p, d)
						} else if i < 2 {
							radius := b.BaseRadius
							if i == 1 {
								radius = b.TopRadius
							}
							g = circle2(radius)
						} else {
							pf := planes[face]
							p := pf.ToLocal(origin)
							d := geom.Vec3{X: direction.Dot(pf.XAxis()), Y: direction.Dot(pf.YAxis())}
							g = line2(p, d)
						}
						pcs = append(pcs, pcurve(surfaces[face], g))
					}
				}
				sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,%s,.PCURVE_S1.)", geometry, references(pcs)))
				edges[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[ends[0]], vertices[ends[1]], sc))
			}
			faces := []int64{}
			for i, loop := range sectorLoops {
				uses := []int64{}
				for _, u := range loop {
					s := ".F."
					if u.sense {
						s = ".T."
					}
					uses = append(uses, emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edges[u.edge], s)))
				}
				l := emit("EDGE_LOOP(''," + references(uses) + ")")
				bound := emit(fmt.Sprintf("FACE_BOUND('',#%d,.T.)", l))
				sense := ".F."
				if sectorSenses[i] {
					sense = ".T."
				}
				faces = append(faces, emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,%s)", bound, surfaces[i], sense)))
			}
			shell := emit("CLOSED_SHELL(''," + references(faces) + ")")
			roots = append(roots, emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(b.Name), shell)))
		}
		return roots
	})
}
