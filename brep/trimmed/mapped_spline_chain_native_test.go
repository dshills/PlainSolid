package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/brep/edgecheck"
	"reflect"
	"testing"
)

func TestMappedSplineChainNativeBoundary(t *testing.T) {
	ctx := context.Background()
	a, ca := mappedChainFixture(t, false)
	b, cb := mappedChainFixture(t, true)
	proof, e := CertifyMappedSplineChainAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = proof.WriteJSON(ctx, &encoded, MappedChainNativeOptions{}); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadMappedSplineChainBoundaryJSON(ctx, bytes.NewReader(encoded.Bytes()), MappedChainNativeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = loaded.WriteJSON(ctx, &again, MappedChainNativeOptions{}); e != nil || !bytes.Equal(encoded.Bytes(), again.Bytes()) {
		t.Fatal("deterministic reload", e)
	}
	for owner := 0; owner < 2; owner++ {
		if !sameAuthoredBezierSpline(loaded.sources[owner].parts[0].source.Curve, proof.sources[owner].parts[0].source.Curve) {
			t.Fatal("source poles lost")
		}
		want, _ := proof.AuthoredParameters(owner)
		got, _ := loaded.AuthoredParameters(owner)
		for i := range want {
			if want[i].Cmp(got[i]) != 0 {
				t.Fatal("raw station identity")
			}
		}
	}
	before, e := proof.TessellateCertified(ctx, MappedChainPolylineOptions{Deflection: .005})
	if e != nil {
		t.Fatal(e)
	}
	after, e := loaded.TessellateCertified(ctx, MappedChainPolylineOptions{Deflection: .005})
	if e != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("certified ledger changed", e)
	}
	mutations := []struct {
		name   string
		mutate func(*nativeMappedChainBoundary)
	}{
		{"speed", func(n *nativeMappedChainBoundary) { n.Owners[1].Weights[0] = "1/3" }},
		{"station", func(n *nativeMappedChainBoundary) { n.Owners[1].Stations[0] = "1/8" }},
		{"gap", func(n *nativeMappedChainBoundary) { n.Owners[0].Ranges[1].Min += .125 }},
		{"omitted tail", func(n *nativeMappedChainBoundary) { n.Owners[0].Ranges[1].Max -= .125 }},
		{"reverse", func(n *nativeMappedChainBoundary) { n.Owners[1].Source.SplineReversed = false }},
		{"support", func(n *nativeMappedChainBoundary) { n.Owners[1].Surface = n.Owners[0].Surface }},
		{"nested source", func(n *nativeMappedChainBoundary) { n.Owners[0].Source.Flip = true }},
		{"surplus map", func(n *nativeMappedChainBoundary) { n.Owners[0].Weights = append(n.Owners[0].Weights, "1") }},
		{"missing owner", func(n *nativeMappedChainBoundary) { n.Owners = n.Owners[:1] }},
	}
	for _, v := range mutations {
		t.Run(v.name, func(t *testing.T) {
			var n nativeMappedChainBoundary
			if e := json.Unmarshal(encoded.Bytes(), &n); e != nil {
				t.Fatal(e)
			}
			v.mutate(&n)
			raw, e := json.Marshal(n)
			if e != nil {
				t.Fatal(e)
			}
			out, e := ReadMappedSplineChainBoundaryJSON(ctx, bytes.NewReader(raw), MappedChainNativeOptions{})
			if e == nil || len(out.ModelCuts()) != 0 {
				t.Fatal("mutation accepted")
			}
		})
	}
	for _, o := range []MappedChainNativeOptions{{MaxBytes: 10}, {Proof: edgecheck.Options{MaxWork: 1}}, {Proof: edgecheck.Options{MaxIntervals: 2}}} {
		out, e := ReadMappedSplineChainBoundaryJSON(ctx, bytes.NewReader(encoded.Bytes()), o)
		if e == nil || len(out.ModelCuts()) != 0 {
			t.Fatal("read budget")
		}
		var dst bytes.Buffer
		if e = proof.WriteJSON(ctx, &dst, o); e == nil || dst.Len() != 0 {
			t.Fatal("write budget")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	out, e := ReadMappedSplineChainBoundaryJSON(canceled, bytes.NewReader(encoded.Bytes()), MappedChainNativeOptions{})
	if e == nil || len(out.ModelCuts()) != 0 {
		t.Fatal("cancel")
	}
	stale := proof
	stale.sources[1].stations = []*big.Rat{big.NewRat(1, 8), big.NewRat(1, 2)}
	var dst bytes.Buffer
	if e = stale.WriteJSON(ctx, &dst, MappedChainNativeOptions{}); e == nil || dst.Len() != 0 {
		t.Fatal("stale proof accepted")
	}
	if _, e = ReadJSON(bytes.NewReader(encoded.Bytes()), JSONLimits{}); e == nil {
		t.Fatal("boundary accepted as solid")
	}
}
