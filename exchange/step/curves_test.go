package step

import (
	"bytes"
	"io"
	"math"
	"plainsolid/geom"
	"strings"
	"testing"
)

const independentCurve = `ISO-10303-21;
HEADER;FILE_DESCRIPTION(('independent spline'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));ENDSEC;
DATA;
#1=CARTESIAN_POINT('',(1.,0.,0.));
#2=CARTESIAN_POINT('',(1.,1.,0.));
#3=CARTESIAN_POINT('',(0.,1.,0.));
#4=(BOUNDED_CURVE() B_SPLINE_CURVE(2,(#1,#2,#3),.CIRCULAR_ARC.,.F.,.F.) B_SPLINE_CURVE_WITH_KNOTS((3,3),(0.,1.),.UNSPECIFIED.) CURVE() GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_CURVE((1.,0.7071067811865476,1.)) REPRESENTATION_ITEM('quarter circle'));
#5=(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.));
#6=(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#5)) REPRESENTATION_CONTEXT('',''));
#7=GEOMETRIC_CURVE_SET('',(#4));
#8=GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION('',(#7),#6);
ENDSEC;END-ISO-10303-21;`

func TestIndependentSTEPBSpline(t *testing.T) {
	m, e := ReadCurves(strings.NewReader(independentCurve), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.Curves[0].Geometry.Eval(.5)
	if e != nil || p.Distance(geom.Vec3{X: math.Sqrt(.5), Y: math.Sqrt(.5)}) > 1e-14 {
		t.Fatal(p, e)
	}
	var b bytes.Buffer
	if e = WriteCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadCurves(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	q, _ := got.Curves[0].Geometry.Eval(.5)
	if q != p || got.Curves[0].Name != m.Curves[0].Name || got.LengthUnit != .001 {
		t.Fatal(got, q, p)
	}
	f, e := Parse(strings.NewReader(independentCurve), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	f.Entities[4] = Entity{ID: 4, Records: []Record{{Type: "B_SPLINE_CURVE_WITH_KNOTS", Args: append(append([]Value{{Kind: String, Text: "polynomial"}}, f.Entities[4].Records[1].Args...), f.Entities[4].Records[2].Args...)}}}
	c, e := DecodeBSplineCurve(f, 4, 3)
	if e != nil {
		t.Fatal(e)
	}
	q, e = c.Geometry.Eval(.5)
	if e != nil || q != (geom.Vec3{X: .75, Y: .75}) {
		t.Fatal(q, e)
	}
}
func TestSTEPBSplineFailures(t *testing.T) {
	for _, pair := range [][2]string{{"(3,3)", "(300000000,3)"}, {"(0.,1.)", "(1.,0.)"}, {"0.7071067811865476", "-1."}, {".CIRCULAR_ARC.", ".UNKNOWN."}, {".F.,.F.", ".BAD.,.F."}, {"(#4)", "(#1)"}} {
		bad := strings.Replace(independentCurve, pair[0], pair[1], 1)
		if _, e := ReadCurves(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(pair)
		}
	}
	if _, e := ReadCurves(strings.NewReader(independentCurve), Limits{MaxVertices: 2}); e == nil {
		t.Fatal("limit")
	}
	if e := WriteCurves(nil, CurveModel{}); e == nil {
		t.Fatal("nil")
	}
}
func FuzzReadCurves(f *testing.F) {
	f.Add(independentCurve)
	f.Fuzz(func(t *testing.T, s string) {
		m, e := ReadCurves(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 300, MaxValues: 3000, MaxDepth: 12, MaxVertices: 100})
		if e == nil {
			for _, c := range m.Curves {
				a, b := c.Geometry.Domain()
				if _, e = c.Geometry.Eval(a*.5 + b*.5); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}

func TestCurveWriterFailures(t *testing.T) {
	m, e := ReadCurves(strings.NewReader(independentCurve), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, w := range []io.Writer{nil, failWriter{}, shortWriter{}} {
		if e = WriteCurves(w, m); e == nil {
			t.Fatal("writer failure lost")
		}
	}
	m.Curves[0].Closed = "BAD"
	if e = WriteCurves(io.Discard, m); e == nil {
		t.Fatal("flag")
	}
}

func TestFullDomainCurveWrapper(t *testing.T) {
	wrapper := "\n#9=TRIMMED_CURVE('wrapped',#4,(#1,PARAMETER_VALUE(0.)),(#3,PARAMETER_VALUE(1.)),.T.,.PARAMETER.);\n"
	text := strings.Replace(independentCurve, "#7=GEOMETRIC_CURVE_SET('',(#4));", "#7=GEOMETRIC_CURVE_SET('',(#9));"+wrapper, 1)
	m, e := ReadCurves(strings.NewReader(text), Limits{})
	if e != nil || m.Curves[0].Name != "wrapped" {
		t.Fatal(m, e)
	}
	for _, pair := range [][2]string{{"PARAMETER_VALUE(0.)", "PARAMETER_VALUE(0.25)"}, {".T.,.PARAMETER.", ".F.,.PARAMETER."}, {"(#1,PARAMETER_VALUE", "(#2,PARAMETER_VALUE"}, {"#1,PARAMETER_VALUE(0.)", "#1"}} {
		bad := strings.Replace(text, pair[0], pair[1], 1)
		if _, e = ReadCurves(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(pair)
		}
	}
}

func TestEmptyRationalCurveWeights(t *testing.T) {
	f, e := Parse(strings.NewReader(independentCurve), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ent := f.Entities[4]
	for i := range ent.Records {
		if ent.Records[i].Type == "RATIONAL_B_SPLINE_CURVE" {
			ent.Records[i].Args[0].Items = nil
		}
	}
	f.Entities[4] = ent
	if _, e = DecodeBSplineCurve(f, 4, 3); e == nil {
		t.Fatal("empty rational weights")
	}
}
