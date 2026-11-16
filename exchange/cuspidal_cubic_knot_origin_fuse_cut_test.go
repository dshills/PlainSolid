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

func TestCuspidalCubicKnotOriginFuseExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Knot Origin Fuse", .001
	const volume = 286551.0 / 22528
	for order := range 2 {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			raw := readCuspidalKnotOriginRecipe(t, "fuse", order)
			fuse, err := trimmed.ReadCuspidalCubicFuseJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicFuseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			h := fuse.History()
			checkCuspidalKnotOriginNeutralBoundary(t, ctx, fuse.Boundary(), h.Vertices[0], h.Germs,
				[]geom.Vec3{{X: .75, Z: 67.0 / 32}, {X: .75, Y: 2, Z: 271.0 / 128}})
			var st, ig bytes.Buffer
			if err := step.WriteCuspidalCubicFuse(ctx, &st, fuse, name, units, step.Limits{}, step.CuspidalCubicSTEPOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || st.Len() != 0 {
				t.Fatalf("strict STEP must refuse the nonterminating wall control before writing: %v", err)
			}
			if err := iges.WriteCuspidalCubicFuse(ctx, &ig, fuse, units, iges.Limits{}, iges.CuspidalCubicIGESOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || ig.Len() != 0 {
				t.Fatalf("strict IGES must refuse the nonterminating wall control before writing: %v", err)
			}
			so := step.CuspidalCubicSTEPOptions{PoleTolerance: 1e-14}
			io := iges.CuspidalCubicIGESOptions{PoleTolerance: 1e-14}
			if err := step.WriteCuspidalCubicFuse(ctx, &st, fuse, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteCuspidalCubicFuse(ctx, &ig, fuse, units, iges.Limits{}, io); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyCuspidalCubicFuseSTEP(ctx, bytes.NewReader(st.Bytes()), fuse, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyCuspidalCubicFuseIGES(ctx, bytes.NewReader(ig.Bytes()), fuse, units, iges.Limits{}, io); err != nil {
				t.Fatal(err)
			}
			checkCuspidalKnotReadback(t, ctx, fuse.Boundary(), h.Vertices[0], [3]int{9, 22, 15}, st.Bytes(), ig.Bytes(), volume, cuspidalKnotOriginDepartureJets(t, ctx))
		})
	}
}

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
			var st, ig bytes.Buffer
			var tolerance float64
			if order == 0 {
				stockPoints = []geom.Vec3{{X: .75, Z: 131.0 / 64}, {X: .75, Y: 2, Z: 265.0 / 128}, {X: .75, Z: 67.0 / 32}, {X: .75, Y: 2, Z: 271.0 / 128}}
				counts = [3]int{7, 20, 15}
				if err := step.WriteCuspidalCubicCut(ctx, &st, cut, name, units, step.Limits{}, step.CuspidalCubicSTEPOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || st.Len() != 0 {
					t.Fatalf("strict STEP must refuse the nonterminating wall control before writing: %v", err)
				}
				if err := iges.WriteCuspidalCubicCut(ctx, &ig, cut, units, iges.Limits{}, iges.CuspidalCubicIGESOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || ig.Len() != 0 {
					t.Fatalf("strict IGES must refuse the nonterminating wall control before writing: %v", err)
				}
				tolerance = 1e-14
			}
			checkCuspidalKnotOriginNeutralBoundary(t, ctx, cut.Boundary(), h.Vertices[0], h.Germs, stockPoints)
			so := step.CuspidalCubicSTEPOptions{PoleTolerance: tolerance}
			io := iges.CuspidalCubicIGESOptions{PoleTolerance: tolerance}
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
			if err := step.VerifyCuspidalCubicCutSTEP(ctx, bytes.NewReader(st.Bytes()), other, name, units, step.Limits{}, step.CuspidalCubicSTEPOptions{PoleTolerance: 1e-14}); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("STEP Cut accepted reversed source owners: %v", err)
			}
			if err := iges.VerifyCuspidalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), other, units, iges.Limits{}, iges.CuspidalCubicIGESOptions{PoleTolerance: 1e-14}); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("IGES Cut accepted reversed source owners: %v", err)
			}
			checkCuspidalKnotReadback(t, ctx, cut.Boundary(), h.Vertices[0], counts, st.Bytes(), ig.Bytes(), volume, cuspidalKnotOriginDepartureJets(t, ctx))
		})
	}
}

func readCuspidalKnotOriginRecipe(t *testing.T, operation string, order int) []byte {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_knot_origin_%s_order_%d.json", operation, order))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
