//go:build research

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

func TestCuspidalCubicCreasedCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Creased Cut", .001
	so := step.CuspidalCubicSTEPOptions{PoleTolerance: 1e-14}
	iopts := iges.CuspidalCubicIGESOptions{PoleTolerance: 1e-14}
	var cuts [2]*trimmed.CuspidalCubicCut
	for order := range cuts {
		raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_creased_cut_order_%d.json", order))
		if err != nil {
			t.Fatal(err)
		}
		cuts[order], err = trimmed.ReadCuspidalCubicCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicCutOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	// The shared crease cancels from the original signed layer gap.
	for order, volume := range [2]float64{1695.0 / 22528, 243.0 / 22528} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			cut := cuts[order]
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
			other := cuts[1-order]
			if err := step.VerifyCuspidalCubicCutSTEP(ctx, bytes.NewReader(st.Bytes()), other, name, units, step.Limits{}, so); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("creased STEP Cut accepted reversed source owners: %v", err)
			}
			if err := iges.VerifyCuspidalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), other, units, iges.Limits{}, iopts); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("creased IGES Cut accepted reversed source owners: %v", err)
			}
			counts := [2][3]int{{7, 22, 17}, {4, 9, 7}}[order]
			checkCuspidalCreasedReadback(t, ctx, cut.Boundary(), cut.History().Vertices[0], counts, st.Bytes(), ig.Bytes(), volume)
		})
	}
}
