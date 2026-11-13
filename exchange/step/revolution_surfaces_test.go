package step

import (
	"bytes"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func revolutionFixture(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/revolution-surface.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRevolutionSurfaceRoundtrip(t *testing.T) {
	raw := revolutionFixture(t)
	m, e := ReadRevolutionSurfaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Surfaces) != 1 || m.LengthUnit != .001 || m.Surfaces[0].Profile.Name != "meridian" {
		t.Fatal(m)
	}
	s, e := m.Surfaces[0].Surface()
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.Eval(.5, .5)
	if e != nil || p.Distance(geom.Vec3{X: 3.25 * math.Cos(.5), Y: 3.25 * math.Sin(.5), Z: 1.25}) > 1e-14 {
		t.Fatal(p, e)
	}
	var out bytes.Buffer
	if e = WriteRevolutionSurfaces(&out, m); e != nil {
		t.Fatal(e)
	}
	again, e := ReadRevolutionSurfaces(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = WriteRevolutionSurfaces(&encoded, again); e != nil || !bytes.Equal(out.Bytes(), encoded.Bytes()) {
		t.Fatal("roundtrip", e)
	}
	if _, e = ReadSurfaces(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("tensor codec broadened")
	}
	if _, e = ReadLinearExtrusions(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("extrusion codec broadened")
	}
	m.Surfaces[0].AngleStart = -.5
	m.Surfaces[0].AngleEnd = .75
	m.Surfaces[0].Axis = geom.Vec3{Z: -1}
	m.LengthUnit = .0254
	out.Reset()
	if e = WriteRevolutionSurfaces(&out, m); e != nil {
		t.Fatal(e)
	}
	again, e = ReadRevolutionSurfaces(&out, Limits{})
	if e != nil || again.LengthUnit != .0254 || again.Surfaces[0].AngleStart != -.5 || again.Surfaces[0].Axis.Z != -1 {
		t.Fatal(again, e)
	}
}
func TestRevolutionSurfaceRejectsSource(t *testing.T) {
	raw := revolutionFixture(t)
	for _, pair := range [][2]string{
		{"#7,0.25,1.25,0.,1.,.T.,.T.", "#7,0.25,1.25,0.,1.,.F.,.T."},
		{"#7,0.25,1.25,0.,1.,.T.,.T.", "#7,0.25,1.25,0.,1.,.T.,.F."},
		{"#7,0.25,1.25,0.,1.,.T.,.T.", "#7,1.25,0.25,0.,1.,.T.,.T."},
		{"#7,0.25,1.25,0.,1.,.T.,.T.", "#7,0.,6.28318530718,0.,1.,.T.,.T."},
		{"#7,0.25,1.25,0.,1.,.T.,.T.", "#7,0.25,1.25,0.1,1.,.T.,.T."},
		{"#5=DIRECTION('',(0.,0.,1.));", "#5=DIRECTION('',(0.,0.,1.00000000000000000001));"},
		{"#3=CARTESIAN_POINT('',(4.,0.,1.));", "#3=CARTESIAN_POINT('',(4.,0.,0.));"},
		{"#3=CARTESIAN_POINT('',(4.,0.,1.));", "#3=CARTESIAN_POINT('',(4.,0.000000000000000001,1.));"},
		{"#2=CARTESIAN_POINT('',(2.,0.,0.));", "#2=CARTESIAN_POINT('',(0.,0.,0.));"},
		{"SI_UNIT($,.RADIAN.)", "SI_UNIT(.MILLI.,.RADIAN.)"},
		{"SI_UNIT($,.RADIAN.)", "SI_UNIT($,.DEGREE.)"},
		{"#9=GEOMETRIC_SET('',(#8));", "#9=GEOMETRIC_SET('',(#7));"},
	} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(raw, bad) {
			t.Fatal("stale", pair)
		}
		if _, e := ReadRevolutionSurfaces(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("accepted", pair)
		}
	}
	for _, l := range []Limits{{MaxBytes: 16}, {MaxVertices: 2}, {MaxEntities: 4}} {
		if _, e := ReadRevolutionSurfaces(bytes.NewReader(raw), l); e == nil {
			t.Fatal("budget")
		}
	}
	m, e := ReadRevolutionSurfaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Surfaces[0].AngleEnd = 10
	var out bytes.Buffer
	if e = WriteRevolutionSurfaces(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("partial invalid output")
	}
}
func FuzzRevolutionSurfaces(f *testing.F) {
	raw, _ := os.ReadFile("testdata/revolution-surface.step")
	f.Add(raw)
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRevolutionSurfaces(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 512, MaxValues: 8192, MaxVertices: 256, MaxFaces: 16})
	})
}

func TestRevolutionSurfaceExactSourceGates(t *testing.T) {
	raw := revolutionFixture(t)
	for _, pair := range [][2]string{{"(2.,0.,0.)", "(2.,1.,0.)"}, {"(4.,0.,1.)", "(4.,1.,1.)"}, {"(3.,0.,3.)", "(3.,1.,3.)"}, {"#15=CARTESIAN_POINT('',(0.,0.,0.));", "#15=CARTESIAN_POINT('',(0.,1.,0.));"}} {
		raw = bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
	}
	m, e := ReadRevolutionSurfaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	bad := bytes.Replace(raw, []byte("(4.,1.,1.)"), []byte("(4.,1.00000000000000000001,1.)"), 1)
	if _, e = ReadRevolutionSurfaces(bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("source plane repaired by float rounding")
	}
	var b bytes.Buffer
	if e = WriteRevolutionSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		for i, r := range en.Records {
			if r.Type == "RATIONAL_B_SPLINE_CURVE" {
				en.Records[i].Args[0].Items[1] = Value{Kind: Number, Text: "1.00000000000000000001"}
				f.Entities[id] = en
			}
		}
	}
	if _, e = ReadRevolutionSurfaces(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
		t.Fatal("rational profile repaired to polynomial")
	}
}
