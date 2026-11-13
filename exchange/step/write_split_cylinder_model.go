package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// SplitCylinderDescriptor specifies the exact family accepted by
// ReadSplitCylinderModel. Source entity IDs and stale native solids are not
// writer inputs: the complete directed graph is constructed from this geometry.
type SplitCylinderDescriptor struct {
	Name                       string
	Frame                      geom.Frame
	Radius, Height, LengthUnit float64
}

// WriteSplitCylinderModel stages and verifies a complete model-only BRep before
// output. Every generated point and decimal must preserve the exact represented
// inputs. Unsupported frames, nonrepresentable sums and units fail atomically.
// The bare geometric representation has no application/product root.
func WriteSplitCylinderModel(w io.Writer, d SplitCylinderDescriptor) error {
	if w == nil || !utf8.ValidString(d.Name) || len(d.Name) > 1024 || !d.Frame.Origin().IsFinite() || !cylinderSupportUnit(d.Frame.XAxis()) || !cylinderSupportUnit(d.Frame.ZAxis()) || d.Frame.XAxis().Dot(d.Frame.ZAxis()) != 0 || !finiteNumber(d.Radius) || !finiteNumber(d.Height) || d.Radius <= 0 || d.Height <= 0 || !validLengthUnit(d.LengthUnit) {
		return splitFailure()
	}
	var b bytes.Buffer
	b.WriteString("ISO-10303-21;\nHEADER;FILE_DESCRIPTION(('PlainSolid exact split cylinder'),'2;1');FILE_NAME('','',(''),(''),'PlainSolid','','');FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));ENDSEC;\nDATA;\n")
	next := int64(0)
	emit := func(s string) int64 { next++; fmt.Fprintf(&b, "#%d=%s;\n", next, s); return next }
	var failure error
	number := func(q *big.Rat) string {
		if q == nil {
			failure = splitFailure()
			return "0."
		}
		if _, exact := q.Float64(); !exact {
			failure = splitFailure()
		}
		s := q.FloatString(q.Denom().BitLen() - 1)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
		if _, e := cylinderSupportNumber(Value{Kind: Number, Text: s}); e != nil {
			failure = e
		}
		if !strings.Contains(s, ".") {
			s += "."
		}
		return s
	}
	realExact := func(v float64) string { return number(new(big.Rat).SetFloat64(v)) }
	direction := func(v geom.Vec3) int64 {
		return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", realExact(v.X), realExact(v.Y), realExact(v.Z)))
	}
	point := func(x, z float64) int64 {
		origin := d.Frame.Origin()
		xa, za := d.Frame.XAxis(), d.Frame.ZAxis()
		out := []string{}
		for i, o := range []float64{origin.X, origin.Y, origin.Z} {
			q := new(big.Rat).SetFloat64(o)
			for j, a := range []float64{[]float64{xa.X, xa.Y, xa.Z}[i], []float64{za.X, za.Y, za.Z}[i]} {
				q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64([]float64{x, z}[j])))
			}
			out = append(out, number(q))
		}
		return emit("CARTESIAN_POINT('',(" + strings.Join(out, ",") + "))")
	}
	axis, reference := direction(d.Frame.ZAxis()), direction(d.Frame.XAxis())
	place := func(h float64) int64 {
		return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", point(0, h), axis, reference))
	}
	bottom, top := place(0), place(d.Height)
	cylinder := emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", bottom, realExact(d.Radius)))
	planes := [2]int64{}
	circles := [2]int64{}
	for i, p := range []int64{bottom, top} {
		planes[i] = emit(fmt.Sprintf("PLANE('',#%d)", p))
		circles[i] = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", p, realExact(d.Radius)))
	}
	points := [4]int64{point(d.Radius, 0), point(-d.Radius, 0), point(d.Radius, d.Height), point(-d.Radius, d.Height)}
	vertices := [4]int64{}
	for i, p := range points {
		vertices[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", p))
	}
	vec := emit(fmt.Sprintf("VECTOR('',#%d,1.)", axis))
	lines := [2]int64{}
	for i, p := range points[:2] {
		lines[i] = emit(fmt.Sprintf("LINE('',#%d,#%d)", p, vec))
	}
	edges := make([]int64, 6)
	for i, a := range [][3]int64{{vertices[0], vertices[1], circles[0]}, {vertices[1], vertices[0], circles[0]}, {vertices[2], vertices[3], circles[1]}, {vertices[3], vertices[2], circles[1]}, {vertices[0], vertices[2], lines[0]}, {vertices[1], vertices[3], lines[1]}} {
		edges[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", a[0], a[1], a[2]))
	}
	faces := []int64{}
	for i, seq := range [][]int{{1, 6, -3, -5}, {2, 5, -4, -6}, {-2, -1}, {3, 4}} {
		uses := []int64{}
		for _, v := range seq {
			sense := "T"
			if v < 0 {
				v = -v
				sense = "F"
			}
			uses = append(uses, emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,.%s.)", edges[v-1], sense)))
		}
		loop := emit("EDGE_LOOP(''," + references(uses) + ")")
		bound := emit(fmt.Sprintf("FACE_OUTER_BOUND('',#%d,.T.)", loop))
		support := cylinder
		sense := "T"
		if i >= 2 {
			support = planes[i-2]
		}
		if i == 2 {
			sense = "F"
		}
		faces = append(faces, emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,.%s.)", bound, support, sense)))
	}
	shell := emit("CLOSED_SHELL(''," + references(faces) + ")")
	body := emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(d.Name), shell))
	length := emitLengthUnit(emit, d.LengthUnit)
	angle := emit("(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.))")
	solid := emit("(NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT())")
	ctx := emit(fmt.Sprintf("(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#%d,#%d,#%d)) REPRESENTATION_CONTEXT('',''))", length, angle, solid))
	emit(fmt.Sprintf("ADVANCED_BREP_SHAPE_REPRESENTATION('',(#%d),#%d)", body, ctx))
	b.WriteString("ENDSEC;END-ISO-10303-21;\n")
	if failure != nil {
		return failure
	}
	if _, e := ReadSplitCylinderModel(bytes.NewReader(b.Bytes()), Limits{MaxBytes: 1 << 16, MaxEntities: 256, MaxValues: 4096, MaxDepth: 16}); e != nil {
		return e
	}
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		e = io.ErrShortWrite
	}
	return e
}
