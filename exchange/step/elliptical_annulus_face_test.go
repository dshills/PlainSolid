package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func ellipticalAnnulusFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-elliptical-annulus-face.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestEllipticalAnnulusFace(t *testing.T) {
	t.Parallel()
	raw := ellipticalAnnulusFixture(t)
	f, e := ReadEllipticalAnnulusFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if f.SourceFaceID != 17 || f.SourceInnerBoundID != 42 || f.SourceInnerEdgeID != 45 || f.SourceInnerVertexID != 46 || f.SourceInnerPCurveID != 54 || f.SourceInnerModelCurveID != 49 || f.InnerMajorRadius != 2 || f.InnerMinorRadius != 1 || len(f.Face.Holes) != 1 {
		t.Fatal("source roles", f)
	}
	for _, s := range []float64{0, .125, .375, .75, 1} {
		uv, e := f.Face.Holes[0][0].Curve.EvalUV(s)
		if e != nil {
			t.Fatal(e)
		}
		p, e := f.Face.Surface.Eval(uv.X, uv.Y)
		if e != nil {
			t.Fatal(e)
		}
		q, e := f.InnerBoundary.Eval(2 * math.Pi * (1 - s))
		if e != nil || p.Distance(q) > 1e-14 {
			t.Fatal("hole lift", p, q, e)
		}
	}
	if !f.Face.Holes[0][0].Reversed || f.Face.Outer[0].Reversed {
		t.Fatal("opposite uses")
	}
	var source bytes.Buffer
	if e = WriteEllipticalAnnulusFaceSource(&source, f.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	again, e := ReadEllipticalAnnulusFace(&source, Limits{})
	if e != nil || !reflect.DeepEqual(again, f) {
		t.Fatal("source reload", e)
	}
	var native bytes.Buffer
	if e = WriteEllipticalAnnulusFaceJSON(&native, f); e != nil {
		t.Fatal(e)
	}
	rawNative := append([]byte(nil), native.Bytes()...)
	back, e := ReadEllipticalAnnulusFaceJSON(&native, Limits{})
	if e != nil || !reflect.DeepEqual(back, f) {
		t.Fatal("native", e)
	}
	native.Reset()
	if e = WriteEllipticalAnnulusFaceJSON(&native, back); e != nil || !bytes.Equal(rawNative, native.Bytes()) {
		t.Fatal("native stability", e)
	}
	if _, e = ReadEllipticalDiskFace(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("disk reader widened")
	}
}
func TestEllipticalAnnulusFaceReject(t *testing.T) {
	t.Parallel()
	raw := ellipticalAnnulusFixture(t)
	for _, m := range [][2]string{{"#42 = FACE_BOUND('',#43,.F.)", "#42 = FACE_BOUND('',#43,.T.)"}, {"#54 = PCURVE('',#31,#55)", "#54 = PCURVE('',#25,#55)"}, {"#49,(#54)", "#49,(#30)"}, {"#49 = ELLIPSE('',#50,2.,1.)", "#49 = ELLIPSE('',#50,4.,1.)"}, {"#49 = ELLIPSE('',#50,2.,1.)", "#49 = ELLIPSE('',#50,3.,2.)"}, {"#56 = ELLIPSE('',#57,2.,1.)", "#56 = ELLIPSE('',#57,2.,1.00000000000000001)"}, {"#51 = CARTESIAN_POINT('',(1.,2.,3.))", "#51 = CARTESIAN_POINT('',(1.25,2.,3.))"}, {"#45,.T.", "#45,.F."}, {"#46,#46,#48,.T.", "#46,#46,#48,.F."}, {"#46 = VERTEX_POINT('',#47)", "#46 = VERTEX_POINT('',#23)"}, {"#18,#42", "#42,#18"}, {"#56),#60", "#37),#60"}} {
		b := bytes.Replace(raw, []byte(m[0]), []byte(m[1]), 1)
		if bytes.Equal(raw, b) {
			t.Fatal("stale mutation", m)
		}
		f, e := ReadEllipticalAnnulusFace(bytes.NewReader(b), Limits{})
		if e == nil || f.Source != nil {
			t.Fatal(m, e)
		}
	}
	f, e := ReadEllipticalAnnulusFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteEllipticalAnnulusFaceJSON(&b, f); e != nil {
		t.Fatal(e)
	}
	data := append([]byte(nil), b.Bytes()...)
	for _, change := range []func(*ellipticalAnnulusNative){func(n *ellipticalAnnulusNative) { n.Hole.From = 0; n.Hole.To = 2 * math.Pi }, func(n *ellipticalAnnulusNative) { n.InnerAncestry[3] = 30 }, func(n *ellipticalAnnulusNative) { n.InnerMajorRadius = 4 }, func(n *ellipticalAnnulusNative) { n.InnerCurve = n.Outer.ModelCurve }, func(n *ellipticalAnnulusNative) { n.Version = 2 }} {
		var n ellipticalAnnulusNative
		if e = json.Unmarshal(data, &n); e != nil {
			t.Fatal(e)
		}
		change(&n)
		out, e := json.Marshal(n)
		if e != nil {
			t.Fatal(e)
		}
		if result, e := ReadEllipticalAnnulusFaceJSON(bytes.NewReader(out), Limits{}); e == nil || result.Source != nil {
			t.Fatal("native tamper")
		}
	}
	f.Face.Holes[0][0].Reversed = false
	b.Reset()
	if e = WriteEllipticalAnnulusFaceJSON(&b, f); e == nil || b.Len() != 0 {
		t.Fatal("stale native winding")
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 65}, {MaxValues: 20}} {
		if _, e = ReadEllipticalAnnulusFace(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
}
func FuzzEllipticalAnnulusFace(f *testing.F) {
	f.Add(ellipticalAnnulusFixture(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadEllipticalAnnulusFace(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 66, MaxValues: 4096, MaxDepth: 16})
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}

func TestEllipticalAnnulusContainment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		a, b float64
		ok   bool
	}{{2.5, .5, true}, {4, 1, false}, {3, 2, false}, {5, 1, false}, {4, 2, false}, {.5, 1, false}} {
		f, e := Parse(bytes.NewReader(ellipticalAnnulusFixture(t)), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		for _, id := range []int64{49, 56} {
			en := f.Entities[id]
			en.Records[0].Args[2].Text = real(tc.a)
			en.Records[0].Args[3].Text = real(tc.b)
			f.Entities[id] = en
		}
		en := f.Entities[47]
		en.Records[0].Args[1].Items[2].Text = real(3 - tc.a)
		f.Entities[47] = en
		raw, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		_, e = ReadEllipticalAnnulusFace(bytes.NewReader(raw), Limits{})
		if (e == nil) != tc.ok {
			t.Fatal(tc, e)
		}
	}
}
