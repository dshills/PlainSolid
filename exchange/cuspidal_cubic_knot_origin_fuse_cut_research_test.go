//go:build research

package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCuspidalCubicKnotOriginCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Knot Origin Cut", .001
	var cuts [2]*trimmed.CuspidalCubicCut
	for order := range cuts {
		raw := readCuspidalKnotOriginRecipe(t, "cut", order)
		var err error
		cuts[order], err = trimmed.ReadCuspidalCubicCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicCutOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	// The shared source hinge contributes the same exact height to both roofs.
	for order, volume := range [2]float64{1695.0 / 22528, 243.0 / 22528} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			cut, other := cuts[order], cuts[1-order]
			h := cut.History()
			var stockPoints []geom.Vec3
			counts := [3]int{4, 7, 5}
			if order == 0 {
				stockPoints = []geom.Vec3{{X: .75, Z: 131.0 / 64}, {X: .75, Y: 2, Z: 265.0 / 128}, {X: .75, Z: 67.0 / 32}, {X: .75, Y: 2, Z: 271.0 / 128}}
				counts = [3]int{7, 20, 15}
			}
			checkCuspidalKnotOriginNeutralBoundary(t, ctx, cut.Boundary(), h.Vertices[0], h.Germs, stockPoints)
			for _, mode := range cuspidalKnotOriginDecimalModes {
				t.Run(mode.name, func(t *testing.T) {
					var st, ig bytes.Buffer
					so := step.CuspidalCubicSTEPOptions{PoleTolerance: mode.tolerance}
					io := iges.CuspidalCubicIGESOptions{PoleTolerance: mode.tolerance}
					if err := step.WriteCuspidalCubicCut(ctx, &st, cut, name, units, step.Limits{}, so); err != nil {
						t.Fatal(err)
					}
					if err := iges.WriteCuspidalCubicCut(ctx, &ig, cut, units, iges.Limits{}, io); err != nil {
						t.Fatal(err)
					}
					if err := step.VerifyCuspidalCubicCutSTEP(ctx, bytes.NewReader(st.Bytes()), cut, name, units, step.Limits{}, so); err != nil {
						t.Fatal(err)
					}
					if err := iges.VerifyCuspidalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), cut, units, iges.Limits{}, io); err != nil {
						t.Fatal(err)
					}
					if err := step.VerifyCuspidalCubicCutSTEP(ctx, bytes.NewReader(st.Bytes()), other, name, units, step.Limits{}, so); !errors.Is(err, geom.ErrConvergence) {
						t.Fatalf("STEP Cut accepted reversed source owners: %v", err)
					}
					if err := iges.VerifyCuspidalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), other, units, iges.Limits{}, io); !errors.Is(err, geom.ErrConvergence) {
						t.Fatalf("IGES Cut accepted reversed source owners: %v", err)
					}
					checkCuspidalKnotReadback(t, ctx, cut.Boundary(), h.Vertices[0], counts, st.Bytes(), ig.Bytes(), volume, cuspidalKnotOriginDepartureJets(t, ctx))
				})
			}
		})
	}
}
