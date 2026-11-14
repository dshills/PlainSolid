package step

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func eccentricEllipseFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-eccentric-ellipse-face.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestEccentricEllipticalFace(t *testing.T) {
	raw := eccentricEllipseFixture(t)
	f, e := ReadEccentricEllipticalFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if f.Clearance.Center != (geom.Vec2{X: 2, Y: 1}) || f.Clearance.Reference != (geom.Vec2{Y: 1}) || f.Clearance.BoxNormSquaredUpper.Cmp(big.NewRat(801, 1600)) != 0 || f.Clearance.ClearanceLower.Cmp(big.NewRat(799, 640)) != 0 {
		t.Fatal(f.Clearance)
	}
	for _, s := range []float64{0, .125, .375, .5, .75, 1} {
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
			t.Fatal("continuous coefficient lift regression", p, q, e)
		}
	}
	if f.SourceInnerPCurveID != 54 || f.SourceInnerModelCurveID != 49 || f.SourceInnerBoundID != 42 || f.InnerVertex != (geom.Vec3{X: -2, Y: 2, Z: 1}) {
		t.Fatal("ancestry")
	}
	var source bytes.Buffer
	if e = WriteEccentricEllipticalFaceSource(&source, f.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	again, e := ReadEccentricEllipticalFace(&source, Limits{})
	if e != nil || !reflect.DeepEqual(again, f) {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteEccentricEllipticalFaceJSON(&native, f); e != nil {
		t.Fatal(e)
	}
	data := append([]byte(nil), native.Bytes()...)
	back, e := ReadEccentricEllipticalFaceJSON(&native, Limits{})
	if e != nil || !reflect.DeepEqual(back, f) {
		t.Fatal(e)
	}
	native.Reset()
	if e = WriteEccentricEllipticalFaceJSON(&native, back); e != nil || !bytes.Equal(data, native.Bytes()) {
		t.Fatal("native stability", e)
	}
	if _, e = ReadEllipticalAnnulusFace(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("concentric reader widened")
	}
	if _, e = ReadEccentricEllipticalFace(bytes.NewReader(ellipticalAnnulusFixture(t)), Limits{}); e == nil {
		t.Fatal("eccentric gate accepted concentric")
	}
}
func TestEccentricEllipticalFaceReject(t *testing.T) {
	raw := eccentricEllipseFixture(t)
	for _, m := range [][2]string{{"#54 = PCURVE('',#31,#55)", "#54 = PCURVE('',#25,#55)"}, {"#49,(#54)", "#49,(#30)"}, {"#58 = CARTESIAN_POINT('',(2.,1.))", "#58 = CARTESIAN_POINT('',(2.00000000000000001,1.))"}, {"#59 = DIRECTION('',(0.,1.))", "#59 = DIRECTION('',(0.,-1.))"}, {"#52 = DIRECTION('',(0.,1.,0.))", "#52 = DIRECTION('',(0.,-1.,0.))"}, {"#42 = FACE_BOUND('',#43,.F.)", "#42 = FACE_BOUND('',#43,.T.)"}, {"#51 = CARTESIAN_POINT('',(0.,2.,1.))", "#51 = CARTESIAN_POINT('',(0.,2.5,1.))"}, {"#46 = VERTEX_POINT('',#47)", "#46 = VERTEX_POINT('',#23)"}} {
		b := bytes.Replace(raw, []byte(m[0]), []byte(m[1]), 1)
		if bytes.Equal(raw, b) {
			t.Fatal("stale mutation", m)
		}
		f, e := ReadEccentricEllipticalFace(bytes.NewReader(b), Limits{})
		if e == nil || f.Source != nil || f.Clearance.ClearanceLower != nil {
			t.Fatal(m, e)
		}
	}
	f, e := ReadEccentricEllipticalFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteEccentricEllipticalFaceJSON(&b, f); e != nil {
		t.Fatal(e)
	}
	original := append([]byte(nil), b.Bytes()...)
	for _, change := range []func(*eccentricEllipseNative){func(n *eccentricEllipseNative) { n.ClearanceLower = "1000" }, func(n *eccentricEllipseNative) { n.Reference.X = 1 }, func(n *eccentricEllipseNative) { n.Face.InnerAncestry[3] = 30 }, func(n *eccentricEllipseNative) { n.Face.Hole.Center.X = 0 }, func(n *eccentricEllipseNative) { n.Version = 2 }} {
		var n eccentricEllipseNative
		if e = json.Unmarshal(original, &n); e != nil {
			t.Fatal(e)
		}
		change(&n)
		data, e := json.Marshal(n)
		if e != nil {
			t.Fatal(e)
		}
		if out, e := ReadEccentricEllipticalFaceJSON(bytes.NewReader(data), Limits{}); e == nil || out.Source != nil {
			t.Fatal("native forgery")
		}
	}
	f.Clearance.ClearanceLower.SetInt64(1000)
	b.Reset()
	if e = WriteEccentricEllipticalFaceJSON(&b, f); e == nil || b.Len() != 0 {
		t.Fatal("stale certificate")
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 65}, {MaxValues: 20}} {
		if _, e = ReadEccentricEllipticalFace(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limits")
		}
	}
}
func FuzzEccentricEllipticalFace(f *testing.F) {
	f.Add(eccentricEllipseFixture(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadEccentricEllipticalFace(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 66, MaxValues: 4096, MaxDepth: 16})
		if e != nil && (m.Source != nil || m.Clearance.ClearanceLower != nil) {
			t.Fatal("partial")
		}
	})
}

