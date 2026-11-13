package step

import (
	"context"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"unicode/utf8"
)

// CircularHoleExtrusionBody describes a signed straight extrusion of a convex
// polygon and disjoint interior circles in Frame XY. The native constructor
// proves strict circle containment/disjointness in represented coordinates.
type CircularHoleExtrusionBody struct {
	Name  string
	Frame geom.Frame
	Outer []geom.Vec2
	Holes []trimmed.ExtrusionCircle
	// Blind holes start at the original sketch cap and stop at positive Depth
	// toward the signed extrusion endpoint; floors remain explicit planar faces.
	Blind  []trimmed.ExtrusionBlindHole
	Height float64
}
type CircularHoleExtrusionModel struct {
	Bodies     []CircularHoleExtrusionBody
	LengthUnit float64
}

// WriteCircularHoleExtrusions retains analytic planes/cylinders and shared
// full circular rims. Each native four-patch hole wall is represented as one
// full cylindrical STEP face with an explicit seam, not as copied native IDs.
func WriteCircularHoleExtrusions(w io.Writer, m CircularHoleExtrusionModel) error {
	if w == nil || len(m.Bodies) == 0 || len(m.Bodies) > 32 {
		return geom.ErrInvalid
	}
	bodies := make([]CircularHoleExtrusionBody, len(m.Bodies))
	for i, b := range m.Bodies {
		if !utf8.ValidString(b.Name) || len(b.Name) > 4096 || len(b.Outer) < 3 || len(b.Outer) > 64 || len(b.Holes)+len(b.Blind) < 1 || len(b.Holes)+len(b.Blind) > 16 {
			return geom.ErrInvalid
		}
		if _, e := trimmed.ExtrudePolygonWithHoles(context.Background(), b.Frame, b.Outer, b.Holes, b.Blind, b.Height); e != nil {
			return e
		}
		b.Outer = append([]geom.Vec2(nil), b.Outer...)
		b.Holes = append([]trimmed.ExtrusionCircle(nil), b.Holes...)
		b.Blind = append([]trimmed.ExtrusionBlindHole(nil), b.Blind...)
		sign, _ := geom.Orient2D(b.Outer[0], b.Outer[1], b.Outer[2])
		if sign < 0 {
			for a, z := 0, len(b.Outer)-1; a < z; a, z = a+1, z-1 {
				b.Outer[a], b.Outer[z] = b.Outer[z], b.Outer[a]
			}
		}
		bodies[i] = b
	}
	specs := make([]circularExtrusionExport, len(bodies))
	for i, body := range bodies {
		specs[i].CircularHoleExtrusionBody = body
	}
	return writeCircularExtrusionSTEP(w, m.LengthUnit, specs)
}

type circularExtrusionExport struct {
	CircularHoleExtrusionBody
	disk *trimmed.ExtrusionCircle
	arcs []roundedSTEPArc
}

