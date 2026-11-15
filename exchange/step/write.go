package step

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"plainsolid/geom"
)

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch {
		case r == '\'':
			b.WriteString("''")
		case r == '\\':
			b.WriteString("\\\\")
		case r >= 32 && r <= 126:
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "\\X4\\%08X\\X0\\", r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}
func real(x float64) string {
	s := strconv.FormatFloat(x, 'g', 17, 64)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		a := s[:i]
		if !strings.Contains(a, ".") {
			a += "."
		}
		return a + "E" + s[i+1:]
	}
	if !strings.Contains(s, ".") {
		s += "."
	}
	return s
}
func references(ids []int64) string {
	var b strings.Builder
	b.WriteByte('(')
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "#%d", id)
	}
	b.WriteByte(')')
	return b.String()
}

// Write emits an AP203-style CONFIG_CONTROL_DESIGN Part 21 file containing
// faceted B-reps, explicit SI units, representation contexts and product links.
// It supports one connected outward shell per body, planar face holes, and
// metre-based SI or explicit conversion units. Coordinates are not rescaled. A fixed
// timestamp makes output deterministic. This is a narrow entity subset, not a
// claim of complete AP203 conformance or independently qualified CAD exchange.
func Write(w io.Writer, m Model) error { return writeModel(w, m, false) }

// WriteAdvanced emits planar ADVANCED_FACE bodies with shared linear EDGE_CURVE
// topology. Units, validation and metadata follow Write; curved geometry is not
// introduced or approximated. Holes retain their oriented boundary loops.
func WriteAdvanced(w io.Writer, m Model) error { return writeModel(w, m, true) }

func writeModel(w io.Writer, m Model, advanced bool) error {
	return writePlanarModel(w, m, advanced, false)
}

