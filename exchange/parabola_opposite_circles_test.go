package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// The independent signed moments of the complete disks in the original
// stock difference are negative on circle 0 and positive on circle 1.
const (
	oppositeUpperLensVolume = 102489625.0 * math.Pi / 17592186044416
	oppositeLowerLensVolume = 316788675.0 * math.Pi / 2251799813685248
)

func TestParabolaOppositeCirclesEnvelopeExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const name, unit = "Parabola and opposite-side circles envelope", .001
	proof := edgecheck.Options{}
	disks := oppositeUpperLensVolume + oppositeLowerLensVolume
	for _, c := range []struct {
		name        string
		axis, order int
		op          parametric.BooleanOp
		want        float64
	}{
		{"common", 0, 0, parametric.Intersection, 29475217033789.0/14740327759872 - disks},
		{"fuse", 1, 1, parametric.Union, 9839813693153.0/4913442586624 + disks},
	} {
		t.Run(c.name, func(t *testing.T) {
			stock := oppositeCirclesExchangeStock(t, ctx, c.axis)
			result, err := trimmed.CombineParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1+c.order], stock[2-c.order], c.axis, c.op, 1e-9, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
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
			body := result.Boundary()
			boundary, err := body.ExtractExactBezierBoundary(ctx, proof)
			if err != nil {
				t.Fatal(err)
			}
			history := result.History()
			for circle, edges := range history.CircleEdges {
				checkOppositeCirclesExchangeRing(t, boundary, c.axis, circle, edges[:])
			}
			checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{body}, []float64{c.want}, st.Bytes(), ig.Bytes())
		})
	}
}

func TestParabolaOppositeCirclesCutExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const name, unit = "Parabola and opposite-side circles Cut", .001
	proof := edgecheck.Options{}
	volumes := [2][]float64{
		{5438485955.0/14740327759872 + oppositeLowerLensVolume, oppositeUpperLensVolume},
		{12928519905.0/4913442586624 + oppositeUpperLensVolume, oppositeLowerLensVolume},
	}
	for axis := range 2 {
		stock := oppositeCirclesExchangeStock(t, ctx, axis)
		for order, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				result, err := trimmed.CutParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, 1e-9, trimmed.ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, history := result.Components(), result.History()
				if len(parts) != 2 || len(history) != 2 || history[0].Region != order || history[1].Region != 2+order {
					t.Fatalf("expected a separate original wall layer and disk lens: %+v", history)
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
					for part, circle := range [2]int{1 - order, order} {
						boundary, err := parts[part].ExtractExactBezierBoundary(ctx, proof)
						if err != nil {
							t.Fatal(err)
						}
						checkOppositeCirclesExchangeRing(t, boundary, axis, circle, history[part].CircleEdges)
						if order == 0 && part == 0 {
							var blocked bytes.Buffer
							budget := iges.Limits{MaxControlPoints: countTwoCirclesExchangeControls(t, boundary)}
							err := iges.WriteParabolaTwoCirclesGraphCut(ctx, &blocked, result, unit, budget, proof)
							if !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
								t.Fatal("IGES published a wall layer without its lens", err)
							}
						}
					}
				}
				if axis == 0 && order == 0 {
					var blocked bytes.Buffer
					err := step.WriteParabolaTwoCirclesGraphCut(ctx, &blocked, result, name, unit, step.Limits{MaxSolids: 1}, proof)
					if !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published a wall layer without its lens", err)
					}
				}
				checkTwoCirclesExchangeReadback(t, ctx, parts, want, st.Bytes(), ig.Bytes())
			})
		}
	}
}

func oppositeCirclesExchangeStock(t *testing.T, ctx context.Context, axis int) [3]*geom.BSplineSurface {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_parabola_opposite_circles_axis_%d.json", axis))
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
	for region, owner := range history.RoofOwners {
		stock[1+owner] = faces[history.RoofFaces[region]].Surface.(*geom.BSplineSurface)
	}
	return stock
}

func checkOppositeCirclesExchangeRing(t *testing.T, b trimmed.ExactBezierBoundary, axis, circle int, edges []int) {
	t.Helper()
	if len(edges) != 4 {
		t.Fatalf("expected four exact quadrant arcs for circle %d, got %v", circle, edges)
	}
	for _, id := range edges {
		checkOppositeCirclesArcLaw(t, b.Edges[id].Model, axis, circle, 2)
		owners := 0
		for _, face := range b.Faces {
			for _, loop := range face.Loops {
				for _, use := range loop {
					if use.Edge == id {
						checkOppositeCirclesArcLaw(t, use.UV, axis, circle, 0)
						owners++
					}
				}
			}
		}
		if owners != 2 {
			t.Fatalf("expected two original roof coedges on arc %d, got %d", id, owners)
		}
	}
}

func checkOppositeCirclesArcLaw(t *testing.T, span geom.ExactBezierSpan, axis, circle int, z int64) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("expected the exact quadratic span in its normalized shared edge clock")
	}
	xCenter := big.NewRat(int64(1+2*circle), 4)
	yCenter := [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 8)}[circle]
	radiusSquared := big.NewRat(1, [2]int64{64, 256}[circle])
	// The homogeneous conic equation has degree at most four in the edge
	// clock. Five distinct rational roots prove the polynomial is zero.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var h [4]big.Rat
		for coordinate := range h {
			for j, coefficient := range basis {
				h[coordinate].Add(&h[coordinate], new(big.Rat).Mul(span.Control[j][coordinate], big.NewRat(coefficient, 1)))
			}
		}
		x := new(big.Rat).Quo(&h[axis], &h[3])
		y := new(big.Rat).Quo(&h[1-axis], &h[3])
		if new(big.Rat).Quo(&h[2], &h[3]).Cmp(big.NewRat(z, 1)) != 0 {
			t.Fatalf("circle %d left its exact plane", circle)
		}
		x.Sub(x, xCenter)
		y.Sub(y, yCenter)
		x.Mul(x, x).Add(x, y.Mul(y, y))
		if x.Cmp(radiusSquared) != 0 {
			t.Fatalf("arc changed the original circle %d", circle)
		}
	}
}
