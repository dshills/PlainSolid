package iges

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	"plainsolid/geom"
)

func fixture(t testing.TB) string {
	t.Helper()
	b, e := os.ReadFile("testdata/curves.iges")
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestIndependentCurves(t *testing.T) {
	m, e := Read(strings.NewReader(fixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.LengthUnit != .001 || len(m.Entities) != 3 {
		t.Fatal(m)
	}
	if m.Entities[0].Line.End != (geom.Vec3{X: 10, Y: 20, Z: 30}) || m.Entities[0].Label != "LINE" {
		t.Fatal(m.Entities[0])
	}
	a := m.Entities[1].Arc
	p, e := a.Eval(.5)
	want := geom.Vec3{X: math.Sqrt(2), Y: math.Sqrt(2), Z: 3}
	if e != nil || p.Distance(want) > 1e-12 {
		t.Fatal(p, e)
	}
	sweep, e := a.Sweep()
	if e != nil || math.Abs(sweep-math.Pi/2) > 1e-12 {
		t.Fatal(sweep, e)
	}
	s := m.Entities[2].Spline
	if !s.Planar || s.Periodic || s.Polynomial || s.Closed || s.Start != 0 || s.End != 1 {
		t.Fatal(s)
	}
	for _, u := range []float64{0, .25, .5, .75, 1} {
		p, e := s.Curve.Eval(u)
		if e != nil || math.Abs(p.Length()-1) > 1e-12 || p.Z != 0 {
			t.Fatal(p, e)
		}
	}
}
func TestRoundTrip(t *testing.T) {
	m, e := Read(strings.NewReader(fixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Entities[2].Spline.Start = .1
	m.Entities[2].Spline.End = .9
	m.Entities[0].Subscript = 42
	for _, unit := range units {
		m.LengthUnit = unit.meters
		var b bytes.Buffer
		if e = Write(&b, m); e != nil {
			t.Fatal(e)
		}
		for _, line := range strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n") {
			if len(line) != 80 {
				t.Fatal("bad width")
			}
			if !strings.ContainsRune("SGDPT", rune(line[72])) {
				t.Fatal("bad section")
			}
		}
		got, e := Read(&b, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if got.LengthUnit != unit.meters || len(got.Entities) != 3 || got.Entities[0].Subscript != 42 || got.Entities[2].Spline.Start != .1 || got.Entities[2].Spline.End != .9 {
			t.Fatal(got)
		}
		a, bp := got.Entities[2].Spline.Curve.Poles(), m.Entities[2].Spline.Curve.Poles()
		for i := range a {
			if a[i] != bp[i] {
				t.Fatal("control point changed")
			}
		}
		for _, u := range []float64{.1, .3, .7, .9} {
			a, _ := got.Entities[2].Spline.Curve.Eval(u)
			b, _ := m.Entities[2].Spline.Curve.Eval(u)
			if a.Distance(b) > 1e-14 {
				t.Fatal("geometry changed")
			}
		}
	}
}
func TestCustomDelimitersAndCRLF(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(fixture(t), "\n"), "\n")
	for i, line := range lines {
		if line[72] == 'G' || line[72] == 'P' {
			n := 72
			if line[72] == 'P' {
				n = 64
			}
			lines[i] = strings.NewReplacer(",", "^", ";", "!").Replace(line[:n]) + line[n:]
		}
	}
	if _, e := Read(strings.NewReader(strings.Join(lines, "\r\n")+"\r\n"), Limits{}); e != nil {
		t.Fatal(e)
	}
}
func replaceField(s string, section byte, number, fieldIndex int, text string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	n := 0
	for i, line := range lines {
		if line[72] == section {
			n++
			if n == number {
				a := []byte(line)
				value := strings.Repeat(" ", 8-len(text)) + text
				copy(a[fieldIndex*8:fieldIndex*8+8], value)
				lines[i] = string(a)
				break
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
func TestMalformedRecordsAndPointers(t *testing.T) {
	s := fixture(t)
	for _, bad := range []string{"", s[:50], s + strings.Split(s, "\n")[0] + "\n", replaceField(s, 'D', 1, 0, "116"), replaceField(s, 'D', 1, 1, "99999"), replaceField(s, 'D', 1, 6, "3"), replaceField(s, 'D', 3, 1, "1"), replaceField(s, 'D', 2, 3, "99999"), replaceField(s, 'P', 1, 8, "2"), replaceField(s, 'D', 2, 4, "1"), strings.Replace(s, "1.D1", "NAN ", 1), strings.Replace(s, "0.7071067811865476", "-0.707106781186547", 1)} {
		if _, e := Read(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatalf("accepted malformed %q", bad)
		}
	}
}
func TestLimits(t *testing.T) {
	for _, l := range []Limits{{MaxBytes: 10}, {MaxRecords: 1}, {MaxEntities: 1}, {MaxValues: 1}, {MaxControlPoints: 2}, {MaxBytes: -1}} {
		if _, e := Read(strings.NewReader(fixture(t)), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestParameterGrammar(t *testing.T) {
	for _, s := range []string{"110,1Hbad;", "110,NaN;", "110,1,2", "110,1;garbage"} {
		budget := 100
		p, e := parseParameters(s, ',', ';', &budget)
		if e == nil {
			if _, e = realValue(p[len(p)-1]); e == nil {
				t.Fatal(s)
			}
		}
	}
	budget := 100
	p, sep, end, e := globalParameters("1H^ ^1H!^0H!", &budget)
	if e != nil || sep != '^' || end != '!' || len(p) != 3 {
		t.Fatal("separator spacing", p, sep, end, e)
	}
	budget = 100
	p, e = parseParameters("3Ha,b,1.D+1,,;", ',', ';', &budget)
	if e != nil || p[0].text != "a,b" || !p[0].stringValue || len(p) != 4 {
		t.Fatal(p, e)
	}
	v, e := realValue(p[1])
	if e != nil || v != 10 {
		t.Fatal(v, e)
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) / 2, nil }
func TestIOAndInvalidGeometry(t *testing.T) {
	if _, e := Read(nil, Limits{}); e == nil {
		t.Fatal("nil")
	}
	if _, e := Read(brokenReader{}, Limits{}); !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Fatal(e)
	}
	m, e := Read(strings.NewReader(fixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, w := range []io.Writer{nil, brokenWriter{}, shortWriter{}} {
		if e = Write(w, m); e == nil {
			t.Fatal("writer error")
		}
	}
	for _, entity := range []Entity{{}, {Line: &Line{}}, {Arc: &Arc{Start: geom.Vec2{X: 1}, End: geom.Vec2{Y: 2}}}, {Spline: &Spline{Curve: &geom.BSpline{}, Start: 0, End: 1}}, {Line: &Line{End: geom.Vec3{X: 1}}, Label: "too long label"}} {
		if e = Write(io.Discard, Model{LengthUnit: 1, Entities: []Entity{entity}}); e == nil {
			t.Fatal(entity)
		}
	}
	if e = Write(io.Discard, Model{LengthUnit: .123}); e == nil {
		t.Fatal("units")
	}
}
func TestFullCircle(t *testing.T) {
	a := Arc{Start: geom.Vec2{X: 1}, End: geom.Vec2{X: 1}}
	s, e := a.Sweep()
	if e != nil || s != 2*math.Pi {
		t.Fatal(s, e)
	}
	p, e := a.Eval(.5)
	if e != nil || p.Distance(geom.Vec3{X: -1}) > 1e-14 {
		t.Fatal(p, e)
	}
	if _, e = a.Eval(2); e == nil {
		t.Fatal("domain")
	}
}
func FuzzRead(f *testing.F) {
	f.Add(fixture(f))
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		m, e := Read(strings.NewReader(s), Limits{MaxBytes: 1 << 16, MaxRecords: 300, MaxEntities: 30, MaxValues: 3000, MaxControlPoints: 100})
		if e == nil {
			for _, entity := range m.Entities {
				if e = entity.validate(); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}

func TestArcNumericResolution(t *testing.T) {
	a := Arc{Start: geom.Vec2{X: -1, Y: 1e-30}, End: geom.Vec2{X: -1, Y: 2e-30}}
	if _, e := a.Sweep(); e == nil {
		t.Fatal("tiny sweep silently became a full circle")
	}
	a = Arc{Center: geom.Vec3{Z: 7}, Start: geom.Vec2{X: 3}, End: geom.Vec2{Y: 3}}
	p, e := a.Eval(1)
	if e != nil || p != (geom.Vec3{Y: 3, Z: 7}) {
		t.Fatal(p, e)
	}
}
func TestDecimalRealsOnly(t *testing.T) {
	for _, s := range []string{"0x1p2", "NaN", "Inf", "1 2", "1.E", "--1"} {
		if _, e := realValue(parameter{text: s}); e == nil {
			t.Fatal(s)
		}
	}
	for _, s := range []string{"1.D+2", "-.5E-2", "2", "+3.0", ".1"} {
		if _, e := realValue(parameter{text: s}); e != nil {
			t.Fatal(s, e)
		}
	}
}
func TestEmptyModel(t *testing.T) {
	var b bytes.Buffer
	if e := Write(&b, Model{LengthUnit: 1}); e != nil {
		t.Fatal(e)
	}
	m, e := Read(&b, Limits{})
	if e != nil || len(m.Entities) != 0 || m.LengthUnit != 1 {
		t.Fatal(m, e)
	}
}

func TestAffinePlacementRoundTrip(t *testing.T) {
	rotation, e := geom.Rotation(geom.Vec3{Y: 1}, math.Pi/2)
	if e != nil {
		t.Fatal(e)
	}
	transform := geom.Translation(geom.Vec3{X: 2, Y: 3, Z: 4}).Mul(rotation)
	m := Model{LengthUnit: 1, Entities: []Entity{{Arc: &Arc{Start: geom.Vec2{X: 1}, End: geom.Vec2{Y: 1}}, Transform: &transform}}}
	var b bytes.Buffer
	if e = Write(&b, m); e != nil {
		t.Fatal(e)
	}
	data := b.String()
	got, e := Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Entities) != 1 || got.Entities[0].Transform == nil {
		t.Fatal(got)
	}
	for _, u := range []float64{0, .5, 1} {
		local, _ := m.Entities[0].Arc.Eval(u)
		want := transform.Apply(local)
		p, e := got.Entities[0].Eval(u)
		if e != nil || p.Distance(want) > 1e-12 {
			t.Fatal(p, want, e)
		}
	}
	for _, bad := range []string{replaceField(data, 'D', 1, 6, "999"), replaceField(data, 'D', 3, 6, "1"), replaceField(data, 'D', 1, 6, "0")} {
		if _, e := Read(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("invalid matrix reference")
		}
	}
	zero := geom.Affine{}
	m.Entities[0].Transform = &zero
	if e := Write(io.Discard, m); e == nil {
		t.Fatal("singular matrix")
	}
}
