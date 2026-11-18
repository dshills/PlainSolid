package step

import (
	"bytes"
	"io"
	"math"
	"plainsolid/geom"
	"strings"
	"testing"
)

const independentSurface = `ISO-10303-21;
HEADER;FILE_DESCRIPTION(('independent surface'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));ENDSEC;
DATA;
#1=CARTESIAN_POINT('',(1.,0.,0.));#2=CARTESIAN_POINT('',(1.,0.,2.));
#3=CARTESIAN_POINT('',(1.,1.,0.));#4=CARTESIAN_POINT('',(1.,1.,2.));
#5=CARTESIAN_POINT('',(0.,1.,0.));#6=CARTESIAN_POINT('',(0.,1.,2.));
#7=(BOUNDED_SURFACE() B_SPLINE_SURFACE(2,1,((#1,#2),(#3,#4),(#5,#6)),.CYLINDRICAL_SURF.,.F.,.F.,.F.) B_SPLINE_SURFACE_WITH_KNOTS((3,3),(2,2),(0.,1.),(0.,1.),.UNSPECIFIED.) GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_SURFACE(((1.,1.),(0.7071067811865476,0.7071067811865476),(1.,1.))) REPRESENTATION_ITEM('quarter cylinder') SURFACE());
#8=(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.));
#9=(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#8)) REPRESENTATION_CONTEXT('',''));
#10=GEOMETRIC_SET('',(#7));
#11=GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION('',(#10),#9);
ENDSEC;END-ISO-10303-21;`

func TestIndependentSTEPBSplineSurface(t *testing.T) {
	t.Parallel()
	m, e := ReadSurfaces(strings.NewReader(independentSurface), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []float64{0, .5, 1} {
		p, e := m.Surfaces[0].Geometry.Eval(u, .3)
		if e != nil || math.Abs(math.Hypot(p.X, p.Y)-1) > 1e-14 || math.Abs(p.Z-.6) > 1e-14 {
			t.Fatal(p, e)
		}
	}
	var b bytes.Buffer
	if e = WriteSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSurfaces(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, _ := got.Surfaces[0].Geometry.Eval(.5, .5)
	if p.Distance(geom.Vec3{X: math.Sqrt(.5), Y: math.Sqrt(.5), Z: 1}) > 1e-14 {
		t.Fatal(p)
	}
	f, e := Parse(strings.NewReader(independentSurface), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	records := f.Entities[7].Records
	f.Entities[7] = Entity{ID: 7, Records: []Record{{Type: "B_SPLINE_SURFACE_WITH_KNOTS", Args: append(append([]Value{{Kind: String, Text: "polynomial"}}, records[1].Args...), records[2].Args...)}}}
	s, e := DecodeBSplineSurface(f, 7, 6)
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.Geometry.Eval(.5, .5)
	if e != nil || p != (geom.Vec3{X: .75, Y: .75, Z: 1}) {
		t.Fatal(p, e)
	}
}
func TestSTEPBSplineSurfaceFailures(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]string{{"(3,3)", "(300000000,3)"}, {"(#3,#4)", "(#3)"}, {"(0.,1.)", "(1.,0.)"}, {"0.7071067811865476", "-1."}, {".CYLINDRICAL_SURF.", ".BAD."}, {".F.,.F.,.F.", ".BAD.,.F.,.F."}} {
		if _, e := ReadSurfaces(strings.NewReader(strings.Replace(independentSurface, pair[0], pair[1], 1)), Limits{}); e == nil {
			t.Fatal(pair)
		}
	}
	if _, e := ReadSurfaces(strings.NewReader(independentSurface), Limits{MaxVertices: 5}); e == nil {
		t.Fatal("limit")
	}
	m, e := ReadSurfaces(strings.NewReader(independentSurface), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, w := range []io.Writer{nil, failWriter{}, shortWriter{}} {
		if e = WriteSurfaces(w, m); e == nil {
			t.Fatal("writer error")
		}
	}
}
func FuzzReadSurfaces(f *testing.F) {
	f.Add(independentSurface)
	f.Fuzz(func(t *testing.T, s string) {
		m, e := ReadSurfaces(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 300, MaxValues: 3000, MaxDepth: 12, MaxVertices: 100})
		if e == nil {
			for _, v := range m.Surfaces {
				a, b, c, d := v.Geometry.Domain()
				if _, e = v.Geometry.Eval(a*.5+b*.5, c*.5+d*.5); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}

func TestEmptyRationalSurfaceWeights(t *testing.T) {
	t.Parallel()
	f, e := Parse(strings.NewReader(independentSurface), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ent := f.Entities[7]
	for i := range ent.Records {
		if ent.Records[i].Type == "RATIONAL_B_SPLINE_SURFACE" {
			ent.Records[i].Args[0].Items = nil
		}
	}
	f.Entities[7] = ent
	if _, e = DecodeBSplineSurface(f, 7, 6); e == nil {
		t.Fatal("empty rational weights")
	}
}
