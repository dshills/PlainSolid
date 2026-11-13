package cylindercells

import (
	"bytes"
	"errors"
	"math"
	"math/big"
	"testing"
)

func physical(v float64, u SIUnit) *big.Rat {
	q := new(big.Rat).SetFloat64(v)
	p := u.MetresExponent
	n := p
	if n < 0 {
		n = -n
	}
	f := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil))
	if p < 0 {
		f.Inv(f)
	}
	return q.Mul(q, f)
}
func TestCylinderAssetExactRescale(t *testing.T) {
	for _, dialect := range []string{Half, Quarter} {
		a, e := FromNative(native(t), "physical part", SIUnit{0}, dialect)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteSTEP(&b, a); e != nil {
			t.Fatal(e)
		}
		a, e = FromSTEP(&b, dialect)
		if e != nil {
			t.Fatal(e)
		}
		before := a
		same, e := RescaleSI(a, a.Unit)
		if e != nil || same != a || same.SourceSTEP == "" {
			t.Fatal("same-unit evidence", e)
		}
		scaled, e := RescaleSI(a, SIUnit{-3})
		if e != nil {
			t.Fatal(e)
		}
		if scaled.SourceSTEP != "" || scaled.Name != a.Name || scaled.Reference != a.Reference || scaled.Axis != a.Axis || a != before {
			t.Fatal("metadata/provenance mutation")
		}
		old := []float64{a.Origin.X, a.Origin.Y, a.Origin.Z, a.Radius, a.Height}
		now := []float64{scaled.Origin.X, scaled.Origin.Y, scaled.Origin.Z, scaled.Radius, scaled.Height}
		for i := range old {
			if physical(old[i], a.Unit).Cmp(physical(now[i], scaled.Unit)) != 0 {
				t.Fatal("physical scalar changed", i)
			}
		}
		restored, e := RescaleSI(scaled, a.Unit)
		if e != nil {
			t.Fatal(e)
		}
		want := a
		want.SourceSTEP = ""
		if restored != want {
			t.Fatal("nonexact reverse")
		}
		if _, e = scaled.Validate(); e != nil {
			t.Fatal(e)
		}
		if e = WriteNative(&b, scaled); e != nil {
			t.Fatal(e)
		}
		saved, e := ReadNative(&b)
		if e != nil || saved != scaled {
			t.Fatal("persistence", e)
		}
		if e = WriteOriginalSTEP(&b, saved); !errors.Is(e, ErrNoSource) {
			t.Fatal("fabricated source", e)
		}
		if e = WriteOriginalSTEP(&b, a); e != nil || b.String() != a.SourceSTEP {
			t.Fatal("old evidence unavailable", e)
		}
	}
}
func TestCylinderAssetRescaleRejectsRoundingBeforeEvidence(t *testing.T) {
	a, e := FromNative(native(t), "part", SIUnit{-3}, Half)
	if e != nil {
		t.Fatal(e)
	}
	// 2/1000 is not binary64-exact. A deliberately invalid source must not be
	// parsed before this scalar failure; the specific sentinel proves ordering.
	a.SourceSTEP = "invalid source"
	if got, e := RescaleSI(a, SIUnit{0}); !errors.Is(e, ErrUnrepresentableScale) || got.Dialect != "" {
		t.Fatal(got, e)
	}
	for _, v := range []float64{math.MaxFloat64, math.SmallestNonzeroFloat64} {
		bad := a
		bad.Origin.X = v
		target := SIUnit{-18}
		bad.Unit = SIUnit{18}
		if v == math.SmallestNonzeroFloat64 {
			target = SIUnit{18}
			bad.Unit = SIUnit{-18}
		}
		if got, e := RescaleSI(bad, target); !errors.Is(e, ErrUnrepresentableScale) || got.Dialect != "" {
			t.Fatal("range", e)
		}
	}
	if _, e = RescaleSI(a, SIUnit{4}); e == nil {
		t.Fatal("unsupported SI prefix")
	}
	if _, e = RescaleSI(a, a.Unit); e == nil || errors.Is(e, ErrUnrepresentableScale) {
		t.Fatal("same-unit stale evidence not validated", e)
	}
}
