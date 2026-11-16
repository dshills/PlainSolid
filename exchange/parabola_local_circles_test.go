package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// These are the independent exact moments of the original roof difference
// over the two disks. The smaller disk lies below the parabola.
const (
	localUpperCircleLens = 8399735575.0 * math.Pi / 3458764513820540928
	localLowerCircleLens = 36822589825.0 * math.Pi / 442721857769029238784
)

func TestParabolaLocalCirclesEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Parabola and locally separated circles envelope", .001
	for _, c := range []struct {
		name        string
		axis, order int
		op          parametric.BooleanOp
		want        float64
	}{
		{"common", 0, 0, parametric.Intersection, 33957950114251843.0/16980857579372544 - localUpperCircleLens - localLowerCircleLens},
		{"fuse", 1, 1, parametric.Union, 34079095217310893.0/16980857579372544 + localUpperCircleLens + localLowerCircleLens},
	} {
		t.Run(c.name, func(t *testing.T) {
			stock := localCirclesExchangeStock(t, ctx, c.axis)
			a, b := stock[1+c.order], stock[2-c.order]
			result, err := trimmed.CombineParabolaTwoCirclesSplineGraphs(ctx, stock[0], a, b, c.axis, c.op, 1e-9, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			proof := edgecheck.Options{}
			solid := result.Boundary()
			exact, err := solid.ExtractExactBezierBoundary(ctx, proof)
			if err != nil {
				t.Fatal(err)
			}
			history := result.History()
			if history.CircleEnclosingRoofs != ([2]int{1, 0}) || history.EnclosingRoof != -1 {
				t.Fatal("changed original enclosure ledger", history)
			}
			for circle, edges := range history.CircleEdges {
				checkLocalCirclesExchangeRing(t, exact, c.axis, circle, edges[:], [2]*geom.BSplineSurface{a, b})
			}
			var st, ig bytes.Buffer
			if err := step.WriteParabolaTwoCirclesGraphEnvelope(ctx, &st, result, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaTwoCirclesGraphEnvelope(ctx, &ig, result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaTwoCirclesGraphEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), result, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaTwoCirclesGraphEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{solid}, []float64{c.want}, st.Bytes(), ig.Bytes())
		})
	}
}

func TestParabolaLocalCirclesCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Parabola and locally separated circles Cut", .001
	proof := edgecheck.Options{}
	volumes := [2][]float64{
		{3765044493245.0/16980857579372544 + localLowerCircleLens, localUpperCircleLens},
		{117380058565805.0/16980857579372544 + localUpperCircleLens, localLowerCircleLens},
	}
	for axis := range 2 {
		stock := localCirclesExchangeStock(t, ctx, axis)
		for order, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				a, b := stock[1+order], stock[2-order]
				result, err := trimmed.CutParabolaTwoCirclesSplineGraphs(ctx, stock[0], a, b, axis, 1e-9, trimmed.ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, history := result.Components(), result.History()
				if len(parts) != 2 || len(history) != 2 || history[0].Region != order || history[1].Region != 2+order {
					t.Fatal("expected a separate wall layer and original positive disk lens", history)
				}
				var st, ig bytes.Buffer
				if err := step.WriteParabolaTwoCirclesGraphCut(ctx, &st, result, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.WriteParabolaTwoCirclesGraphCut(ctx, &ig, result, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if axis == 0 {
					if err := step.VerifyParabolaTwoCirclesGraphCutSTEP(ctx, bytes.NewReader(st.Bytes()), result, name, unit, step.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := iges.VerifyParabolaTwoCirclesGraphCutIGES(ctx, bytes.NewReader(ig.Bytes()), result, unit, iges.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					wallControls := 0
					for part, circle := range [2]int{1 - order, order} {
						exact, err := parts[part].ExtractExactBezierBoundary(ctx, proof)
						if err != nil {
							t.Fatal(err)
						}
						checkLocalCirclesExchangeRing(t, exact, axis, circle, history[part].CircleEdges, [2]*geom.BSplineSurface{a, b})
						if part == 0 && order == 1 {
							wallControls = countTwoCirclesExchangeControls(t, exact)
						}
					}
					if wallControls > 0 {
						var blocked bytes.Buffer
						err := step.WriteParabolaTwoCirclesGraphCut(ctx, &blocked, result, name, unit, step.Limits{MaxSolids: 1}, proof)
						if !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
							t.Fatal("STEP dropped the tiny positive lens under a short budget", err)
						}
						err = iges.WriteParabolaTwoCirclesGraphCut(ctx, &blocked, result, unit, iges.Limits{MaxControlPoints: wallControls}, proof)
						if !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
							t.Fatal("IGES dropped the tiny positive lens under a short budget", err)
						}
					}
				}
				checkTwoCirclesExchangeReadback(t, ctx, parts, want, st.Bytes(), ig.Bytes())
			})
		}
	}
}

