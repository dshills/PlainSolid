package step

import (
	"bytes"
	"math/big"
	"os"
	"testing"
)

func TestGeometricTangentComposite(t *testing.T) {
	raw, e := os.ReadFile("testdata/geometric-tangent-composite-curve.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCompositeCurves(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("positional widened")
	}
	if _, e = ReadTangentCompositeCurves(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("C1 widened")
	}
	m, e := ReadGeometricTangentCompositeCurves(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := compositeEndpointDerivative(m.Curves[0].Segments[0], true)
	b, _ := compositeEndpointDerivative(m.Curves[0].Segments[1], false)
	if a[0].Cmp(big.NewRat(3, 2)) != 0 || b[0].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(a, b)
	}
	var out bytes.Buffer
	if e = WriteGeometricTangentCompositeCurves(&out, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadGeometricTangentCompositeCurves(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteTangentCompositeCurves(&bytes.Buffer{}, m.TangentCompositeCurveModel); e == nil {
		t.Fatal("C1 writer widened")
	}
	if _, e = ReadGeometricTangentCompositeCurves(bytes.NewReader(raw), Limits{MaxVertices: 3}); e == nil {
		t.Fatal("limit")
	}
	for _, replacement := range []string{"(1.5,0.,0.)", "(0.5,0.,0.)"} {
		altered := bytes.ReplaceAll(raw, []byte("(2.5,0.,0.)"), []byte(replacement))
		if bytes.Equal(altered, raw) {
			t.Fatal("stale zero/opposed mutation")
		}
		if result, err := ReadGeometricTangentCompositeCurves(bytes.NewReader(altered), Limits{}); err == nil || len(result.Curves) != 0 {
			t.Fatal("zero/opposed source accepted", err)
		}
	}
	// A tiny represented perturbation breaks exact direction, without a join gap.
	bad := bytes.ReplaceAll(raw, []byte("(2.5,0.,0.)"), []byte("(2.5,0.0000000000000001,0.)"))
	if bytes.Equal(bad, raw) {
		t.Fatal("mutation stale")
	}
	r, e := ReadGeometricTangentCompositeCurves(bytes.NewReader(bad), Limits{})
	if e == nil || len(r.Curves) != 0 {
		t.Fatal("near tangent accepted", e)
	}
}
func TestGeometricTangentProportionalPredicate(t *testing.T) {
	vec := func(x, y, z int64) [3]*big.Rat {
		return [3]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), big.NewRat(z, 1)}
	}
	for _, tc := range []struct {
		a, b [3]*big.Rat
		want bool
	}{
		{vec(1, -2, 3), vec(2, -4, 6), true}, {vec(0, -2, 3), vec(0, -4, 6), true},
		{vec(1, -2, 3), vec(-2, 4, -6), false}, {vec(1, -2, 3), vec(2, -4, 7), false},
		{vec(0, 0, 0), vec(0, 0, 0), false}, {vec(1, 0, 0), vec(0, 0, 0), false},
	} {
		if got := positiveProportionalCompositeTangents(tc.a, tc.b); got != tc.want {
			t.Fatal(got, tc)
		}
	}
}
func FuzzGeometricTangentComposite(f *testing.F) {
	raw, e := os.ReadFile("testdata/geometric-tangent-composite-curve.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		m, e := ReadGeometricTangentCompositeCurves(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 256, MaxValues: 4096, MaxDepth: 12, MaxVertices: 128})
		if e != nil {
			if len(m.Curves) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteGeometricTangentCompositeCurves(&out, m); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadGeometricTangentCompositeCurves(&out, Limits{}); e != nil {
			t.Fatal(e)
		}
	})
}