func writePlanarModel(w io.Writer, m Model, advanced, exactSupports bool) error {
	if w == nil {
		return errors.New("step: nil writer")
	}
	if len(m.Bodies) == 0 {
		return errors.New("step: empty model")
	}
	if !validLengthUnit(m.LengthUnit) {
		return errors.New("step: invalid length unit")
	}
	for _, body := range m.Bodies {
		if !utf8.ValidString(body.Name) {
			return errors.New("step: invalid UTF-8 body name")
		}
		if e := body.Solid.Validate(); e != nil {
			return e
		}
		volume := body.Solid.Tessellate().SignedVolume()
		if body.Solid.Topology().Shells != 1 || volume <= 0 || math.IsNaN(volume) || math.IsInf(volume, 0) {
			return errors.New("step: each body must have one outward connected shell")
		}
	}
	pointNumber := real
	if exactSupports {
		pointNumber = exactVertexContactPointText
	}
	floatDirection := func(d geom.Vec3) string {
		return real(d.X) + "," + real(d.Y) + "," + real(d.Z)
	}
	b := bufio.NewWriter(w)
	var writeErr error
	line := func(format string, args ...any) {
		if writeErr == nil {
			_, writeErr = fmt.Fprintf(b, format+"\n", args...)
		}
	}
	next := int64(1)
	emit := func(s string) int64 { id := next; next++; line("#%d=%s;", id, s); return id }
	line("ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('PlainSolid faceted B-rep'),'2;1');\nFILE_NAME('model.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));\nENDSEC;\nDATA;")
	length := emitLengthUnit(emit, m.LengthUnit)
	angle := emit("(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.))")
	solidAngle := emit("(NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT())")
	uncertainty := emit(fmt.Sprintf("UNCERTAINTY_MEASURE_WITH_UNIT(LENGTH_MEASURE(1.E-7),#%d,'distance_accuracy_value','')", length))
	context := emit(fmt.Sprintf("(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT((#%d)) GLOBAL_UNIT_ASSIGNED_CONTEXT((#%d,#%d,#%d)) REPRESENTATION_CONTEXT('',''))", uncertainty, length, angle, solidAngle))
	app := emit("APPLICATION_CONTEXT('configuration controlled 3d designs of mechanical parts and assemblies')")
	emit(fmt.Sprintf("APPLICATION_PROTOCOL_DEFINITION('international standard','config_control_design',1994,#%d)", app))
	productContext := emit(fmt.Sprintf("MECHANICAL_CONTEXT('',#%d,'mechanical')", app))
	definitionContext := emit(fmt.Sprintf("DESIGN_CONTEXT('',#%d,'design')", app))
	for bodyIndex, body := range m.Bodies {
		vertices, edges, faces := body.Solid.Vertices(), body.Solid.Edges(), body.Solid.Faces()
		pointIDs := make([]int64, len(vertices))
		for i, p := range vertices {
			pointIDs[i] = emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", pointNumber(p.X), pointNumber(p.Y), pointNumber(p.Z)))
		}
		edgeIDs := make([]int64, len(edges))
		if advanced {
			vertexIDs := make([]int64, len(vertices))
			for i, id := range pointIDs {
				vertexIDs[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", id))
			}
			for i, e := range edges {
				a, b := vertices[e.Start], vertices[e.End]
				var ratios string
				var err error
				if exactSupports {
					ratios, err = vertexContactDirectionText(vertexContactDifference(mappedRatVec(b), mappedRatVec(a)))
				} else {
					var d geom.Vec3
					d, err = b.Sub(a).Unit()
					ratios = floatDirection(d)
				}
				if err != nil {
					return err
				}
				direction := emit(fmt.Sprintf("DIRECTION('',(%s))", ratios))
				vector := emit(fmt.Sprintf("VECTOR('',#%d,1.)", direction))
				lineID := emit(fmt.Sprintf("LINE('',#%d,#%d)", pointIDs[e.Start], vector))
				edgeIDs[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertexIDs[e.Start], vertexIDs[e.End], lineID))
			}
		}
		faceIDs := make([]int64, len(faces))
		for i, face := range faces {
			boundIDs := []int64{}
			// Entity identity comes from oriented edge starts, not tessellated triangles.
			loopIDs := make([][]int64, 1+len(face.Holes))
			outer := make([]int64, len(face.Boundary))
			for j, u := range face.Boundary {
				e := edges[u.Edge]
				v := e.Start
				if u.Reversed {
					v = e.End
				}
				outer[j] = pointIDs[v]
			}
			loopIDs[0] = outer
			for h, uses := range face.Holes {
				ids := make([]int64, len(uses))
				for j, u := range uses {
					e := edges[u.Edge]
					v := e.Start
					if u.Reversed {
						v = e.End
					}
					ids[j] = pointIDs[v]
				}
				loopIDs[h+1] = ids
			}
			for j, ids := range loopIDs {
				var loop int64
				if advanced {
					uses := face.Boundary
					if j > 0 {
						uses = face.Holes[j-1]
					}
					oriented := make([]int64, len(uses))
					for k, u := range uses {
						sense := ".T."
						if u.Reversed {
							sense = ".F."
						}
						oriented[k] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edgeIDs[u.Edge], sense))
					}
					loop = emit("EDGE_LOOP(''," + references(oriented) + ")")
				} else {
					loop = emit("POLY_LOOP(''," + references(ids) + ")")
				}
				kind := "FACE_BOUND"
				if j == 0 {
					kind = "FACE_OUTER_BOUND"
				}
				boundIDs = append(boundIDs, emit(fmt.Sprintf("%s('',#%d,.T.)", kind, loop)))
			}
			firstUse := face.Boundary[0]
			firstEdge := edges[firstUse.Edge]
			first, second := firstEdge.Start, firstEdge.End
			if firstUse.Reversed {
				first, second = second, first
			}
			var normalRatios, referenceRatios string
			if exactSupports {
				var err error
				normalRatios, err = vertexContactDirectionText(vertexContactOrientedNormal(vertices, edges, face.Boundary))
				if err != nil {
					return err
				}
				referenceRatios, err = vertexContactDirectionText(vertexContactDifference(mappedRatVec(vertices[second]), mappedRatVec(vertices[first])))
				if err != nil {
					return err
				}
			} else {
				plane, err := body.Solid.FacePlane(i)
				if err != nil {
					return err
				}
				normal := plane.Normal()
				direction := vertices[second].Sub(vertices[first])
				xaxis, err := direction.Sub(normal.Scale(direction.Dot(normal))).Unit()
				if err != nil {
					return err
				}
				normalRatios, referenceRatios = floatDirection(normal), floatDirection(xaxis)
			}
			axisID := emit(fmt.Sprintf("DIRECTION('',(%s))", normalRatios))
			refID := emit(fmt.Sprintf("DIRECTION('',(%s))", referenceRatios))
			placement := emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", pointIDs[first], axisID, refID))
			surface := emit(fmt.Sprintf("PLANE('',#%d)", placement))
			faceKind := "FACE_SURFACE"
			if advanced {
				faceKind = "ADVANCED_FACE"
			}
			faceIDs[i] = emit(fmt.Sprintf("%s('',%s,#%d,.T.)", faceKind, references(boundIDs), surface))
		}
		shell := emit("CLOSED_SHELL(''," + references(faceIDs) + ")")
		bodyKind, repKind := "FACETED_BREP", "FACETED_BREP_SHAPE_REPRESENTATION"
		if advanced {
			bodyKind, repKind = "MANIFOLD_SOLID_BREP", "ADVANCED_BREP_SHAPE_REPRESENTATION"
		}
		bodyID := emit(fmt.Sprintf("%s(%s,#%d)", bodyKind, quote(body.Name), shell))
		rep := emit(fmt.Sprintf("%s(%s,(#%d),#%d)", repKind, quote(body.Name), bodyID, context))
		product := emit(fmt.Sprintf("PRODUCT('%d',%s,'',(#%d))", bodyIndex+1, quote(body.Name), productContext))
		formation := emit(fmt.Sprintf("PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE('','',#%d,.NOT_KNOWN.)", product))
		definition := emit(fmt.Sprintf("PRODUCT_DEFINITION('design','',#%d,#%d)", formation, definitionContext))
		shape := emit(fmt.Sprintf("PRODUCT_DEFINITION_SHAPE('','',#%d)", definition))
		emit(fmt.Sprintf("SHAPE_DEFINITION_REPRESENTATION(#%d,#%d)", shape, rep))
	}
	line("ENDSEC;\nEND-ISO-10303-21;")
	if writeErr != nil {
		return writeErr
	}
	return b.Flush()
}
