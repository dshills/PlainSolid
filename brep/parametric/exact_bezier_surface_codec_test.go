package parametric

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/geom"
	"strings"
	"testing"
)

func exactNativeSurfaceFixture(t *testing.T) *geom.ExactBezierSurface {
	t.Helper()
	arc := [][3]*big.Rat{{big.NewRat(7, 1), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(7, 1), big.NewRat(26, 3), big.NewRat(1, 1)}, {big.NewRat(65, 3), big.NewRat(52, 3), big.NewRat(13, 9)}}
	var h [][4]*big.Rat
	for _, p := range arc {
		for _, z := range []int64{0, 3} {
			h = append(h, [4]*big.Rat{p[0], p[1], new(big.Rat).Mul(big.NewRat(z, 1), p[2]), p[2]})
		}
	}
	patch, e := geom.NewExactHomogeneousBezierSurface(context.Background(), 2, 1, h, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	surface, e := geom.NewExactBezierSurface(patch)
	if e != nil {
		t.Fatal(e)
	}
	return surface
}
func TestExactBezierSurfaceNativeCodec(t *testing.T) {
	source := exactNativeSurfaceFixture(t)
	raw, e := EncodeSurface(source)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(raw), "26/3") {
		t.Fatal("coefficient rounded")
	}
	decoded, e := DecodeSurface(raw, 6)
	if e != nil {
		t.Fatal(e)
	}
	back, ok := decoded.(*geom.ExactBezierSurface)
	if !ok {
		t.Fatalf("lost exact association: %T", decoded)
	}
	for _, uv := range [][2]float64{{0, 0}, {.375, .25}, {1, 1}} {
		a, e := source.Eval(uv[0], uv[1])
		if e != nil {
			t.Fatal(e)
		}
		b, e := back.Eval(uv[0], uv[1])
		if e != nil || a != b {
			t.Fatal(a, b, e)
		}
		au, av, e := source.Partials(uv[0], uv[1])
		if e != nil {
			t.Fatal(e)
		}
		bu, bv, e := back.Partials(uv[0], uv[1])
		if e != nil || au != bu || av != bv {
			t.Fatal(au, av, bu, bv, e)
		}
	}
	a, e := source.ExactPatch().EvaluateRational(context.Background(), big.NewRat(1, 3), big.NewRat(2, 7), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := back.ExactPatch().EvaluateRational(context.Background(), big.NewRat(1, 3), big.NewRat(2, 7), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if a.Point[i].Cmp(b.Point[i]) != 0 || a.Du[i].Cmp(b.Du[i]) != 0 {
			t.Fatal(a, b)
		}
	}
	if _, e = back.ExactPatch().ToBSplineSurface(); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal(e)
	}
	// Native placement keeps the exact support as its base rather than flattening
	// it into a rounded spline or treating transformed UV as new model controls.
	mapped := MapSurface(source, geom.Translation(geom.Vec3{X: 2, Y: -3, Z: 4}))
	encoded, e := EncodeSurface(mapped)
	if e != nil {
		t.Fatal(e)
	}
	restored, e := DecodeSurface(encoded, 6)
	if e != nil {
		t.Fatal(e)
	}
	base, _ := SurfaceMapping(restored)
	if _, ok := base.(*geom.ExactBezierSurface); !ok {
		t.Fatalf("mapped association %T", base)
	}
	expected, e := mapped.Eval(.25, .75)
	if e != nil {
		t.Fatal(e)
	}
	actual, e := restored.Eval(.25, .75)
	if e != nil || actual != expected {
		t.Fatal(actual, expected, e)
	}
	if s, e := DecodeSurface(raw, 5); e == nil || s != nil {
		t.Fatal("control budget", s, e)
	}
}
func TestExactBezierSurfaceNativeCodecRejects(t *testing.T) {
	source := exactNativeSurfaceFixture(t)
	raw, e := EncodeSurface(source)
	if e != nil {
		t.Fatal(e)
	}
	var original map[string]json.RawMessage
	if e = json.Unmarshal(raw, &original); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(map[string]json.RawMessage){
		func(m map[string]json.RawMessage) { m["Kind"] = json.RawMessage(`"bspline"`) },
		func(m map[string]json.RawMessage) { m["A"] = json.RawMessage(`1`) },
		func(m map[string]json.RawMessage) { m["Extra"] = json.RawMessage(`1`) },
		func(m map[string]json.RawMessage) {
			m["ExactBezier"] = json.RawMessage(strings.Replace(string(m["ExactBezier"]), `"26/3"`, `"1e1000000"`, 1))
		},
		func(m map[string]json.RawMessage) {
			var x map[string]any
			_ = json.Unmarshal(m["ExactBezier"], &x)
			x["DegreeU"] = 17
			m["ExactBezier"], _ = json.Marshal(x)
		},
		func(m map[string]json.RawMessage) {
			var x map[string]any
			_ = json.Unmarshal(m["ExactBezier"], &x)
			rows := x["Controls"].([]any)
			rows[0] = append(rows[0].([]any), "9")
			m["ExactBezier"], _ = json.Marshal(x)
		},
		func(m map[string]json.RawMessage) {
			var x map[string]any
			_ = json.Unmarshal(m["ExactBezier"], &x)
			rows := x["Controls"].([]any)
			rows[0].([]any)[3] = "-1"
			m["ExactBezier"], _ = json.Marshal(x)
		},
	} {
		m := map[string]json.RawMessage{}
		for k, v := range original {
			m[k] = append(json.RawMessage(nil), v...)
		}
		change(m)
		input, _ := json.Marshal(m)
		s, e := DecodeSurface(input, 1000)
		if e == nil || s != nil {
			t.Fatal("malformed accepted", string(input), e)
		}
	}
	var data exactBezierSurfaceData
	if e = json.Unmarshal([]byte(`{"Controls":"`+strings.Repeat("x", exactSurfaceMaxBytes)+`"}`), &data); e == nil {
		t.Fatal("payload cap ignored")
	}
	if s, e := geom.NewExactBezierSurface(nil); e == nil || s != nil {
		t.Fatal(s, e)
	}
}
