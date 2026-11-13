package step

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func linearExtrusionFixture(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/linear-extrusion.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestLinearExtrusionIndependentRoundtrip(t *testing.T) {
	raw := linearExtrusionFixture(t)
	m, e := ReadLinearExtrusions(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Surfaces) != 1 || m.LengthUnit != .001 || m.Surfaces[0].Name != "curtain" || m.Surfaces[0].Profile.Name != "arch" {
		t.Fatal(m)
	}
	g, e := m.Surfaces[0].Surface()
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []float64{0, .25, .5, 1} {
		for _, v := range []float64{-1, 0, 2} {
			p, e := g.Eval(u, v)
			if e != nil || p.Sub(geom.Vec3{X: 2 * u, Y: 4 * u * (1 - u), Z: v}).Length() > 1e-14 {
				t.Fatal(p, e)
			}
		}
	}
	var b bytes.Buffer
	if e = WriteLinearExtrusions(&b, m); e != nil {
		t.Fatal(e)
	}
	again, e := ReadLinearExtrusions(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if again.Surfaces[0].VStart != -1 || again.Surfaces[0].VEnd != 2 || again.Surfaces[0].Axis != m.Surfaces[0].Axis {
		t.Fatal(again)
	}
	// Exact rational profile weights and non-default source units remain explicit.
	c, e := geom.NewBSpline(2, []geom.Vec3{{X: 0}, {X: 1, Y: 2}, {X: 2}}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	m.Surfaces[0].Profile.Geometry = c
	m.Surfaces[0].Axis = geom.Vec3{Y: -1}
	m.LengthUnit = .0254
	b.Reset()
	if e = WriteLinearExtrusions(&b, m); e != nil {
		t.Fatal(e)
	}
	again, e = ReadLinearExtrusions(&b, Limits{})
	if e != nil || again.LengthUnit != .0254 || again.Surfaces[0].Profile.Geometry.Weights()[1] != .75 {
		t.Fatal(again, e)
	}
}
func TestLinearExtrusionRejects(t *testing.T) {
	raw := linearExtrusionFixture(t)
	for _, pair := range [][2]string{
		{"#7,0.,1.,-1.,2.,.T.,.T.", "#7,0.,1.,-1.,2.,.F.,.T."},
		{"#7,0.,1.,-1.,2.,.T.,.T.", "#7,0.,1.,-1.,2.,.T.,.F."},
		{"#7,0.,1.,-1.,2.,.T.,.T.", "#7,0.25,1.,-1.,2.,.T.,.T."},
		{"#7,0.,1.,-1.,2.,.T.,.T.", "#7,0.,1.,2.,-1.,.T.,.T."},
		{"#6=VECTOR('',#5,1.);", "#6=VECTOR('',#5,2.);"},
		{"#6=VECTOR('',#5,1.);", "#6=VECTOR('',#5,1.00000000000000000001);"},
		{"#5=DIRECTION('',(0.,0.,1.));", "#5=DIRECTION('',(0.,1.,1.));"},
		{"#5=DIRECTION('',(0.,0.,1.));", "#5=DIRECTION('',(0.,0.,1.00000000000000000001));"},
		{"#7,0.,1.,-1.,2.,.T.,.T.", "#7,0.,0.99999999999999999999,-1.,2.,.T.,.T."},
		{"#9=GEOMETRIC_SET('',(#8));", "#9=GEOMETRIC_SET('',(#7));"},
		{"#9=GEOMETRIC_SET('',(#8));", "#9=GEOMETRIC_SET('',(#8,#8));"},
	} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("stale mutation")
		}
		if _, e := ReadLinearExtrusions(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal(pair)
		}
	}
	for _, l := range []Limits{{MaxVertices: 5}, {MaxEntities: 4}, {MaxBytes: 16}} {
		if _, e := ReadLinearExtrusions(bytes.NewReader(raw), l); e == nil {
			t.Fatal("budget", l)
		}
	}
	m, e := ReadLinearExtrusions(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	c, _ := geom.NewBSpline(1, []geom.Vec3{{Z: math.Ldexp(1, 53)}, {X: 1, Z: math.Ldexp(1, 53)}}, []float64{0, 0, 1, 1}, nil)
	m.Surfaces[0].Profile.Geometry = c
	m.Surfaces[0].VStart = 0
	m.Surfaces[0].VEnd = 1
	var b bytes.Buffer
	if e := WriteLinearExtrusions(&b, m); !errors.Is(e, geom.ErrUnrepresentable) || b.Len() != 0 {
		t.Fatal("rounded translation or partial write", e, b.Len())
	}
	if e := WriteLinearExtrusions(io.Discard, LinearExtrusionModel{Surfaces: []LinearExtrusionSurface{{}}, LengthUnit: .001}); e == nil {
		t.Fatal("nil profile")
	}
}
func FuzzLinearExtrusions(f *testing.F) {
	raw, _ := os.ReadFile("testdata/linear-extrusion.step")
	f.Add(raw)
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadLinearExtrusions(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 512, MaxValues: 8192, MaxVertices: 256, MaxFaces: 16})
	})
}

func TestLinearExtrusionSourceWeightAndOwnership(t *testing.T) {
	raw := linearExtrusionFixture(t)
	m, e := ReadLinearExtrusions(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteLinearExtrusions(&b, m); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		for i, r := range en.Records {
			if r.Type == "RATIONAL_B_SPLINE_CURVE" {
				en.Records[i].Args[0].Items[1] = Value{Kind: Number, Text: "3"}
				f.Entities[id] = en
			}
		}
	}
	if _, e = ReadLinearExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded weight normalization", e)
	}
	bad := bytes.Replace(raw, []byte("ENDSEC;\nEND-ISO"), []byte("#90=SURFACE_OF_LINEAR_EXTRUSION('',#1,#6);\nENDSEC;\nEND-ISO"), 1)
	if _, e = ReadLinearExtrusions(bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("unowned surface")
	}
	if _, e = DecodeLinearExtrusionSurface(nil, 1, 10); e == nil {
		t.Fatal("nil file")
	}
}

func TestLinearExtrusionRetainedOCCTSupport(t *testing.T) {
	raw, e := os.ReadFile("testdata/linear-extrusion-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "RECTANGULAR_TRIMMED_SURFACE"); !ok {
			continue
		}
		s, e := DecodeLinearExtrusionSurface(f, id, 64)
		if e != nil {
			t.Fatal(e)
		}
		g, e := s.Surface()
		if e != nil {
			t.Fatal(e)
		}
		p, e := g.Eval(.5, 2)
		if e != nil || p.Distance(geom.Vec3{X: 1, Y: 1, Z: 2}) > 1e-12 {
			t.Fatal(p, e)
		}
		found++
	}
	if found != 1 {
		t.Fatal(found)
	}
	if _, e = ReadLinearExtrusions(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("silently imported face topology")
	}
}

type extrusionFailWriter struct{}

func (extrusionFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestLinearExtrusionWriterError(t *testing.T) {
	m, e := ReadLinearExtrusions(bytes.NewReader(linearExtrusionFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteLinearExtrusions(extrusionFailWriter{}, m); !errors.Is(e, io.ErrClosedPipe) {
		t.Fatal(e)
	}
}
