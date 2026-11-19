package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCylinderLiftAgreementOCCT(t *testing.T) {
	slowtest.Skip(t)
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundCylinderPCurveLift(bytes.NewReader(raw), Limits{}, 21, 4e-8)
	if e != nil {
		t.Fatal(e)
	}
	if !r.WithinTolerance || r.Spans != 28 {
		t.Fatal("unproved tolerance", r.DistanceSquaredUpper.FloatString(20))
	}
	strict, e := BoundCylinderPCurveLift(bytes.NewReader(raw), Limits{}, 21, 1e-8)
	if e != nil || strict.WithinTolerance || strict.DistanceSquaredUpper.Cmp(r.DistanceSquaredUpper) != 0 {
		t.Fatal("tolerance decision", e)
	}
	// Independently cycle all represented model-space coordinates. Local
	// pcurves remain unchanged; a signed-coordinate frame must preserve bounds.
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		rec := &en.Records[0]
		if (rec.Type == "CARTESIAN_POINT" || rec.Type == "DIRECTION") && len(rec.Args) == 2 && len(rec.Args[1].Items) == 3 {
			v := rec.Args[1].Items
			rec.Args[1].Items = []Value{v[2], v[0], v[1]}
			f.Entities[id] = en
		}
	}
	posed, e := BoundCylinderPCurveLift(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21, 4e-8)
	if e != nil || !posed.WithinTolerance || posed.DistanceSquaredUpper.Cmp(r.DistanceSquaredUpper) != 0 {
		t.Fatal("frame changed exact enclosure", e)
	}
	for i, q := range r.CoordinateBounds {
		t.Log(i, q.FloatString(15))
	}
}
func TestLiftTrigIndependent(t *testing.T) {
	t.Parallel()
	for _, u := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(-1, 2)} {
		s, c, errBound, e := liftTrigBernstein([]*big.Rat{u, u})
		if e != nil {
			t.Fatal(e)
		}
		if errBound.Sign() < 0 || len(s) != 11 || len(c) != 11 {
			t.Fatal("invalid enclosure")
		}
		// Independent higher-order Maclaurin sums enclose the true constant
		// values more tightly than the implementation's center enclosure.
		if u.Sign() != 0 {
			sinRef, cosRef := new(big.Rat), new(big.Rat)
			for k := 0; k <= 80; k++ {
				num := new(big.Int).Exp(u.Num(), big.NewInt(int64(k)), nil)
				den := new(big.Int).Exp(u.Denom(), big.NewInt(int64(k)), nil)
				fact := big.NewInt(1)
				for j := 2; j <= k; j++ {
					fact.Mul(fact, big.NewInt(int64(j)))
				}
				den.Mul(den, fact)
				term := new(big.Rat).SetFrac(num, den)
				switch k % 4 {
				case 0:
					cosRef.Add(cosRef, term)
				case 1:
					sinRef.Add(sinRef, term)
				case 2:
					cosRef.Sub(cosRef, term)
				case 3:
					sinRef.Sub(sinRef, term)
				}
			}
			refError := big.NewRat(1, 1)
			for j := 1; j <= 81; j++ {
				refError.Mul(refError, new(big.Rat).Abs(u))
				refError.Quo(refError, big.NewRat(int64(j), 1))
			}
			for i, expected := range []*big.Rat{sinRef, cosRef} {
				actual := s[0]
				if i == 1 {
					actual = c[0]
				}
				if new(big.Rat).Sub(actual, errBound).Cmp(new(big.Rat).Sub(expected, refError)) > 0 || new(big.Rat).Add(actual, errBound).Cmp(new(big.Rat).Add(expected, refError)) < 0 {
					t.Fatal("independent trigonometric enclosure mismatch")
				}
			}
		}
		if u.Sign() == 0 {
			for i := range s {
				if s[i].Sign() != 0 || c[i].Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("zero angle not exact")
				}
			}
		}
	}
	// Bernstein product of t and t is [0,0,1]; degree elevation of t is [0,1/2,1].
	p := liftProduct([]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)})
	if p[0].Sign() != 0 || p[1].Sign() != 0 || p[2].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(p)
	}
	q := liftElevate([]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, 2)
	if q[1].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(q)
	}
}

func TestCylinderLiftRejectsAndMismatch(t *testing.T) {
	slowtest.Skip(t)
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	pc, e := simple(f, 382, "B_SPLINE_CURVE_WITH_KNOTS", 9)
	if e != nil {
		t.Fatal(e)
	}
	ids, e := listRefs(pc.Args[2], 1)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range ids {
		en := f.Entities[id]
		v := en.Records[0].Args[1].Items[1]
		q, e := liftSourceRat(v)
		if e != nil {
			t.Fatal(e)
		}
		q.Add(q, big.NewRat(1, 1000))
		en.Records[0].Args[1].Items[1] = Value{Kind: Number, Text: q.FloatString(15)}
		f.Entities[id] = en
	}
	r, e := BoundCylinderPCurveLift(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21, 1e-6)
	if e != nil || r.WithinTolerance || r.CoordinateBounds[2].Cmp(big.NewRat(1, 1001)) < 0 {
		t.Fatal("changed lifted height escaped", e)
	}
	f, e = Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	en := f.Entities[382]
	en.Records[0].Args[7].Items[1] = Value{Kind: Number, Text: "0.08"}
	f.Entities[382] = en
	if _, e = BoundCylinderPCurveLift(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21, 1e-6); e == nil {
		t.Fatal("mismatched knots accepted")
	}
	f, e = Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	angle := f.Entities[640]
	for i := range angle.Records {
		if angle.Records[i].Type == "SI_UNIT" {
			angle.Records[i].Args[0] = Value{Kind: Enumeration, Text: "MILLI"}
		}
	}
	f.Entities[640] = angle
	if _, e = BoundCylinderPCurveLift(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21, 1e-6); e == nil {
		t.Fatal("angle scaling silently ignored")
	}
	if _, e = liftSourceRat(Value{Kind: Number, Text: "1e-999999999"}); e == nil {
		t.Fatal("unbounded exponent")
	}
	if _, _, _, e = liftTrigBernstein([]*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}); e == nil {
		t.Fatal("wide angle span accepted")
	}
}
func FuzzCylinderLiftAgreement(f *testing.F) {
	b, _ := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	f.Add(b)
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = BoundCylinderPCurveLift(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 16384, MaxEdgeUses: 256}, 21, 1e-6)
	})
}
