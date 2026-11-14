package step

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// EllipticalAnnulusFace is one planar face with a concentric aligned elliptical
// hole. Its embedded Face owns outer local edge0 and reversed hole local edge1.
// It is not a material solid; the original independent edge roles are retained.
type EllipticalAnnulusFace struct {
	EllipticalDiskFace
	InnerBoundary                                                                                            geom.Ellipse
	InnerVertex                                                                                              geom.Vec3
	InnerMajorRadius, InnerMinorRadius                                                                       float64
	SourceInnerBoundID, SourceInnerEdgeID, SourceInnerVertexID, SourceInnerPCurveID, SourceInnerModelCurveID int64
}

func ellipticalAnnulusFailure() error {
	return fmt.Errorf("step: unsupported exact concentric elliptical annulus face")
}

// ReadEllipticalAnnulusFace proves the complete original OCCT66-record graph,
// including opposite FACE_BOUND senses, independent model/pcurve pairs and
// strictly contained aligned ellipses. All geometric coefficients are exact.
func ReadEllipticalAnnulusFace(r io.Reader, l Limits) (EllipticalAnnulusFace, error) {
	fail := func() (EllipticalAnnulusFace, error) { return EllipticalAnnulusFace{}, ellipticalAnnulusFailure() }
	if r == nil {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 66)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return EllipticalAnnulusFace{}, e
	}
	if len(f.Entities) != 66 {
		return fail()
	}
	roles := map[string]int64{}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		kind := en.Records[0].Type
		switch kind {
		case "SHAPE_DEFINITION_REPRESENTATION", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_RELATED_PRODUCT_CATEGORY", "PLANE", "ADVANCED_FACE":
			if roles[kind] != 0 {
				return fail()
			}
			roles[kind] = id
		}
	}
	faceRecord, e := simple(f, roles["ADVANCED_FACE"], "ADVANCED_FACE", 4)
	if e != nil {
		return fail()
	}
	bounds, e := listRefs(faceRecord.Args[1], 1)
	if e != nil || len(bounds) != 2 {
		return fail()
	}
	model := func(bound int64) (int64, error) {
		br, e := simple(f, bound, "FACE_BOUND", 3)
		if e != nil {
			return 0, e
		}
		lid, e := ref(br.Args[1])
		if e != nil {
			return 0, e
		}
		lr, e := simple(f, lid, "EDGE_LOOP", 2)
		if e != nil {
			return 0, e
		}
		uses, e := listRefs(lr.Args[1], 1)
		if e != nil || len(uses) != 1 {
			return 0, ellipticalAnnulusFailure()
		}
		ur, e := simple(f, uses[0], "ORIENTED_EDGE", 5)
		if e != nil {
			return 0, e
		}
		eid, e := ref(ur.Args[3])
		if e != nil {
			return 0, e
		}
		er, e := simple(f, eid, "EDGE_CURVE", 5)
		if e != nil {
			return 0, e
		}
		sid, e := ref(er.Args[3])
		if e != nil {
			return 0, e
		}
		sr, e := simple(f, sid, "SURFACE_CURVE", 4)
		if e != nil {
			return 0, e
		}
		return ref(sr.Args[1])
	}
	mid, e := model(bounds[0])
	if e != nil {
		return fail()
	}
	innerID, e := model(bounds[1])
	if e != nil || innerID == mid {
		return fail()
	}
	mr, e := simple(f, mid, "ELLIPSE", 4)
	if e != nil {
		return fail()
	}
	mf, e := ref(mr.Args[1])
	if e != nil {
		return fail()
	}
	frame, e := mappedExactFrame(f, mf)
	if e != nil {
		return fail()
	}
	a, e := cylinderSupportNumber(mr.Args[2])
	if e != nil {
		return fail()
	}
	b, e := cylinderSupportNumber(mr.Args[3])
	if e != nil || a <= b || b <= 0 {
		return fail()
	}
	inner, e := simple(f, innerID, "ELLIPSE", 4)
	if e != nil {
		return fail()
	}
	innerFrameID, e := ref(inner.Args[1])
	if e != nil {
		return fail()
	}
	innerFrame, e := mappedExactFrame(f, innerFrameID)
	if e != nil || innerFrame != frame {
		return fail()
	}
	ia, e := cylinderSupportNumber(inner.Args[2])
	if e != nil {
		return fail()
	}
	ib, e := cylinderSupportNumber(inner.Args[3])
	if e != nil || ib <= 0 || ia <= ib || ia >= a || ib >= b {
		return fail()
	}
	var buf bytes.Buffer
	analyticSupportHeader(&buf)
	buf.WriteString(ellipseAnnulusTemplate)
	fmt.Fprintln(&buf, "ENDSEC;END-ISO-10303-21;")
	want, e := Parse(&buf, Limits{})
	if e != nil {
		return fail()
	}
	number := func(x float64) Value {
		q := new(big.Rat).SetFloat64(x)
		s, e := mappedTerminatingDecimal(q)
		if e != nil {
			return Value{}
		}
		return Value{Kind: Number, Text: s}
	}
	setVec := func(id int64, v geom.Vec3) {
		en := want.Entities[id]
		en.Records[0].Args[1].Items = []Value{number(v.X), number(v.Y), number(v.Z)}
		want.Entities[id] = en
	}
	for _, ids := range [][3]int64{{27, 28, 29}, {33, 34, 35}, {51, 52, 53}} {
		setVec(ids[0], frame.Origin())
		setVec(ids[1], frame.ZAxis())
		setVec(ids[2], frame.XAxis())
	}
	for _, id := range []int64{25, 37} {
		en := want.Entities[id]
		en.Records[0].Args[2] = mr.Args[2]
		en.Records[0].Args[3] = mr.Args[3]
		want.Entities[id] = en
	}
	for _, id := range []int64{49, 56} {
		en := want.Entities[id]
		en.Records[0].Args[2] = inner.Args[2]
		en.Records[0].Args[3] = inner.Args[3]
		want.Entities[id] = en
	}
	// Exact +major vertex; failure to represent a required native point is explicit.
	oo := []float64{frame.Origin().X, frame.Origin().Y, frame.Origin().Z}
	xx := []float64{frame.XAxis().X, frame.XAxis().Y, frame.XAxis().Z}
	v := [3]float64{}
	for i := range 3 {
		q := new(big.Rat).Add(new(big.Rat).SetFloat64(oo[i]), new(big.Rat).Mul(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(xx[i])))
		var exact bool
		v[i], exact = q.Float64()
		if !exact || !finiteNumber(v[i]) {
			return fail()
		}
	}
	vertex := geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	setVec(23, vertex)
	for i := range 3 {
		q := new(big.Rat).Add(new(big.Rat).SetFloat64(oo[i]), new(big.Rat).Mul(new(big.Rat).SetFloat64(ia), new(big.Rat).SetFloat64(xx[i])))
		var exact bool
		v[i], exact = q.Float64()
		if !exact || !finiteNumber(v[i]) {
			return fail()
		}
	}
	innerVertex := geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	setVec(47, innerVertex)
	mapping, ok := ellipseFaceGraph(want, f, [][2]int64{{3, roles["SHAPE_DEFINITION_REPRESENTATION"]}, {1, roles["APPLICATION_PROTOCOL_DEFINITION"]}, {66, roles["PRODUCT_RELATED_PRODUCT_CATEGORY"]}})
	if !ok || mapping[25] != mid || mapping[49] != innerID || mapping[31] != roles["PLANE"] || mapping[17] != roles["ADVANCED_FACE"] {
		return fail()
	}
	unit, e := contextUnits(f, mapping[61])
	if e != nil || unit != .001 {
		return fail()
	}
	plane, e := geom.NewPlaneSurface(frame)
	if e != nil {
		return fail()
	}
	boundary, e := geom.NewEllipse(frame, a, b)
	if e != nil {
		return fail()
	}
	pr, e := simple(f, mapping[7], "PRODUCT", 4)
	if e != nil {
		return fail()
	}
	uv := trimmed.EllipseUV{Cos: geom.Vec2{X: a}, Sin: geom.Vec2{Y: b}, To: 2 * math.Pi}
	face := trimmed.Face{Surface: plane, Outer: []trimmed.Use{{Edge: 0, UVStart: geom.Vec2{X: a}, UVEnd: geom.Vec2{X: a}, Curve: uv}}}
	innerCurve, e := geom.NewEllipse(frame, ia, ib)
	if e != nil {
		return fail()
	}
	hole := trimmed.EllipseUV{Cos: geom.Vec2{X: ia}, Sin: geom.Vec2{Y: ib}, From: 2 * math.Pi}
	face.Holes = [][]trimmed.Use{{{Edge: 1, Reversed: true, UVStart: geom.Vec2{X: ia}, UVEnd: geom.Vec2{X: ia}, Curve: hole}}}
	return EllipticalAnnulusFace{EllipticalDiskFace: EllipticalDiskFace{Name: pr.Args[1].Text, Face: face, Boundary: boundary, Vertex: vertex, MajorRadius: a, MinorRadius: b, LengthUnit: unit, Source: f, SourceFaceID: mapping[17], SourceEdgeID: mapping[21], SourceVertexID: mapping[22], SourcePCurveID: mapping[30], SourceModelCurveID: mid, SourcePlaneID: mapping[31]}, InnerBoundary: innerCurve, InnerVertex: innerVertex, InnerMajorRadius: ia, InnerMinorRadius: ib, SourceInnerBoundID: mapping[42], SourceInnerEdgeID: mapping[45], SourceInnerVertexID: mapping[46], SourceInnerPCurveID: mapping[54], SourceInnerModelCurveID: innerID}, nil
}

