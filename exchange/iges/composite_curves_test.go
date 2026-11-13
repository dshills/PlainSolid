package iges

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func compositeWireFixture(t *testing.T) Model {
	t.Helper()
	c, e := geom.NewBSpline(2, []geom.Vec3{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 2}}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return Model{LengthUnit: .001, Entities: []Entity{{Line: &Line{Start: geom.Vec3{}, End: geom.Vec3{X: 1}}, Label: "LINE"}, {Arc: &Arc{Center: geom.Vec3{X: 1, Y: 1}, Start: geom.Vec2{X: 1}, End: geom.Vec2{X: 2, Y: 1}}, Label: "ARC"}, {Spline: &Spline{Curve: c, Start: 0, End: 1, Polynomial: true}, Label: "SPLINE"}, {Composite: &CompositeCurve{Curves: []int{0, 1, 2}}, Label: "WIRE"}}}
}
func TestCompositeCurvesRoundtrip(t *testing.T) {
	m := compositeWireFixture(t)
	var b bytes.Buffer
	if e := WriteCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	got, e := ReadCompositeCurves(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Entities) != 4 || got.Entities[3].Label != "WIRE" || got.LengthUnit != m.LengthUnit {
		t.Fatal(got)
	}
	var c bytes.Buffer
	if e = WriteCompositeCurves(&c, got); e != nil || !bytes.Equal(raw, c.Bytes()) {
		t.Fatal("noncanonical", e)
	}
	if _, e = Read(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("generic broadened")
	}
	if e = Write(&bytes.Buffer{}, m); e == nil {
		t.Fatal("generic writer broadened")
	}
	for _, l := range []Limits{{MaxBytes: 80}, {MaxEntities: 2}, {MaxControlPoints: 2}} {
		if _, e = ReadCompositeCurves(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
}
func TestCompositeCurvesRejects(t *testing.T) {
	for _, mutate := range []func(*Model){
		func(m *Model) { m.Entities[1].Arc.Start.X = math.Nextafter(1, 2) },
		func(m *Model) { m.Entities[1].Arc.End.Y = math.Nextafter(1, 2) },
		func(m *Model) { m.Entities[3].Composite.Curves = []int{1, 0, 2} },
		func(m *Model) { m.Entities[3].Composite.Curves = []int{0, 1, 0} },
		func(m *Model) { m.Entities[3].Composite.Curves = []int{3} },
		func(m *Model) {
			m.Entities[3].Composite.Curves = []int{4}
			m.Entities = append(m.Entities, Entity{Composite: &CompositeCurve{Curves: []int{3}}})
		},
		func(m *Model) { m.Entities = append(m.Entities, Entity{Composite: &CompositeCurve{Curves: []int{0}}}) },
		func(m *Model) { m.Entities[3].Composite.Curves = []int{0} },
		func(m *Model) { m.Entities[0].Line.Start.X = .1 },
	} {
		m := compositeWireFixture(t)
		mutate(&m)
		var out bytes.Buffer
		if e := WriteCompositeCurves(&out, m); e == nil || out.Len() != 0 {
			t.Fatal("invalid writer not atomic", e)
		}
	}
}
func TestCompositeCurvesSingleClosedAndTransformed(t *testing.T) {
	m := Model{LengthUnit: 1, Entities: []Entity{{Arc: &Arc{Center: geom.Vec3{}, Start: geom.Vec2{X: 1}, End: geom.Vec2{X: 1}}}, {Composite: &CompositeCurve{Curves: []int{0}}}}}
	var b bytes.Buffer
	if e := WriteCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadCompositeCurves(&b, Limits{}); e != nil {
		t.Fatal(e)
	}
	m = compositeWireFixture(t)
	a := geom.Affine{{1, 0, 0, 2}, {0, 1, 0, -4}, {0, 0, 1, 8}}
	for i := 0; i < 3; i++ {
		m.Entities[i].Transform = &a
	}
	if e := WriteCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadCompositeCurves(&b, Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestCompositeSourceExactNumbers(t *testing.T) {
	for _, s := range []string{"1.00000000000000000001", "0.1", "1E999999999", "NaN"} {
		if e := compositeSourceNumbers(110, []parameter{{text: s}}); e == nil {
			t.Fatal(s)
		}
	}
	if e := compositeSourceNumbers(110, []parameter{{text: "1.25"}}); e != nil {
		t.Fatal(e)
	}
}
func FuzzCompositeCurves(f *testing.F) {
	f.Add([]byte(""))
	if b, e := os.ReadFile("testdata/composite-curve.igs"); e == nil {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadCompositeCurves(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxRecords: 512, MaxEntities: 64, MaxValues: 4096, MaxControlPoints: 128})
	})
}

func TestCompositeCurvesSourceMutations(t *testing.T) {
	raw, e := os.ReadFile("testdata/composite-curve.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ old, new string }{
		{"102,2,3,5;", "102,2,1,5;"},
		{"102,2,3,5;", "102,2,3,3;"},
		{"102,2,3,5;", "102,2,5,3;"},
		{"102,2,3,5;", "102,65,3,5;"},
		{"110,0.,0.,0.,1.,0.,0.;", "110,0.,0.,0.,1.00000000000000000001,0.,0.;"},
		{"100,0.,1.,1.,1.,0.,2.,1.;", "100,0.,1.,1.,1.,0.,2.,1.000000000001;"},
	} {
		lines := strings.Split(string(raw), "\n")
		found := false
		for i, line := range lines {
			if strings.HasPrefix(line, tc.old) {
				lines[i] = fmt.Sprintf("%-64s", tc.new) + line[64:]
				found = true
			}
		}
		if !found {
			t.Fatal("mutation missed")
		}
		if _, e = ReadCompositeCurves(strings.NewReader(strings.Join(lines, "\n")), Limits{}); e == nil {
			t.Fatal(tc.new)
		}
	}
}

func TestCompositeCurvesTrimmedSplineAndMultipleRoots(t *testing.T) {
	c, e := geom.NewBSpline(1, []geom.Vec3{{}, {X: 2, Y: 2}}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	m := Model{LengthUnit: 1, Entities: []Entity{
		{Spline: &Spline{Curve: c, Start: .25, End: .75, Polynomial: true}},
		{Line: &Line{Start: geom.Vec3{X: 1.5, Y: 1.5}, End: geom.Vec3{X: 3, Y: 1.5}}},
		{Composite: &CompositeCurve{Curves: []int{0, 1}}},
		{Line: &Line{Start: geom.Vec3{Z: 2}, End: geom.Vec3{Z: 4}}},
		{Composite: &CompositeCurve{Curves: []int{3}}},
	}}
	var b bytes.Buffer
	if e = WriteCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadCompositeCurves(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Entities[0].Spline.Start != .25 || got.Entities[0].Spline.End != .75 || len(got.Entities) != 5 {
		t.Fatal("trim/root lost")
	}
	m.Entities[1].Line.Start.X = 2
	if e = WriteCompositeCurves(&bytes.Buffer{}, m); e == nil {
		t.Fatal("full-domain endpoint substituted for trim")
	}
}
