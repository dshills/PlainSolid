package step

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func ellipticalDiskFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-elliptical-disk-face.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestEllipticalDiskFace(t *testing.T) {
	t.Parallel()
	raw := ellipticalDiskFixture(t)
	f, e := ReadEllipticalDiskFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if f.SourceFaceID != 17 || f.SourceEdgeID != 21 || f.SourceVertexID != 22 || f.SourcePCurveID != 30 || f.SourceModelCurveID != 25 || f.SourcePlaneID != 31 || f.LengthUnit != .001 || f.MajorRadius != 4 || f.MinorRadius != 2 || f.Vertex != (geom.Vec3{X: 1, Y: 2, Z: -1}) {
		t.Fatal("source roles", f)
	}
	for _, t0 := range []float64{0, .125, .375, .5, .75, 1} {
		uv, e := f.Face.Outer[0].Curve.EvalUV(t0)
		if e != nil {
			t.Fatal(e)
		}
		p, e := f.Face.Surface.Eval(uv.X, uv.Y)
		if e != nil {
			t.Fatal(e)
		}
		q, e := f.Boundary.Eval(2 * math.Pi * t0)
		if e != nil || p.Distance(q) > 2e-14 {
			t.Fatal("model/pcurve", p, q, e)
		}
	}
	du, dv, e := f.Face.Surface.Partials(0, 0)
	if e != nil || du.Cross(dv) != (geom.Vec3{Y: 1}) {
		t.Fatal("orientation", du, dv, e)
	}
	var source bytes.Buffer
	if e = WriteEllipticalDiskFaceSource(&source, f.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	again, e := ReadEllipticalDiskFace(&source, Limits{})
	if e != nil || !reflect.DeepEqual(again.Source, f.Source) {
		t.Fatal("source reload", e)
	}
	var native bytes.Buffer
	if e = WriteEllipticalDiskFaceJSON(&native, f); e != nil {
		t.Fatal(e)
	}
	original := append([]byte(nil), native.Bytes()...)
	back, e := ReadEllipticalDiskFaceJSON(&native, Limits{})
	if e != nil || !reflect.DeepEqual(back, f) {
		t.Fatal("native reload", e)
	}
	native.Reset()
	if e = WriteEllipticalDiskFaceJSON(&native, back); e != nil || !bytes.Equal(original, native.Bytes()) {
		t.Fatal("native unstable", e)
	}
	if _, e = ReadEllipticalWire(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("wire reader widened")
	}
}
func TestEllipticalDiskFaceReject(t *testing.T) {
	t.Parallel()
	raw := ellipticalDiskFixture(t)
	for _, m := range [][2]string{{"#30 = PCURVE('',#31,#36)", "#30 = PCURVE('',#25,#36)"}, {"#37),#41)", "#37),#42)"}, {"#22,#22,#24,.T.", "#22,#22,#24,.F."}, {"#21,.T.", "#21,.F."}, {"#19,.T.", "#19,.F."}, {"#31,.T.", "#31,.F."}, {"#22 = VERTEX_POINT('',#23)", "#22 = VERTEX_POINT('',#27)"}, {"#37 = ELLIPSE('',#38,4.,2.)", "#37 = ELLIPSE('',#38,4.,2.00000000000000001)"}, {"#40 = DIRECTION('',(1.,0.))", "#40 = DIRECTION('',(-1.,0.))"}, {"#35 = DIRECTION('',(0.,0.,-1.))", "#35 = DIRECTION('',(1.,0.,0.))"}, {".PCURVE_S1.", ".CURVE_3D."}, {"(1.,2.,-1.)", "(1.,2.,-1.00000000000000001)"}, {"SI_UNIT(.MILLI.,.METRE.)", "SI_UNIT($,.METRE.)"}} {
		b := bytes.Replace(raw, []byte(m[0]), []byte(m[1]), 1)
		if bytes.Equal(b, raw) {
			t.Fatal("stale mutation", m)
		}
		f, e := ReadEllipticalDiskFace(bytes.NewReader(b), Limits{})
		if e == nil || f.Source != nil || f.Face.Surface != nil {
			t.Fatal(m, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 46}, {MaxValues: 20}} {
		if _, e := ReadEllipticalDiskFace(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	f, e := ReadEllipticalDiskFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteEllipticalDiskFaceJSON(&b, f); e != nil {
		t.Fatal(e)
	}
	data := append([]byte(nil), b.Bytes()...)
	for _, mutate := range []func(*ellipticalDiskNative){func(n *ellipticalDiskNative) { n.Version = 2 }, func(n *ellipticalDiskNative) { n.Outer.To = math.Pi }, func(n *ellipticalDiskNative) { n.Vertex.X += 1 }, func(n *ellipticalDiskNative) { n.Ancestry[1] = 25 }, func(n *ellipticalDiskNative) { n.Surface = json.RawMessage(`{"Kind":"plane"}`) }} {
		var n ellipticalDiskNative
		if e = json.Unmarshal(data, &n); e != nil {
			t.Fatal(e)
		}
		mutate(&n)
		r, e := json.Marshal(n)
		if e != nil {
			t.Fatal(e)
		}
		if back, e := ReadEllipticalDiskFaceJSON(bytes.NewReader(r), Limits{}); e == nil || back.Source != nil {
			t.Fatal("native forged")
		}
	}
	f.Face.Reversed = true
	b.Reset()
	if e = WriteEllipticalDiskFaceJSON(&b, f); e == nil || b.Len() != 0 {
		t.Fatal("stale native orientation")
	}
	if _, e = ReadEllipticalDiskFaceJSON(bytes.NewReader(data), Limits{MaxBytes: 100}); e == nil {
		t.Fatal("native limit")
	}
}
func FuzzEllipticalDiskFace(f *testing.F) {
	f.Add(ellipticalDiskFixture(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadEllipticalDiskFace(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 47, MaxValues: 4096, MaxDepth: 16})
		if e != nil && (m.Source != nil || m.Face.Surface != nil) {
			t.Fatal("partial")
		}
	})
}

func TestEllipticalDiskFaceIDsAndWriterBound(t *testing.T) {
	t.Parallel()
	f, e := ReadEllipticalDiskFace(bytes.NewReader(ellipticalDiskFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ids := map[int64]int64{}
	for id := range f.Source.Entities {
		ids[id] = id + 200
	}
	var b bytes.Buffer
	analyticSupportHeader(&b)
	for i := len(f.Source.Order) - 1; i >= 0; i-- {
		id := f.Source.Order[i]
		fmt.Fprintf(&b, "#%d=%s;\n", id+200, assemblyEntity(f.Source.Entities[id], ids))
	}
	b.WriteString("ENDSEC;END-ISO-10303-21;")
	q, e := ReadEllipticalDiskFace(&b, Limits{})
	if e != nil || q.SourceFaceID != 217 || q.SourcePCurveID != 230 {
		t.Fatal("source identity", e)
	}
	cyc := Value{Kind: Aggregate}
	cyc.Items = make([]Value, 1)
	cyc.Items[0] = cyc
	f.Source.Header[0].Args = []Value{cyc}
	b.Reset()
	if e = WriteEllipticalDiskFaceSource(&b, f.Source, Limits{}); e == nil || b.Len() != 0 {
		t.Fatal("cyclic caller input")
	}
}
