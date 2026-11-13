package step

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

const independentSphere = `ISO-10303-21;
HEADER;FILE_DESCRIPTION(('independent full sphere'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));ENDSEC;
DATA;
#1=CARTESIAN_POINT('',(1.,2.,3.));
#2=DIRECTION('',(0.,0.,1.));
#3=DIRECTION('',(1.,0.,0.));
#4=AXIS2_PLACEMENT_3D('',#1,#2,#3);
#5=SPHERICAL_SURFACE('',#4,2.);
#6=CARTESIAN_POINT('',(1.,2.,5.));
#7=VERTEX_POINT('',#6);
#8=VERTEX_LOOP('',#7);
#9=FACE_BOUND('',#8,.T.);
#10=ADVANCED_FACE('',(#9),#5,.T.);
#11=CLOSED_SHELL('',(#10));
#12=MANIFOLD_SOLID_BREP('sphere',#11);
#13=(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.));
#14=(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#13)) REPRESENTATION_CONTEXT('',''));
#15=ADVANCED_BREP_SHAPE_REPRESENTATION('',(#12),#14);
ENDSEC;END-ISO-10303-21;`

func checkSphere(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(p.Volume-32*math.Pi/3) > 1e-7 || math.Abs(p.Area-16*math.Pi) > 1e-7 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 3}) > 1e-9 {
		t.Fatal(p)
	}
}
func TestIndependentSphereBody(t *testing.T) {
	m, e := ReadAnalyticBodies(strings.NewReader(independentSphere), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkSphere(t, m)
	south := strings.Replace(independentSphere, "(1.,2.,5.)", "(1.,2.,1.)", 1)
	if _, e = ReadAnalyticBodies(strings.NewReader(south), Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestSphereBodyFailures(t *testing.T) {
	for _, pair := range [][2]string{{"(1.,2.,5.)", "(3.,2.,3.)"}, {"(1.,2.,5.)", "(1.,2.,4.)"}, {"(#9),#5,.T.", "(#9),#5,.F."}, {"#8,.T.", "#8,.F."}, {"VERTEX_LOOP", "EDGE_LOOP"}, {"CLOSED_SHELL", "OPEN_SHELL"}, {"#4,2.", "#4,-2."}, {"(#10)", "(#10,#10)"}, {"(1.,0.,0.)", "(0.,0.,1.)"}} {
		bad := strings.Replace(independentSphere, pair[0], pair[1], 1)
		if bad == independentSphere {
			t.Fatal(pair)
		}
		if _, e := ReadAnalyticBodies(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(pair)
		}
	}
	for _, l := range []Limits{{MaxVertices: 1}, {MaxEdgeUses: 3}} {
		if _, e := ReadAnalyticBodies(strings.NewReader(independentSphere), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestElementarySurfaceDecode(t *testing.T) {
	for _, test := range []struct {
		entity string
		want   geom.Vec3
	}{{"PLANE('',#4)", geom.Vec3{X: 1, Y: 2, Z: 3}}, {"CYLINDRICAL_SURFACE('',#4,2.)", geom.Vec3{X: 3, Y: 2, Z: 3}}, {"SPHERICAL_SURFACE('',#4,2.)", geom.Vec3{X: 3, Y: 2, Z: 3}}, {"TOROIDAL_SURFACE('',#4,3.,1.)", geom.Vec3{X: 5, Y: 2, Z: 3}}} {
		f, e := Parse(strings.NewReader(strings.Replace(independentSphere, "SPHERICAL_SURFACE('',#4,2.)", test.entity, 1)), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		s, e := DecodeAnalyticSurface(f, 5)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.Eval(0, 0)
		if e != nil || p != test.want {
			t.Fatal(p, e)
		}
	}
}
func TestOCCTAnalyticSphereImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sphere.step")
	stepOracle(t, path, "write-step-sphere")
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	checkSphere(t, m)
}
func FuzzReadAnalyticBodies(f *testing.F) {
	f.Add(independentSphere)
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ReadAnalyticBodies(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 300, MaxValues: 3000, MaxDepth: 12, MaxVertices: 100, MaxFaces: 100, MaxSolids: 10, MaxEdgeUses: 100})
	})
}

func TestRotatedSpherePoleFrame(t *testing.T) {
	text := strings.Replace(independentSphere, "#2=DIRECTION('',(0.,0.,1.))", "#2=DIRECTION('',(1.,0.,0.))", 1)
	text = strings.Replace(text, "#3=DIRECTION('',(1.,0.,0.))", "#3=DIRECTION('',(0.,1.,0.))", 1)
	text = strings.Replace(text, "(1.,2.,5.)", "(3.,2.,3.)", 1)
	m, e := ReadAnalyticBodies(strings.NewReader(text), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkSphere(t, m)
}
