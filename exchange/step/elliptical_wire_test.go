package step

import (
	"bytes"
	"math"
	"os"
	"plainsolid/geom"
	"reflect"
	"strconv"
	"testing"
)

func ellipticalWireFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-natural-ellipse-wire.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestEllipticalWire(t *testing.T) {
	m, e := ReadEllipticalWire(bytes.NewReader(ellipticalWireFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.MajorRadius != 4 || m.MinorRadius != 2 || m.LengthUnit != .001 || m.EllipseID != 16 || m.Location != (geom.Vec3{X: 1, Y: 2, Z: 3}) {
		t.Fatal(m)
	}
	p, e := m.Geometry.Eval(.375)
	want := geom.Vec3{X: 1 - 2*math.Sin(.375), Y: 2, Z: 3 - 4*math.Cos(.375)}
	if e != nil || p.Distance(want) > 1e-14 {
		t.Fatal(p, want, e)
	}
	var b bytes.Buffer
	if e = WriteEllipticalWireSource(&b, m.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	n, e := ReadEllipticalWire(&b, Limits{})
	if e != nil || n.Name != m.Name || n.Geometry != m.Geometry || !reflect.DeepEqual(n.Source, m.Source) {
		t.Fatal(e)
	}
	if _, e := ReadCurves(bytes.NewReader(ellipticalWireFixture(t)), Limits{}); e == nil {
		t.Fatal("old spline reader widened")
	}

	// The source entity IDs/order are not a canonical role assumption.
	raw, e := mappedPlanarSourceBytes(m.Source)
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ids := map[int64]int64{}
	for id := range f.Entities {
		ids[id] = id + 100
	}
	var out bytes.Buffer
	analyticSupportHeader(&out)
	for i := len(f.Order) - 1; i >= 0; i-- {
		id := f.Order[i]
		out.WriteString("#" + strconv.FormatInt(id+100, 10) + "=" + assemblyEntity(f.Entities[id], ids) + ";\n")
	}
	out.WriteString("ENDSEC;END-ISO-10303-21;")
	q, e := ReadEllipticalWire(&out, Limits{})
	if e != nil || q.EllipseID != 116 {
		t.Fatal(e)
	}
}
func TestEllipticalWireReject(t *testing.T) {
	raw := ellipticalWireFixture(t)
	for _, m := range [][2]string{{"4.,2.)", "2.,4.)"}, {"4.,2.)", "4.00000000000000001,2.)"}, {"(0.,1.,0.)", "(0.,1.,0.00000000000000001)"}, {"('',(#16))", "('',(#16,#16))"}, {"#15 = GEOMETRIC_CURVE_SET", "#15 = GEOMETRIC_SET"}, {"(#4,#10)", "(#4,#15)"}, {"SI_UNIT(.MILLI.,.METRE.)", "SI_UNIT($,.METRE.)"}, {"(#25))", "(#23))"}} {
		b := bytes.Replace(raw, []byte(m[0]), []byte(m[1]), 1)
		if bytes.Equal(b, raw) {
			t.Fatal("stale mutation", m)
		}
		q, e := ReadEllipticalWire(bytes.NewReader(b), Limits{})
		if e == nil || q.Source != nil {
			t.Fatal(m, e)
		}
	}
	b, e := os.ReadFile("testdata/occt-rounded-ellipse-wire.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipticalWire(bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("rounded period")
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 25}, {MaxValues: 20}} {
		if _, e = ReadEllipticalWire(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limits")
		}
	}
	m, e := ReadEllipticalWire(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	en := m.Source.Entities[16]
	en.Records[0].Args[2].Text = "0"
	m.Source.Entities[16] = en
	var out bytes.Buffer
	if e = WriteEllipticalWireSource(&out, m.Source, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("tamper not atomic")
	}
}
func FuzzEllipticalWire(f *testing.F) {
	f.Add(ellipticalWireFixture(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadEllipticalWire(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 26, MaxValues: 2048, MaxDepth: 16})
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}

func TestEllipticalWireWriterBound(t *testing.T) {
	m, e := ReadEllipticalWire(bytes.NewReader(ellipticalWireFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	v := Value{Kind: Aggregate}
	v.Items = make([]Value, 1)
	v.Items[0] = v
	m.Source.Header[0].Args = []Value{v}
	var b bytes.Buffer
	if e = WriteEllipticalWireSource(&b, m.Source, Limits{}); e == nil || b.Len() != 0 {
		t.Fatal("cyclic caller-owned value")
	}
}
