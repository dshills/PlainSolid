package step

import (
	"bytes"
	"math/big"
	"os"
	"testing"
)

func TestOffAxisRimLiftPair(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundOffAxisRimLiftAgreement(bytes.NewReader(raw), Limits{}, 15, 5e-7)
	if e != nil {
		t.Fatal(e)
	}
	if !r.WithinTolerance || !r.FullDomainSameParameter || r.AuthoredTrimSelectionProven || r.RepairedBoundaryAgreementProven || r.MaterialEquivalenceProven || r.EdgeID != 21 || r.ModelCurveID != 25 || r.SpherePCurveID != 196 || r.CylinderPCurveID != 375 || r.SphereSpans != 28 || r.CylinderSpans != 28 {
		t.Fatal(r)
	}
	sum := new(big.Rat)
	for i, b := range r.WorldCoordinateBounds {
		if b.Cmp(new(big.Rat).Add(r.SphereWorldCoordinateBounds[i], r.CylinderWorldCoordinateBounds[i])) != 0 {
			t.Fatal("component binding")
		}
		sum.Add(sum, new(big.Rat).Mul(b, b))
	}
	if sum.Cmp(r.DistanceSquaredUpper) != 0 {
		t.Fatal("norm")
	}
	t.Log("squared gap bound", r.DistanceSquaredUpper.FloatString(25))
	tight, e := BoundOffAxisRimLiftAgreement(bytes.NewReader(raw), Limits{}, 15, 1e-8)
	if e != nil || tight.WithinTolerance || tight.DistanceSquaredUpper.Cmp(r.DistanceSquaredUpper) != 0 {
		t.Fatal("tolerance changed proof", e)
	}
	for _, pair := range [][2]string{
		{"#196 = PCURVE('',#197,#202);", "#196 = PCURVE('',#376,#202);"},
		{"#201 = DIRECTION('',(1.,0.,0.));", "#201 = DIRECTION('',(1.,1.,0.));"},
	} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(raw, bad) {
			t.Fatal("stale mutation")
		}
		if _, e := BoundOffAxisRimLiftAgreement(bytes.NewReader(bad), Limits{}, 15, 1e-6); e == nil {
			t.Fatal("invalid source", pair)
		}
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	en := f.Entities[203]
	en.Records[0].Args[7].Items[1] = Value{Kind: Number, Text: "0.08"}
	f.Entities[203] = en
	if _, e := BoundOffAxisRimLiftAgreement(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 15, 1e-6); e == nil {
		t.Fatal("domain/knot mismatch")
	}
	if _, e := BoundOffAxisRimLiftAgreement(bytes.NewReader(raw), Limits{MaxEntities: 16}, 15, 1e-6); e == nil {
		t.Fatal("entity budget")
	}
	if _, e := BoundOffAxisRimLiftAgreement(bytes.NewReader(raw), Limits{MaxBytes: 16}, 15, 1e-6); e == nil {
		t.Fatal("byte budget")
	}
}

func TestOffAxisRimLiftPairIndependentFrames(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	// Rotate only the sphere reference X axis by +90 degrees. Its local residual
	// axes now differ from the cylinder's; no global frame alignment is assumed.
	changed := bytes.Replace(raw, []byte("#201 = DIRECTION('',(1.,0.,0.));"), []byte("#201 = DIRECTION('',(0.,1.,0.));"), 1)
	if bytes.Equal(changed, raw) {
		t.Fatal("stale frame")
	}
	changed = compactLiftPairFixture(t, changed)
	pair, e := BoundOffAxisRimLiftAgreement(bytes.NewReader(changed), Limits{}, 15, 1e-6)
	if e != nil || pair.WithinTolerance {
		t.Fatal("rotated owner certified", e)
	}
	owner, e := BoundSpherePCurveLift(bytes.NewReader(changed), Limits{}, 21, 1e-6)
	if e != nil {
		t.Fatal(e)
	}
	for world, local := range []int{1, 0, 2} {
		if pair.SphereWorldCoordinateBounds[world].Cmp(owner.CoordinateBounds[local]) != 0 {
			t.Fatal("local frame residual used as world coordinate")
		}
	}
	// A moved pcurve still has valid ownership, but cannot reuse the original
	// small paired tolerance merely because both records reference model #25.
	shifted := bytes.Replace(raw, []byte("#204 = CARTESIAN_POINT('',(0.,0.927295218002));"), []byte("#204 = CARTESIAN_POINT('',(0.,1.1));"), 1)
	shifted = compactLiftPairFixture(t, shifted)
	q, e := BoundOffAxisRimLiftAgreement(bytes.NewReader(shifted), Limits{}, 15, 1e-6)
	if e != nil || q.WithinTolerance {
		t.Fatal("changed latitude certified", e)
	}
}

// One constant span retains the real ownership graph while isolating frame
// composition from the expensive 28-span trigonometric approximation fixture.
func compactLiftPairFixture(t *testing.T, raw []byte) []byte {
	t.Helper()
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []int64{25, 203, 382} {
		en := f.Entities[id]
		a := en.Records[0].Args
		a[1] = Value{Kind: Number, Text: "1"}
		first, _ := ref(a[2].Items[0])
		second, _ := ref(a[2].Items[1])
		point := f.Entities[second]
		point.Records[0].Args[1] = f.Entities[first].Records[0].Args[1]
		f.Entities[second] = point
		a[2].Items = []Value{a[2].Items[0], a[2].Items[1]}
		a[6].Items = []Value{{Kind: Number, Text: "2"}, {Kind: Number, Text: "2"}}
		a[7].Items = []Value{{Kind: Number, Text: "0"}, {Kind: Number, Text: "1"}}
		f.Entities[id] = en
	}
	out := serializeWrappedProofFile(f)
	if _, e := InspectCylinderPCurve(bytes.NewReader(out), Limits{}, 21); e != nil {
		t.Fatal("compact", e)
	}
	return out
}
