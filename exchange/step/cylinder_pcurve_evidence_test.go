package step

import (
	"bytes"
	"math/big"
	"os"
	"testing"
)

func TestCylinderPCurveEvidenceOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := InspectCylinderPCurve(bytes.NewReader(raw), Limits{}, 21)
	if e != nil {
		t.Fatal(e)
	}
	if !r.IncreasingU || r.DecreasingU || r.PCurveID != 375 || r.ParameterCurveID != 382 || r.ModelCurveID != 25 || r.CylinderID != 376 || r.MasterRepresentation != "PCURVE_S1" || r.MasterPCurveID != 196 || r.ParameterStart.Sign() != 0 || r.ParameterEnd.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r)
	}
	if r.PositiveTurnDefect[0].Sign() <= 0 || r.PositiveTurnDefect[1].Cmp(big.NewRat(5, 10000000000000)) >= 0 {
		t.Fatal("uncertified one-turn angle defect")
	}
	t.Log("U range", r.UStart.RatString(), r.UEnd.RatString(), "spans", len(r.DerivativeSpans))
}
func TestCylinderPCurveEvidenceRejects(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*File){
		func(f *File) {
			en := f.Entities[375]
			en.Records[0].Args[1] = Value{Kind: Reference, Ref: 197}
			f.Entities[375] = en
		},
		func(f *File) {
			en := f.Entities[24]
			en.Records[0].Args[2].Items[1] = en.Records[0].Args[2].Items[0]
			f.Entities[24] = en
		},
		func(f *File) {
			en := f.Entities[20]
			en.Records[0].Args[4] = Value{Kind: Enumeration, Text: "T"}
			f.Entities[20] = en
		},
		func(f *File) {
			en := f.Entities[382]
			en.Records[0].Args[7].Items[0] = Value{Kind: Number, Text: "-1"}
			f.Entities[382] = en
		},
	} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		change(f)
		if _, e = InspectCylinderPCurve(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21); e == nil {
			t.Fatal("accepted malformed ownership/domain")
		}
	}
}

func TestPCurvePiEnclosure(t *testing.T) {
	lo, hi := pcurvePiBounds()
	if lo.Cmp(big.NewRat(314159, 100000)) <= 0 || hi.Cmp(big.NewRat(314160, 100000)) >= 0 || lo.Cmp(hi) >= 0 {
		t.Fatal(lo, hi)
	}
	width := new(big.Rat).Sub(hi, lo)
	limit, _ := new(big.Rat).SetString("1e-40")
	if width.Cmp(limit) >= 0 {
		t.Fatal("weak pi enclosure")
	}
}
func FuzzCylinderPCurveEvidence(f *testing.F) {
	b, _ := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = InspectCylinderPCurve(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 16384, MaxEdgeUses: 256}, 21)
	})
}

func TestCylinderPCurveDoesNotInferMatching(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	rec, e := simple(f, 382, "B_SPLINE_CURVE_WITH_KNOTS", 9)
	if e != nil {
		t.Fatal(e)
	}
	ids, e := listRefs(rec.Args[2], 1)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range ids {
		en := f.Entities[id]
		en.Records[0].Args[1].Items[0] = Value{Kind: Number, Text: "0."}
		f.Entities[id] = en
	}
	out, e := InspectCylinderPCurve(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 21)
	if e != nil || out.IncreasingU || out.DecreasingU || out.UStart.Sign() != 0 || out.UEnd.Sign() != 0 {
		t.Fatal(out, e)
	}
	// The associations remain structurally valid, but no monotonicity or model
	// correspondence is inferred from them. This API does not accept a body.
}
