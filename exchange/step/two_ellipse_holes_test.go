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

func twoEllipseFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-two-ellipse-holes.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestTwoEllipticalHoleFace(t *testing.T) {
	raw := twoEllipseFixture(t)
	f, e := ReadTwoEllipticalHoleFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(f.Face.Holes) != 2 || f.SeparationAxis != 0 || f.SeparationLower.Cmp(big.NewRat(3, 1)) != 0 || f.Holes[0].SourceBoundID != 42 || f.Holes[1].SourceBoundID != 61 || f.Holes[0].SourcePCurveID != 54 || f.Holes[1].SourcePCurveID != 73 || f.Face.Holes[0][0].Edge != 1 || f.Face.Holes[1][0].Edge != 2 {
		t.Fatal("owners/separation", f)
	}
	if f.Holes[0].Clearance.BoxNormSquaredUpper.Cmp(big.NewRat(5, 18)) != 0 || f.Holes[0].Clearance.ClearanceLower.Cmp(big.NewRat(13, 6)) != 0 || f.Holes[1].Clearance.BoxNormSquaredUpper.Cmp(big.NewRat(41, 100)) != 0 || f.Holes[1].Clearance.ClearanceLower.Cmp(big.NewRat(177, 100)) != 0 {
		t.Fatal("independent containment bounds")
	}

	for i, h := range f.Holes {
		for _, s := range []float64{0, .125, .375, .5, .75, 1} {
			uv, e := f.Face.Holes[i][0].Curve.EvalUV(s)
			if e != nil {
				t.Fatal(e)
			}
			p, e := f.Face.Surface.Eval(uv.X, uv.Y)
			if e != nil {
				t.Fatal(e)
			}
			q, e := h.Boundary.Eval(2 * math.Pi * (1 - s))
			if e != nil || p.Distance(q) > 2e-14 {
				t.Fatal("owner lift", i, p, q, e)
			}
		}
	}
	var source bytes.Buffer
	if e = WriteTwoEllipticalHoleFaceSource(&source, f.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	again, e := ReadTwoEllipticalHoleFace(&source, Limits{})
	if e != nil || !reflect.DeepEqual(again, f) {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteTwoEllipticalHoleFaceJSON(&native, f); e != nil {
		t.Fatal(e)
	}
	data := append([]byte(nil), native.Bytes()...)
	back, e := ReadTwoEllipticalHoleFaceJSON(&native, Limits{})
	if e != nil || !reflect.DeepEqual(back, f) {
		t.Fatal(e)
	}
	native.Reset()
	if e = WriteTwoEllipticalHoleFaceJSON(&native, back); e != nil || !bytes.Equal(data, native.Bytes()) {
		t.Fatal("native unstable", e)
	}
	if _, e = ReadEccentricEllipticalFace(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("single-hole gate widened")
	}
}
func TestTwoEllipticalHoleFaceReject(t *testing.T) {
	raw := twoEllipseFixture(t)
	for _, m := range [][2]string{{"#18,#42,#61", "#18,#42,#42"}, {"#68,(#73)", "#68,(#54)"}, {"#73 = PCURVE('',#31,#74)", "#73 = PCURVE('',#25,#74)"}, {"#65 = VERTEX_POINT('',#66)", "#65 = VERTEX_POINT('',#47)"}, {"#61 = FACE_BOUND('',#62,.F.)", "#61 = FACE_BOUND('',#62,.T.)"}, {"#75),#79", "#56),#79"}, {"#68 = ELLIPSE('',#69,2.,1.)", "#68 = ELLIPSE('',#69,2.,1.00000000000000001)"}} {
		b := bytes.Replace(raw, []byte(m[0]), []byte(m[1]), 1)
		if bytes.Equal(raw, b) {
			t.Fatal("stale mutation", m)
		}
		f, e := ReadTwoEllipticalHoleFace(bytes.NewReader(b), Limits{})
		if e == nil || f.Source != nil || f.SeparationLower != nil {
			t.Fatal(m, e)
		}
	}
	f, e := ReadTwoEllipticalHoleFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteTwoEllipticalHoleFaceJSON(&b, f); e != nil {
		t.Fatal(e)
	}
	data := append([]byte(nil), b.Bytes()...)
	for _, change := range []func(*twoEllipseNative){func(n *twoEllipseNative) { n.SeparationLower = "100" }, func(n *twoEllipseNative) { n.Parts[1].Face.InnerAncestry = n.Parts[0].Face.InnerAncestry }, func(n *twoEllipseNative) { n.Parts[1].Face.Hole = n.Parts[0].Face.Hole }, func(n *twoEllipseNative) { n.Version = 2 }} {
		var n twoEllipseNative
		if e = json.Unmarshal(data, &n); e != nil {
			t.Fatal(e)
		}
		change(&n)
		r, e := json.Marshal(n)
		if e != nil {
			t.Fatal(e)
		}
		if f, e := ReadTwoEllipticalHoleFaceJSON(bytes.NewReader(r), Limits{}); e == nil || f.Source != nil {
			t.Fatal("native forgery")
		}
	}
	f.Face.Holes[1][0].Edge = 1
	b.Reset()
	if e = WriteTwoEllipticalHoleFaceJSON(&b, f); e == nil || b.Len() != 0 {
		t.Fatal("colliding native edge roles")
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 84}, {MaxValues: 20}} {
		if _, e = ReadTwoEllipticalHoleFace(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
}
func twoEllipseMoveSecond(t testing.TB, c geom.Vec2) []byte {
	t.Helper()
	f, e := Parse(bytes.NewReader(twoEllipseFixture(t)), Limits{})
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
	put(77, []float64{c.X, c.Y})
	put(70, []float64{1 - c.Y, 2, 3 - c.X})
	put(66, []float64{-1 - c.Y, 2, 3 - c.X})
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestTwoEllipticalHoleSeparation(t *testing.T) {
	for _, tc := range []struct {
		c    geom.Vec2
		ok   bool
		axis int
		gap  *big.Rat
	}{{geom.Vec2{X: 3, Y: 1}, true, 0, big.NewRat(3, 1)}, {geom.Vec2{X: -3, Y: 4}, false, 0, nil}, {geom.Vec2{X: 0, Y: 1}, false, 0, nil}, {geom.Vec2{X: -2, Y: 1}, false, 0, nil}, {geom.Vec2{X: -3, Y: 3}, false, 0, nil}, {geom.Vec2{X: 0, Y: 3.5}, true, 1, big.NewRat(1, 2)}} {
		raw := twoEllipseMoveSecond(t, tc.c)
		f, e := ReadTwoEllipticalHoleFace(bytes.NewReader(raw), Limits{})
		if (e == nil) != tc.ok {
			t.Fatal(tc, e)
		}
		if e == nil && (f.SeparationAxis != tc.axis || f.SeparationLower.Cmp(tc.gap) != 0) {
			t.Fatal("gap", tc, f.SeparationAxis, f.SeparationLower)
		}
	}
	// Original bound order, not numeric entity ID, determines local hole roles.
	raw := bytes.Replace(twoEllipseFixture(t), []byte("#18,#42,#61"), []byte("#18,#61,#42"), 1)
	f, e := ReadTwoEllipticalHoleFace(bytes.NewReader(raw), Limits{})
	if e != nil || f.Holes[0].SourceBoundID != 61 || f.Holes[1].SourceBoundID != 42 {
		t.Fatal(e)
	}
}
func FuzzTwoEllipticalHoleFace(f *testing.F) {
	f.Add(twoEllipseFixture(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadTwoEllipticalHoleFace(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 85, MaxValues: 4096, MaxDepth: 16})
		if e != nil && (m.Source != nil || m.SeparationLower != nil) {
			t.Fatal("partial")
		}
	})
}