func localCirclesExchangeStock(t *testing.T, ctx context.Context, axis int) [3]*geom.BSplineSurface {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_parabola_local_circles_axis_%d.json", axis))
	if err != nil {
		t.Fatal(err)
	}
	result, err := trimmed.ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	history := result.History()
	faces := result.Boundary().Faces()
	stock := [3]*geom.BSplineSurface{faces[history.Base].Surface.(*geom.BSplineSurface)}
	for i, owner := range history.RoofOwners {
		stock[1+owner] = faces[history.RoofFaces[i]].Surface.(*geom.BSplineSurface)
	}
	return stock
}

func checkLocalCirclesExchangeRing(t *testing.T, b trimmed.ExactBezierBoundary, axis, circle int, edges []int, roofs [2]*geom.BSplineSurface) {
	t.Helper()
	if len(edges) != 4 {
		t.Fatalf("expected four exact arcs on original circle %d, got %v", circle, edges)
	}
	for _, id := range edges {
		checkLocalCirclesConicLaw(t, b.Edges[id].Model, axis, circle, 2)
		var seen [2]int
		forward := 0
		for _, face := range b.Faces {
			for _, loop := range face.Loops {
				for _, use := range loop {
					if use.Edge != id {
						continue
					}
					if face.Spline == roofs[0] {
						seen[0]++
					} else if face.Spline == roofs[1] {
						seen[1]++
					} else {
						t.Fatal("circle coedge lost its original roof support", id)
					}
					checkLocalCirclesConicLaw(t, use.UV, axis, circle, 0)
					if face.Reversed == use.Reversed {
						forward++
					}
				}
			}
		}
		if seen != ([2]int{1, 1}) || forward != 1 {
			t.Fatal("lost opposed directed uses on the two original circle owners", id, seen, forward)
		}
	}
}

func checkLocalCirclesConicLaw(t *testing.T, span geom.ExactBezierSpan, axis, circle int, z int64) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("lost the exact quadratic source clock")
	}
	cx := big.NewRat(int64(1+6*circle), 8)
	cy := big.NewRat(5, 16)
	radiusSquared := big.NewRat(1, [2]int64{1024, 4096}[circle])
	// Substitution of the homogeneous quadratic into the circle identity
	// has degree at most four, so five exact rational roots suffice.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var h [4]big.Rat
		for k := range h {
			for j, coefficient := range basis {
				h[k].Add(&h[k], new(big.Rat).Mul(span.Control[j][k], big.NewRat(coefficient, 1)))
			}
		}
		x := new(big.Rat).Quo(&h[axis], &h[3])
		y := new(big.Rat).Quo(&h[1-axis], &h[3])
		if new(big.Rat).Quo(&h[2], &h[3]).Cmp(big.NewRat(z, 1)) != 0 {
			t.Fatal("circle left its original plane")
		}
		x.Sub(x, cx)
		y.Sub(y, cy)
		if x.Mul(x, x).Add(x, y.Mul(y, y)).Cmp(radiusSquared) != 0 {
			t.Fatal("changed original circle or owner side", circle)
		}
	}
}