func eccentricEllipseMoved(t testing.TB, c, r geom.Vec2) []byte {
	t.Helper()
	f, e := Parse(bytes.NewReader(eccentricEllipseFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	put := func(id int64, a []float64) {
		en := f.Entities[id]
		v := []Value{}
		for _, x := range a {
			v = append(v, Value{Kind: Number, Text: real(x)})
		}
		en.Records[0].Args[1].Items = v
		f.Entities[id] = en
	}
	put(58, []float64{c.X, c.Y})
	put(59, []float64{r.X, r.Y})
	put(51, []float64{1 - c.Y, 2, 3 - c.X})
	put(53, []float64{-r.Y, 0, -r.X})
	put(47, []float64{1 - c.Y - 2*r.Y, 2, 3 - c.X - 2*r.X})
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestEccentricEllipticalFaceClearance(t *testing.T) {
	for _, r := range []geom.Vec2{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}} {
		raw := eccentricEllipseMoved(t, geom.Vec2{X: 2, Y: 1}, r)
		f, e := ReadEccentricEllipticalFace(bytes.NewReader(raw), Limits{})
		if e != nil || f.Clearance.Reference != r || f.Clearance.ClearanceLower.Sign() <= 0 {
			t.Fatal(r, e)
		}
	}
	for _, tc := range []struct {
		c  geom.Vec2
		ok bool
	}{{geom.Vec2{X: 6}, true}, {geom.Vec2{X: 7}, false}, {geom.Vec2{X: 6, Y: 2}, false}, {geom.Vec2{X: -2, Y: -1}, true}, {geom.Vec2{}, false}} {
		raw := eccentricEllipseMoved(t, tc.c, geom.Vec2{Y: 1})
		f, e := ReadEccentricEllipticalFace(bytes.NewReader(raw), Limits{})
		if (e == nil) != tc.ok || (e != nil && f.Source != nil) {
			t.Fatal(tc, e)
		}
	}
	// An inconclusive rectangle certificate must reject; this test does not
	// assert whether a stronger containment proof could accept the candidate.
	raw := eccentricEllipseMoved(t, geom.Vec2{X: 6, Y: 1}, geom.Vec2{Y: 1})
	if _, e := ReadEccentricEllipticalFace(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("rectangle proof was weakened")
	}
}