func writeCircularExtrusionSTEP(w io.Writer, unit float64, bodies []circularExtrusionExport) error {
	return writeStandalone(w, unit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
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
		line := func(a, b geom.Vec3) int64 {
			d, _ := b.Sub(a).Unit()
			p, v := point(a), dir(d)
			x := emit(fmt.Sprintf("VECTOR('',#%d,1.)", v))
			return emit(fmt.Sprintf("LINE('',#%d,#%d)", p, x))
		}
		line2 := func(p, d geom.Vec2) int64 {
			a := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(p.X), real(p.Y)))
			v := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(d.X), real(d.Y)))
			x := emit(fmt.Sprintf("VECTOR('',#%d,%s)", v, real(math.Hypot(d.X, d.Y))))
			return emit(fmt.Sprintf("LINE('',#%d,#%d)", a, x))
		}
		circle2 := func(c geom.Vec2, r float64) int64 {
			p := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(c.X), real(c.Y)))
			d := emit("DIRECTION('',(1.,0.))")
			a := emit(fmt.Sprintf("AXIS2_PLACEMENT_2D('',#%d,#%d)", p, d))
			return emit(fmt.Sprintf("CIRCLE('',#%d,%s)", a, real(r)))
		}
		var ctx int64
		pc := func(surface, curve int64) int64 {
			if ctx == 0 {
				ctx = emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
			}
			r := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", curve, ctx))
			return emit(fmt.Sprintf("PCURVE('',#%d,#%d)", surface, r))
		}
		edge := func(a, b, curve int64, pcs []int64, seam bool) int64 {
			kind := "SURFACE_CURVE"
			if seam {
				kind = "SEAM_CURVE"
			}
			s := emit(fmt.Sprintf("%s('',#%d,%s,.CURVE_3D.)", kind, curve, references(pcs)))
			return emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", a, b, s))
		}
		use := func(e int64, sense bool) int64 {
			s := ".F."
			if sense {
				s = ".T."
			}
			return emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", e, s))
		}
		bound := func(uses []int64, outer bool) int64 {
			l := emit("EDGE_LOOP(''," + references(uses) + ")")
			kind := "FACE_BOUND"
			if outer {
				kind = "FACE_OUTER_BOUND"
			}
			return emit(fmt.Sprintf("%s('',#%d,.T.)", kind, l))
		}
		face := func(surface int64, bounds []int64, sense bool) int64 {
			s := ".F."
			if sense {
				s = ".T."
			}
			return emit(fmt.Sprintf("ADVANCED_FACE('',%s,#%d,%s)", references(bounds), surface, s))
		}
		roots := []int64{}
		for _, b := range bodies {
			n := len(b.Outer)
			lo, hi := math.Min(0, b.Height), math.Max(0, b.Height)
			caps := [2]int64{}
			capFrames := [2]geom.Frame{}
			walls := make([]int64, n)
			wallFrames := make([]geom.Frame, n)
			vertices := make([]int64, 2*n)
			points := make([]geom.Vec3, 2*n)
			for j, z := range []float64{lo, hi} {
				f, _ := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{Z: z}), b.Frame.XAxis(), b.Frame.ZAxis())
				capFrames[j] = f
				caps[j] = emit(fmt.Sprintf("PLANE('',#%d)", place(f)))
				for i, p := range b.Outer {
					v := b.Frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y, Z: z})
					points[j*n+i] = v
					pt := point(v)
					vertices[j*n+i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", pt))
				}
			}
			for i := range b.Outer {
				if len(b.arcs) != 0 && b.arcs[i].radius > 0 {
					arc := b.arcs[i]
					f, _ := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{X: arc.center.X, Y: arc.center.Y, Z: lo}), b.Frame.XAxis(), b.Frame.ZAxis())
					wallFrames[i] = f
					walls[i] = emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", place(f), real(arc.radius)))
					continue
				}
				d := points[(i+1)%n].Sub(points[i])
				f, _ := geom.NewFrame(points[i], d, d.Cross(b.Frame.ZAxis()))
				wallFrames[i] = f
				walls[i] = emit(fmt.Sprintf("PLANE('',#%d)", place(f)))
			}
			planePC := func(sid int64, f geom.Frame, a, z geom.Vec3) int64 {
				p := f.ToLocal(a)
				d, _ := z.Sub(a).Unit()
				return pc(sid, line2(geom.Vec2{X: p.X, Y: p.Y}, geom.Vec2{X: d.Dot(f.XAxis()), Y: d.Dot(f.YAxis())}))
			}
			wallVerticalPC := func(i int, atEnd bool, a, z geom.Vec3) int64 {
				if len(b.arcs) != 0 && b.arcs[i].radius > 0 {
					q := b.arcs[i].quarter
					if atEnd {
						q++
					}
					return pc(walls[i], line2(geom.Vec2{X: float64(q) * math.Pi / 2}, geom.Vec2{Y: 1}))
				}
				return planePC(walls[i], wallFrames[i], a, z)
			}
			ring := [2][]int64{make([]int64, n), make([]int64, n)}
			vertical := make([]int64, n)
			for i := range b.Outer {
				next := (i + 1) % n
				for j := 0; j < 2; j++ {
					a, z := points[j*n+i], points[j*n+next]
					if len(b.arcs) != 0 && b.arcs[i].radius > 0 {
						arc := b.arcs[i]
						cap := capFrames[j]
						f, _ := geom.NewFrame(cap.ToWorld(geom.Vec3{X: arc.center.X, Y: arc.center.Y}), cap.XAxis(), cap.ZAxis())
						model := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", place(f), real(arc.radius)))
						ring[j][i] = edge(vertices[j*n+i], vertices[j*n+next], model, []int64{
							pc(walls[i], line2(geom.Vec2{Y: float64(j) * (hi - lo)}, geom.Vec2{X: 1})),
							pc(caps[j], circle2(arc.center, arc.radius)),
						}, false)
					} else {
						ring[j][i] = edge(vertices[j*n+i], vertices[j*n+next], line(a, z), []int64{planePC(caps[j], capFrames[j], a, z), planePC(walls[i], wallFrames[i], a, z)}, false)
					}
				}
				a, z := points[i], points[n+i]
				previous := (i + n - 1) % n
				vertical[i] = edge(vertices[i], vertices[n+i], line(a, z), []int64{wallVerticalPC(previous, true, a, z), wallVerticalPC(i, false, a, z)}, false)
			}
			faces := []int64{}
			for i := range b.Outer {
				next := (i + 1) % n
				loop := []int64{use(ring[0][i], true), use(vertical[next], true), use(ring[1][i], false), use(vertical[i], false)}
				faces = append(faces, face(walls[i], []int64{bound(loop, true)}, true))
			}
			capBounds := [2][]int64{}
			for j := 0; b.disk == nil && j < 2; j++ {
				uses := []int64{}
				for i := 0; i < n; i++ {
					k := i
					if j == 0 {
						k = n - 1 - i
					}
					uses = append(uses, use(ring[j][k], j == 1))
				}
				capBounds[j] = []int64{bound(uses, true)}
			}
			type cavity struct {
				circle    trimmed.ExtrusionCircle
				low, high float64
				entry     int
				outer     bool
			}
			cavities := make([]cavity, 0, 1+len(b.Holes)+len(b.Blind))
			if b.disk != nil {
				cavities = append(cavities, cavity{circle: *b.disk, low: lo, high: hi, entry: -1, outer: true})
			}

			for _, h := range b.Holes {
				cavities = append(cavities, cavity{circle: h, low: lo, high: hi, entry: -1})
			}
			for _, h := range b.Blind {
				start, depth, entry := 0., h.Depth, 0
				if b.Height < 0 {
					depth = -depth
					entry = 1
				}
				if h.OppositeCap {
					start, depth, entry = b.Height, b.Height-depth, entry^1
				}
				cavities = append(cavities, cavity{circle: h.ExtrusionCircle, low: math.Min(start, depth), high: math.Max(start, depth), entry: entry})
			}
			for _, hole := range cavities {
				h := hole.circle
				low, high := hole.low, hole.high
				owners := caps
				if hole.entry >= 0 {
					floorZ := high
					if hole.entry == 1 {
						floorZ = low
					}
					ff, _ := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{Z: floorZ}), b.Frame.XAxis(), b.Frame.ZAxis())
					owners[1-hole.entry] = emit(fmt.Sprintf("PLANE('',#%d)", place(ff)))
				}
				cf, _ := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y, Z: low}), b.Frame.XAxis(), b.Frame.ZAxis())
				cylinder := emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", place(cf), real(h.Radius)))
				rim := [2]int64{}
				pole := [2]int64{}
				seamPoints := [2]geom.Vec3{}
				for j, z := range []float64{low, high} {
					center := b.Frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y, Z: z})
					f, _ := geom.NewFrame(center, b.Frame.XAxis(), b.Frame.ZAxis())
					q := center.Add(b.Frame.XAxis().Scale(h.Radius))
					seamPoints[j] = q
					pt := point(q)
					pole[j] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", pt))
					circle := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", place(f), real(h.Radius)))
					rim[j] = edge(pole[j], pole[j], circle, []int64{pc(cylinder, line2(geom.Vec2{Y: float64(j) * (high - low)}, geom.Vec2{X: 1})), pc(owners[j], circle2(h.Center, h.Radius))}, false)
					if hole.outer {
						capBounds[j] = append(capBounds[j], bound([]int64{use(rim[j], j == 1)}, true))
					} else if hole.entry < 0 || j == hole.entry {
						capBounds[j] = append(capBounds[j], bound([]int64{use(rim[j], j == 0)}, false))
					} else {
						faces = append(faces, face(owners[j], []int64{bound([]int64{use(rim[j], j == 0)}, true)}, j == 0))
					}
				}
				seam := edge(pole[0], pole[1], line(seamPoints[0], seamPoints[1]), []int64{pc(cylinder, line2(geom.Vec2{}, geom.Vec2{Y: 1})), pc(cylinder, line2(geom.Vec2{X: 2 * math.Pi}, geom.Vec2{Y: 1}))}, true)
				var loop []int64
				if hole.outer {
					loop = []int64{use(rim[0], true), use(seam, true), use(rim[1], false), use(seam, false)}
				} else {
					loop = []int64{use(seam, false), use(rim[0], false), use(seam, true), use(rim[1], true)}
				}
				faces = append(faces, face(cylinder, []int64{bound(loop, true)}, hole.outer))
			}
			for j := 0; j < 2; j++ {
				faces = append(faces, face(caps[j], capBounds[j], j == 1))
			}
			shell := emit("CLOSED_SHELL(''," + references(faces) + ")")
			roots = append(roots, emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(b.Name), shell)))
		}
		return roots
	})
}
