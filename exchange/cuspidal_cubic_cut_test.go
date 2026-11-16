package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCuspidalCubicCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Cut", .001
	// The original gap integrates to 33/512 across the footprint. With
	// u=1/4+3*t² the integral of the negative gap over the pointed lobe
	// has magnitude 243/22528, giving the two positive Cut volumes.
	const pointed = 243.0 / 22528
	volumes := [2]float64{33.0/512 + pointed, pointed}
	so := step.CuspidalCubicSTEPOptions{PoleTolerance: 1e-14}
	iopts := iges.CuspidalCubicIGESOptions{PoleTolerance: 1e-14}
	for order, volume := range volumes {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_cut_order_%d.json", order))
			if err != nil {
				t.Fatal(err)
			}
			cut, err := trimmed.ReadCuspidalCubicCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicCutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var st, ig bytes.Buffer
			if err := step.WriteCuspidalCubicCut(ctx, &st, cut, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteCuspidalCubicCut(ctx, &ig, cut, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyCuspidalCubicCutSTEP(ctx, bytes.NewReader(st.Bytes()), cut, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyCuspidalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), cut, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			otherRaw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_cut_order_%d.json", 1-order))
			if err != nil {
				t.Fatal(err)
			}
			other, err := trimmed.ReadCuspidalCubicCutJSON(ctx, bytes.NewReader(otherRaw), trimmed.JSONLimits{}, trimmed.CuspidalCubicCutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyCuspidalCubicCutSTEP(ctx, bytes.NewReader(st.Bytes()), other, name, units, step.Limits{}, so); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("STEP verifier accepted a Cut with reversed operand ownership: %v", err)
			}
			if err := iges.VerifyCuspidalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), other, units, iges.Limits{}, iopts); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("IGES verifier accepted a Cut with reversed operand ownership: %v", err)
			}
			var refused bytes.Buffer
			if err := step.WriteCuspidalCubicCut(ctx, &refused, cut, name, units, step.Limits{MaxFaces: 1}, so); err == nil || refused.Len() != 0 {
				t.Fatal("limited STEP writer published Cut geometry", err)
			}
			if err := iges.WriteCuspidalCubicCut(ctx, &refused, cut, units, iges.Limits{MaxEntities: 1}, iopts); err == nil || refused.Len() != 0 {
				t.Fatal("limited IGES writer published Cut geometry", err)
			}
			checkCuspidalMaterialReadback(t, ctx, cut.Boundary(), cut.History().Vertices, st.Bytes(), ig.Bytes(), volume)
		})
	}
}
