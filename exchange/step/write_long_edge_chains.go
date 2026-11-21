package step

import (
	"context"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// LongEdgeTreatmentBody is the native horizontal fillet/chamfer stock recipe.
// Geometry is authored at [0,Length]×[0,Width]×[0,Height].
type LongEdgeTreatmentBody struct {
	Name                                                   string
	Length, Width, Height, Radius, TopSetback, SideSetback float64
}
type LongEdgeTreatmentModel struct {
	Bodies     []LongEdgeTreatmentBody
	LengthUnit float64
}

// LongEdgeChainBody adds the native Z-through bore and rectangular pocket and
// upper rectangular boss. Footprints must occupy the retained flat top strip.
type LongEdgeChainBody struct {
	LongEdgeTreatmentBody
	Bore   trimmed.ExtrusionCircle
	Pocket trimmed.ExtrusionRectangle
	Boss   trimmed.ExtrusionBoss
}
type LongEdgeChainModel struct {
	Bodies     []LongEdgeChainBody
	LengthUnit float64
}

func WriteLongEdgeTreatments(w io.Writer, m LongEdgeTreatmentModel) error {
	if w == nil || len(m.Bodies) != 1 {
		return geom.ErrInvalid
	}
	b := m.Bodies[0]
	r, e := fillet.TreatOppositeLongCapEdges(context.Background(), b.Length, b.Width, b.Height, b.Radius, b.TopSetback, b.SideSetback, fillet.Options{})
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopology(w, m.LengthUnit, b.Name, r.Solid)
}

// WriteLongEdgeChains exports the actual retained native face/edge graph,
// including its bore, through-pocket and boss. The recipe is reconstructed and
// validated by the native chain constructor; arbitrary caller solids, unknown
// trims and guessed geometry are not accepted by this narrow adapter.
func WriteLongEdgeChains(w io.Writer, m LongEdgeChainModel) error {
	if w == nil || len(m.Bodies) != 1 {
		return geom.ErrInvalid
	}
	b := m.Bodies[0]
	r, e := fillet.TreatLongCapEdgesWithFeatures(context.Background(), b.Length, b.Width, b.Height, b.Radius, b.TopSetback, b.SideSetback, b.Bore, b.Pocket, b.Boss, fillet.Options{})
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopology(w, m.LengthUnit, b.Name, r.Solid)
}

type longEdgeSurface struct {
	kind   string
	frame  geom.Frame
	radius float64
}
type longEdgeOwner struct {
	face int
	use  trimmed.Use
}
type longEdgeCurve struct {
	same              bool
	circle            bool
	ellipse           bool
	minorRadius       float64
	frame             geom.Frame
	radius            float64
	origin, direction geom.Vec3
}

func longEdgeSurfaceOf(surface geom.Surface) (longEdgeSurface, error) {
	base, m := parametric.SurfaceMapping(surface)
	for i := 0; i < 3; i++ {
		count := 0
		for j := 0; j < 3; j++ {
			v := m[i][j]
			if v != 0 {
				if v != 1 && v != -1 {
					return longEdgeSurface{}, geom.ErrDomain
				}
				count++
			}
		}
		if count != 1 || math.IsNaN(m[i][3]) || math.IsInf(m[i][3], 0) {
			return longEdgeSurface{}, geom.ErrDomain
		}
	}
	if m.Determinant() != 1 {
		return longEdgeSurface{}, geom.ErrDomain
	}
	var out longEdgeSurface
	var frame geom.Frame
	switch s := base.(type) {
	case geom.PlaneSurface:
		out.kind = "PLANE"
		frame = s.Frame()
	case geom.CylinderSurface:
		out.kind = "CYLINDRICAL_SURFACE"
		frame, out.radius = s.Parameters()
	default:
		return out, geom.ErrDomain
	}
	var e error
	out.frame, e = geom.NewFrame(m.Apply(frame.Origin()), m.ApplyVector(frame.XAxis()), m.ApplyVector(frame.ZAxis()))
	return out, e
}

// The caller supplies only topology freshly produced by the two validated
// native constructors above; this is not a generic Solid serializer.
type nativeLongEdgeLimits struct{ faces, edges, vertices, loops, uses int }

func writeNativeLongEdgeTopology(w io.Writer, unit float64, name string, s trimmed.Solid) error {
	return writeNativeLongEdgeTopologyBounded(w, unit, name, s, nativeLongEdgeLimits{64, 256, 256, 5, 64})
}
func writeNativeLongEdgeTopologyBounded(w io.Writer, unit float64, name string, s trimmed.Solid, limits nativeLongEdgeLimits) error {
	return writeNativeLongEdgeTopologyWithDirection(w, unit, name, s, limits, nil)
}

func writeNativeLongEdgeTopologyWithDirection(w io.Writer, unit float64, name string, s trimmed.Solid, limits nativeLongEdgeLimits, obliqueDirection *geom.Vec3) error {
	return writeNativeLongEdgeTopologyConics(w, unit, name, s, limits, obliqueDirection, false)
}

func writeNativeLongEdgeTopologyConics(w io.Writer, unit float64, name string, s trimmed.Solid, limits nativeLongEdgeLimits, obliqueDirection *geom.Vec3, crossedOblique bool) error {
	return writeNativeMixedDisjointTopology(w, unit, name, s, limits, obliqueDirection, crossedOblique, nil)
}

func writeNativeMixedDisjointTopology(w io.Writer, unit float64, name string, s trimmed.Solid, limits nativeLongEdgeLimits, obliqueDirection *geom.Vec3, crossedOblique bool, directions map[int]geom.Vec3) error {
	if !utf8.ValidString(name) || len(name) > 4096 {
		return geom.ErrInvalid
	}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if len(faces) < 3 || len(faces) > limits.faces || len(edges) > limits.edges || len(vertices) > limits.vertices {
		return geom.ErrDomain
	}
	surfaces := make([]longEdgeSurface, len(faces))
	owners := make([][]longEdgeOwner, len(edges))
	curves := make([]longEdgeCurve, len(edges))
	for i, face := range faces {
		var e error
		surfaces[i], e = longEdgeSurfaceOf(face.Surface)
		if e != nil {
			return e
		}
		loops := append([][]trimmed.Use{face.Outer}, face.Holes...)
		if len(loops) > limits.loops {
			return geom.ErrDomain
		}
		for _, loop := range loops {
			if len(loop) < 3 || len(loop) > limits.uses {
				return geom.ErrDomain
			}
			for j, u := range loop {
				if u.Edge < 0 || u.Edge >= len(edges) {
					return geom.ErrInvalid
				}
				edge := edges[u.Edge]
				if edge.Start < 0 || edge.End < 0 || edge.Start >= len(vertices) || edge.End >= len(vertices) || edge.Start == edge.End {
					return geom.ErrDomain
				}
				next := loop[(j+1)%len(loop)]
				if next.Edge < 0 || next.Edge >= len(edges) {
					return geom.ErrInvalid
				}
				end := edge.End
				if u.Reversed {
					end = edge.Start
				}
				start := edges[next.Edge].Start
				if next.Reversed {
					start = edges[next.Edge].End
				}
				if start != end {
					return geom.ErrDomain
				}
				owners[u.Edge] = append(owners[u.Edge], longEdgeOwner{i, u})
			}
		}
	}
	for i, edge := range edges {
		os := owners[i]
		if len(os) != 2 || os[0].face == os[1].face || (os[0].use.Reversed != faces[os[0].face].Reversed) == (os[1].use.Reversed != faces[os[1].face].Reversed) {
			return geom.ErrDomain
		}
		a, b := vertices[edge.Start], vertices[edge.End]
		if crossedOblique {
			var conic longEdgeCurve
			var err error
			curved := false
			if os[0].use.Curve != nil && os[1].use.Curve != nil {
				first, second := surfaces[os[0].face], surfaces[os[1].face]
				if first.kind == "CYLINDRICAL_SURFACE" && second.kind == "CYLINDRICAL_SURFACE" {
					conic, err = crossedObliqueBoreEdge(a, b, os, surfaces)
					curved = true
				} else if first.kind != second.kind && first.frame.ZAxis().Cross(second.frame.ZAxis()) != (geom.Vec3{}) {
					conic, err = obliqueBoreEdgeCurve(a, b, os, surfaces)
					curved = true
				}
			}
			if err != nil {
				return err
			}
			if curved {
				curves[i] = conic
				continue
			}
		}
		if (obliqueDirection != nil || directions != nil) && os[0].use.Curve != nil && os[1].use.Curve != nil && surfaces[os[0].face].kind != surfaces[os[1].face].kind {
			curve, err := obliqueBoreEdgeCurve(a, b, os, surfaces)
			if err != nil {
				return err
			}
			curves[i] = curve
			continue
		}
		direction, e := b.Sub(a).Unit()
		if e != nil {
			return e
		}
		curves[i] = longEdgeCurve{origin: a, direction: direction, same: true}
		for _, owner := range os {
			surface := surfaces[owner.face]
			u := owner.use
			if surface.kind != "CYLINDRICAL_SURFACE" {
				continue
			}
			if u.Curve != nil {
				return geom.ErrDomain
			}
			if u.UVStart.Y == u.UVEnd.Y && u.UVStart.X != u.UVEnd.X {
				delta := u.UVEnd.X - u.UVStart.X
				if u.Reversed {
					delta = -delta
				}
				if math.Abs(delta) > math.Pi+1e-10 {
					return geom.ErrDomain
				}
				axis := surface.frame.ZAxis()
				if delta < 0 {
					axis = axis.Scale(-1)
				}
				frame, e := geom.NewFrame(surface.frame.ToWorld(geom.Vec3{Z: u.UVStart.Y}), surface.frame.XAxis(), axis)
				if e != nil {
					return e
				}
				curves[i] = longEdgeCurve{circle: true, frame: frame, radius: surface.radius, same: true}
				break
			} else if u.UVStart.X != u.UVEnd.X {
				return geom.ErrDomain
			}
		}
	}

	for i, os := range owners {
		if !curves[i].circle {
			continue
		}
		for _, owner := range os {
			surface := surfaces[owner.face]
			if surface.kind != "PLANE" {
				continue
			}
			if curves[i].frame.ZAxis().Dot(surface.frame.ZAxis()) < 0 {
				frame, e := geom.NewFrame(curves[i].frame.Origin(), curves[i].frame.XAxis(), curves[i].frame.ZAxis().Scale(-1))
				if e != nil {
					return e
				}
				curves[i].frame = frame
				curves[i].same = false
			}
			break
		}
	}
	// Preflight every owner representation before any bytes are written.
	for i, os := range owners {
		curve := curves[i]
		if curve.ellipse {
			continue
		}
		for _, owner := range os {
			surface := surfaces[owner.face]
			u := owner.use
			if surface.kind == "PLANE" {
				if curve.circle {
					if curve.frame.ZAxis().Distance(surface.frame.ZAxis()) > 1e-10 {
						return geom.ErrDomain
					}
				} else if u.Curve != nil {
					return geom.ErrDomain
				}
			} else if curve.circle {
				if u.Curve != nil || u.UVStart.Y != u.UVEnd.Y {
					return geom.ErrDomain
				}
				if math.Abs(math.Abs(curve.frame.ZAxis().Dot(surface.frame.ZAxis()))-1) > 1e-10 {
					return geom.ErrDomain
				}
			} else if u.Curve != nil || u.UVStart.X != u.UVEnd.X {
				return geom.ErrDomain
			}
		}
	}
	return writeStandaloneWithUncertainty(w, unit, 0, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point3 := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		dir3 := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
		}
		placeDirection := func(f geom.Frame, normal geom.Vec3) int64 {
			p, z, x := point3(f.Origin()), dir3(normal), dir3(f.XAxis())
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", p, z, x))
		}
		place := func(f geom.Frame) int64 { return placeDirection(f, f.ZAxis()) }
		point2 := func(p geom.Vec2) int64 { return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(p.X), real(p.Y))) }
		line2 := func(p, d geom.Vec2) int64 {
			o := point2(p)
			v := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(d.X), real(d.Y)))
			v = emit(fmt.Sprintf("VECTOR('',#%d,%s)", v, real(math.Hypot(d.X, d.Y))))
			return emit(fmt.Sprintf("LINE('',#%d,#%d)", o, v))
		}
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		surfaceIDs := make([]int64, len(faces))
		for i, surface := range surfaces {
			normal := surface.frame.ZAxis()
			if obliqueDirection != nil && surface.kind == "CYLINDRICAL_SURFACE" {
				normal = *obliqueDirection
			}
			if d, ok := directions[i]; ok {
				normal = d
			}
			p := placeDirection(surface.frame, normal)
			if surface.kind == "PLANE" {
				surfaceIDs[i] = emit(fmt.Sprintf("PLANE('',#%d)", p))
			} else {
				surfaceIDs[i] = emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", p, real(surface.radius)))
			}
		}
		vertexIDs := make([]int64, len(vertices))
		for i, p := range vertices {
			pid := point3(p)
			vertexIDs[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", pid))
		}
		edgeIDs := make([]int64, len(edges))
		for i, edge := range edges {
			curve := curves[i]
			var model int64
			if curve.ellipse {
				p := place(curve.frame)
				model = emit(fmt.Sprintf("ELLIPSE('',#%d,%s,%s)", p, real(curve.radius), real(curve.minorRadius)))
				sense := ".T."
				if !curve.same {
					sense = ".F."
				}
				edgeIDs[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,%s)", vertexIDs[edge.Start], vertexIDs[edge.End], model, sense))
				continue
			}
			if curve.circle {
				p := place(curve.frame)
				model = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", p, real(curve.radius)))
			} else {
				p, d := point3(curve.origin), dir3(curve.direction)
				v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
				model = emit(fmt.Sprintf("LINE('',#%d,#%d)", p, v))
			}
			pcs := [2]int64{}
			for j, owner := range owners[i] {
				surface := surfaces[owner.face]
				u := owner.use
				var geometry int64
				if surface.kind == "PLANE" {
					if curve.circle {
						p := surface.frame.ToLocal(curve.frame.Origin())
						x := curve.frame.XAxis()
						d := geom.Vec2{X: x.Dot(surface.frame.XAxis()), Y: x.Dot(surface.frame.YAxis())}
						pid := point2(geom.Vec2{X: p.X, Y: p.Y})
						dir := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(d.X), real(d.Y)))
						placement := emit(fmt.Sprintf("AXIS2_PLACEMENT_2D('',#%d,#%d)", pid, dir))
						geometry = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", placement, real(curve.radius)))
					} else {
						p := surface.frame.ToLocal(curve.origin)
						d := curve.direction
						geometry = line2(geom.Vec2{X: p.X, Y: p.Y}, geom.Vec2{X: d.Dot(surface.frame.XAxis()), Y: d.Dot(surface.frame.YAxis())})
					}
				} else if curve.circle {
					phase := math.Atan2(curve.frame.XAxis().Dot(surface.frame.YAxis()), curve.frame.XAxis().Dot(surface.frame.XAxis()))
					sense := curve.frame.ZAxis().Dot(surface.frame.ZAxis())
					geometry = line2(geom.Vec2{X: phase, Y: u.UVStart.Y}, geom.Vec2{X: sense})
				} else {
					p := u.UVStart
					if u.Reversed {
						p = u.UVEnd
					}
					geometry = line2(p, geom.Vec2{Y: curve.direction.Dot(surface.frame.ZAxis())})
				}
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", geometry, ctx))
				pcs[j] = emit(fmt.Sprintf("PCURVE('',#%d,#%d)", surfaceIDs[owner.face], rep))
			}
			sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,(#%d,#%d),.CURVE_3D.)", model, pcs[0], pcs[1]))
			sense := ".T."
			if !curve.same {
				sense = ".F."
			}
			edgeIDs[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,%s)", vertexIDs[edge.Start], vertexIDs[edge.End], sc, sense))
		}
		boolText := func(b bool) string {
			if b {
				return ".T."
			}
			return ".F."
		}
		refs := func(ids []int64) string {
			out := make([]string, len(ids))
			for i, id := range ids {
				out[i] = fmt.Sprintf("#%d", id)
			}
			return strings.Join(out, ",")
		}
		faceIDs := make([]int64, len(faces))
		for i, face := range faces {
			loops := append([][]trimmed.Use{face.Outer}, face.Holes...)
			bounds := make([]int64, len(loops))
			for j, loop := range loops {
				uses := make([]int64, len(loop))
				for k, u := range loop {
					uses[k] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edgeIDs[u.Edge], boolText(!u.Reversed)))
				}
				lid := emit(fmt.Sprintf("EDGE_LOOP('',(%s))", refs(uses)))
				kind := "FACE_BOUND"
				if j == 0 {
					kind = "FACE_OUTER_BOUND"
				}
				bounds[j] = emit(fmt.Sprintf("%s('',#%d,%s)", kind, lid, boolText(!face.Reversed)))
			}
			faceIDs[i] = emit(fmt.Sprintf("ADVANCED_FACE('',(%s),#%d,%s)", refs(bounds), surfaceIDs[i], boolText(!face.Reversed)))
		}
		shell := emit(fmt.Sprintf("CLOSED_SHELL('',(%s))", refs(faceIDs)))
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(name), shell))}
	})
}
