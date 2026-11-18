package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCuspidalCubicKnotOriginFuseExchange(t *testing.T) {
	slowtest.Skip(t)
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
			for _, mode := range cuspidalKnotOriginDecimalModes {
				t.Run(mode.name, func(t *testing.T) {
					var st, ig bytes.Buffer
					so := step.CuspidalCubicSTEPOptions{PoleTolerance: mode.tolerance}
					io := iges.CuspidalCubicIGESOptions{PoleTolerance: mode.tolerance}
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
					if mode.tolerance == 0 {
						var refused bytes.Buffer
						if err := step.WriteCuspidalCubicFuse(ctx, &refused, fuse, name, units, step.Limits{MaxBytes: int64(st.Len() - 1)}, so); !errors.Is(err, geom.ErrConvergence) || refused.Len() != 0 {
							t.Fatalf("STEP byte limit published part of the exact shell: %v", err)
						}
						controls := countCuspidalKnotOriginControls(t, ctx, fuse.Boundary())
						// Each of the two elevated wall curves adds one control.
						if err := iges.WriteCuspidalCubicFuse(ctx, &refused, fuse, units, iges.Limits{MaxControlPoints: controls + 1}, io); !errors.Is(err, geom.ErrConvergence) || refused.Len() != 0 {
							t.Fatalf("IGES short control limit published part of the exact shell: %v", err)
						}
						for _, limit := range []int{controls + 2, math.MaxInt} {
							var finished bytes.Buffer
							if err := iges.WriteCuspidalCubicFuse(ctx, &finished, fuse, units, iges.Limits{MaxControlPoints: limit}, io); err != nil || !bytes.Equal(finished.Bytes(), ig.Bytes()) {
								t.Fatalf("IGES control limit %d changed the exact shell: %v", limit, err)
							}
						}
					}
					checkCuspidalKnotReadback(t, ctx, fuse.Boundary(), h.Vertices[0], [3]int{9, 22, 15}, st.Bytes(), ig.Bytes(), volume, cuspidalKnotOriginDepartureJets(t, ctx))
				})
			}
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

var cuspidalKnotOriginDecimalModes = [...]struct {
	name      string
	tolerance float64
}{{"exact", 0}, {"bounded", 1e-14}}

func TestCuspidalCubicWallElevationScope(t *testing.T) {
	ctx := context.Background()
	for _, fixture := range []string{"fuse_order_0", "creased_fuse_order_0"} {
		t.Run(fixture, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/native_cuspidal_cubic_" + fixture + ".json")
			if err != nil {
				t.Fatal(err)
			}
			fuse, err := trimmed.ReadCuspidalCubicFuseJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicFuseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := step.WriteCuspidalCubicFuse(ctx, &out, fuse, "Cuspidal Fuse", .001, step.Limits{}, step.CuspidalCubicSTEPOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out.Len() != 0 {
				t.Fatalf("STEP applied wall elevation without the shared cusp knot: %v", err)
			}
			if err := iges.WriteCuspidalCubicFuse(ctx, &out, fuse, .001, iges.Limits{}, iges.CuspidalCubicIGESOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out.Len() != 0 {
				t.Fatalf("IGES applied wall elevation without the shared cusp knot: %v", err)
			}
		})
	}
}

func countCuspidalKnotOriginControls(t *testing.T, ctx context.Context, solid trimmed.Solid) int {
	t.Helper()
	b, err := solid.ExtractExactBezierBoundaryAtRationalKnots(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, edge := range b.Edges {
		count += len(edge.Model.Control)
	}
	for _, face := range b.Faces {
		s, err := face.BSplineSupport()
		if err != nil {
			t.Fatal(err)
		}
		count += len(s.Data().Poles)
		for _, loop := range face.Loops {
			for _, use := range loop {
				count += len(use.UV.Control)
			}
		}
	}
	return count
}

func readCuspidalKnotOriginRecipe(t *testing.T, operation string, order int) []byte {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_knot_origin_%s_order_%d.json", operation, order))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
