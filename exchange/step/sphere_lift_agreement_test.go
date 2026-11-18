package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestSphereLiftAgreementOCCT(t *testing.T) {
	slowtest.Skip(t)
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundSpherePCurveLift(bytes.NewReader(raw), Limits{}, 21, 4e-7)
	if e != nil {
		t.Fatal(e)
	}
	if !r.WithinTolerance || r.Spans != 28 || r.PCurveID != 196 || r.ParameterCurveID != 203 || r.SphereID != 197 {
		t.Fatal("unproved tolerance", r.DistanceSquaredUpper.FloatString(20))
	}
	for i, q := range r.CoordinateBounds {
		t.Log(i, q.FloatString(15))
	}
	if r.DistanceSquaredUpper.Cmp(big.NewRat(1, 100000000000000)) <= 0 {
		t.Fatal("bound unexpectedly certifies 1e-7")
	}
}

func TestSphereLiftMismatchAndSubset(t *testing.T) {
	slowtest.Skip(t)
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	rec, e := simple(f, 203, "B_SPLINE_CURVE_WITH_KNOTS", 9)
	if e != nil {
		t.Fatal(e)
	}
	ids, e := listRefs(rec.Args[2], 1)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range ids {
		en := f.Entities[id]
		q, e := liftSourceRat(en.Records[0].Args[1].Items[1])
		if e != nil {
			t.Fatal(e)
		}
		q.Add(q, big.NewRat(1, 1000))
		en.Records[0].Args[1].Items[1] = Value{Kind: Number, Text: q.FloatString(15)}
		f.Entities[id] = en
	}
	out, e := BoundSpherePCurveLift(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21, 1e-6)
	if e != nil || out.WithinTolerance {
		t.Fatal("changed latitude certified", e)
	}
	f, e = Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	en := f.Entities[203]
	en.Records[0].Args[7].Items[1] = Value{Kind: Number, Text: "0.08"}
	f.Entities[203] = en
	if _, e = BoundSpherePCurveLift(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21, 1e-6); e == nil {
		t.Fatal("mismatched knots accepted")
	}
}
func TestSphereLiftFrameAndTaylor(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		r := &en.Records[0]
		if (r.Type == "CARTESIAN_POINT" || r.Type == "DIRECTION") && len(r.Args) == 2 && len(r.Args[1].Items) == 3 {
			v := r.Args[1].Items
			r.Args[1].Items = []Value{v[2], v[0], v[1]}
			f.Entities[id] = en
		}
	}
	_, axes, e := sphereLiftFrame(f, 198)
	if e != nil {
		t.Fatal(e)
	}
	for a, want := range []int{1, 2, 0} {
		for c := 0; c < 3; c++ {
			expected := int64(0)
			if c == want {
				expected = 1
			}
			if axes[a][c].Cmp(big.NewRat(expected, 1)) != 0 {
				t.Fatal("frame permutation")
			}
		}
	}
	s, c, bound, e := liftTrigBernsteinOrder([]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1)}, 6)
	if e != nil || len(s) != 7 || len(c) != 7 || bound.Sign() != 0 {
		t.Fatal(e)
	}
	for i := range s {
		if s[i].Sign() != 0 || c[i].Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("zero-angle lift")
		}
	}
	if _, _, _, e = liftTrigBernsteinOrder([]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1)}, 11); e == nil {
		t.Fatal("unbounded Taylor order")
	}
}
func FuzzSphereLiftAgreement(f *testing.F) {
	b, _ := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	f.Add(b)
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = BoundSpherePCurveLift(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 16384, MaxEdgeUses: 256}, 21, 1e-6)
	})
}

func TestSphereLiftExactConstantIdentity(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	// Algebraically independent known lift: U=V=0 maps to (R,0,0).
	// The rest of this reused ownership scaffold is intentionally not a body
	// acceptance fixture; endpoint and cylinder agreement are separate questions.
	for _, curve := range []int64{25, 203} {
		r, e := simple(f, curve, "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if e != nil {
			t.Fatal(e)
		}
		ids, e := listRefs(r.Args[2], 1)
		if e != nil {
			t.Fatal(e)
		}
		for _, id := range ids {
			en := f.Entities[id]
			for c := range en.Records[0].Args[1].Items {
				value := "0."
				if curve == 25 && c == 0 {
					value = "5."
				}
				en.Records[0].Args[1].Items[c] = Value{Kind: Number, Text: value}
			}
			f.Entities[id] = en
		}
	}
	out, e := BoundSpherePCurveLift(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21, 1e-30)
	if e != nil || !out.WithinTolerance || out.DistanceSquaredUpper.Sign() != 0 {
		t.Fatal("exact spherical lift identity", e)
	}
}