// WriteEllipticalAnnulusFaceSource re-proves and emits the original graph only.
func WriteEllipticalAnnulusFaceSource(w io.Writer, source *File, l Limits) error {
	if w == nil || !ellipseOwnedSourceBound(source, 66) {
		return ellipticalAnnulusFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadEllipticalAnnulusFace(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

const ellipseAnnulusTemplate = `

#1 = APPLICATION_PROTOCOL_DEFINITION('international standard',
  'automotive_design',2000,#2);
#2 = APPLICATION_CONTEXT(
  'core data for automotive mechanical design processes');
#3 = SHAPE_DEFINITION_REPRESENTATION(#4,#10);
#4 = PRODUCT_DEFINITION_SHAPE('','',#5);
#5 = PRODUCT_DEFINITION('design','',#6,#9);
#6 = PRODUCT_DEFINITION_FORMATION('','',#7);
#7 = PRODUCT('Open CASCADE STEP translator 8.1 1',
  'Open CASCADE STEP translator 8.1 1','',(#8));
#8 = PRODUCT_CONTEXT('',#2,'mechanical');
#9 = PRODUCT_DEFINITION_CONTEXT('part definition',#2,'design');
#10 = MANIFOLD_SURFACE_SHAPE_REPRESENTATION('',(#11,#15),#61);
#11 = AXIS2_PLACEMENT_3D('',#12,#13,#14);
#12 = CARTESIAN_POINT('',(0.,0.,0.));
#13 = DIRECTION('',(0.,0.,1.));
#14 = DIRECTION('',(1.,0.,-0.));
#15 = SHELL_BASED_SURFACE_MODEL('',(#16));
#16 = OPEN_SHELL('',(#17));
#17 = ADVANCED_FACE('',(#18,#42),#31,.T.);
#18 = FACE_BOUND('',#19,.T.);
#19 = EDGE_LOOP('',(#20));
#20 = ORIENTED_EDGE('',*,*,#21,.T.);
#21 = EDGE_CURVE('',#22,#22,#24,.T.);
#22 = VERTEX_POINT('',#23);
#23 = CARTESIAN_POINT('',(1.,2.,-1.));
#24 = SURFACE_CURVE('',#25,(#30),.PCURVE_S1.);
#25 = ELLIPSE('',#26,4.,2.);
#26 = AXIS2_PLACEMENT_3D('',#27,#28,#29);
#27 = CARTESIAN_POINT('',(1.,2.,3.));
#28 = DIRECTION('',(0.,1.,0.));
#29 = DIRECTION('',(0.,0.,-1.));
#30 = PCURVE('',#31,#36);
#31 = PLANE('',#32);
#32 = AXIS2_PLACEMENT_3D('',#33,#34,#35);
#33 = CARTESIAN_POINT('',(1.,2.,3.));
#34 = DIRECTION('',(0.,1.,0.));
#35 = DIRECTION('',(0.,0.,-1.));
#36 = DEFINITIONAL_REPRESENTATION('',(#37),#41);
#37 = ELLIPSE('',#38,4.,2.);
#38 = AXIS2_PLACEMENT_2D('',#39,#40);
#39 = CARTESIAN_POINT('',(0.,0.));
#40 = DIRECTION('',(1.,0.));
#41 = ( GEOMETRIC_REPRESENTATION_CONTEXT(2)
PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('2D SPACE',''
  ) );
#42 = FACE_BOUND('',#43,.F.);
#43 = EDGE_LOOP('',(#44));
#44 = ORIENTED_EDGE('',*,*,#45,.T.);
#45 = EDGE_CURVE('',#46,#46,#48,.T.);
#46 = VERTEX_POINT('',#47);
#47 = CARTESIAN_POINT('',(1.,2.,1.));
#48 = SURFACE_CURVE('',#49,(#54),.PCURVE_S1.);
#49 = ELLIPSE('',#50,2.,1.);
#50 = AXIS2_PLACEMENT_3D('',#51,#52,#53);
#51 = CARTESIAN_POINT('',(1.,2.,3.));
#52 = DIRECTION('',(0.,1.,0.));
#53 = DIRECTION('',(0.,0.,-1.));
#54 = PCURVE('',#31,#55);
#55 = DEFINITIONAL_REPRESENTATION('',(#56),#60);
#56 = ELLIPSE('',#57,2.,1.);
#57 = AXIS2_PLACEMENT_2D('',#58,#59);
#58 = CARTESIAN_POINT('',(0.,0.));
#59 = DIRECTION('',(1.,0.));
#60 = ( GEOMETRIC_REPRESENTATION_CONTEXT(2)
PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('2D SPACE',''
  ) );
#61 = ( GEOMETRIC_REPRESENTATION_CONTEXT(3)
GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT((#65)) GLOBAL_UNIT_ASSIGNED_CONTEXT(
(#62,#63,#64)) REPRESENTATION_CONTEXT('Context #1',
  '3D Context with UNIT and UNCERTAINTY') );
#62 = ( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );
#63 = ( NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.) );
#64 = ( NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT() );
#65 = UNCERTAINTY_MEASURE_WITH_UNIT(LENGTH_MEASURE(1.E-07),#62,
  'distance_accuracy_value','confusion accuracy');
#66 = PRODUCT_RELATED_PRODUCT_CATEGORY('part',$,(#7));

`
