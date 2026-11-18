//go:build research

package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaMixedCirclesCutExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const name, unit = "Mixed parabola-circle Cut", .001
	proof := edgecheck.Options{}
	for _, f := range []struct {
		axis, side, order int
		volumes           []float64
	}{
		{0, 0, 0, []float64{2793913668193235.0/8694199080638742528 - 68480808725.0*math.Pi/72057594037927936,
			79148254988215.0/8694199080638742528 + 105062125.0*math.Pi/105553116266496}},
		{0, 1, 0, []float64{6874310365806835.0/8694199080638742528 + 88781875.0*math.Pi/105553116266496,
			99498870437015.0/8694199080638742528 - 88781875.0*math.Pi/105553116266496, 18988606175.0 * math.Pi / 216172782113783808}},
		{1, 0, 1, []float64{41843838384235475.0/8694199080638742528 + 105062125.0*math.Pi/105553116266496,
			79148254988215.0/8694199080638742528 - 105062125.0*math.Pi/105553116266496, 9724805825.0 * math.Pi / 216172782113783808}},
		{1, 1, 1, []float64{14006678396434675.0/8694199080638742528 - 54278891275.0*math.Pi/72057594037927936,
			99498870437015.0/8694199080638742528 + 88781875.0*math.Pi/105553116266496}},
	} {
		t.Run(fmt.Sprintf("%d/%d/%d", f.axis, f.side, f.order), func(t *testing.T) {
			raw := mixedCirclesExchangeRecipe(t, "cut", f.axis, f.side, f.order)
			result, err := trimmed.ReadParabolaMixedCirclesCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			parts, history := result.Components(), result.History()
			controlsForFirstTwo := 0
			for i, part := range parts {
				exact, err := part.ExtractExactBezierBoundary(ctx, proof)
				if err != nil {
					t.Fatal(err)
				}
				h := history[i]
				checkParabolaCircleCrossingsExactEdges(t, exact, f.axis, h.ParabolaEdges, h.CircleEdges)
				if h.SeparatedEdges[0] >= 0 {
					holes := 0
					if i == 0 {
						holes = 8
					}
					checkMixedCirclesSeparatedRing(t, exact, f.axis, f.side, h.SeparatedEdges, holes)
				}
				if f.axis == 0 && len(parts) == 3 && i < 2 {
					controlsForFirstTwo += countTwoCirclesExchangeControls(t, exact)
				}
			}
			var st, ig bytes.Buffer
			if err := step.WriteParabolaMixedCirclesCut(ctx, &st, result, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaMixedCirclesCut(ctx, &ig, result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaMixedCirclesCutSTEP(ctx, bytes.NewReader(st.Bytes()), result, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaMixedCirclesCutIGES(ctx, bytes.NewReader(ig.Bytes()), result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if controlsForFirstTwo > 0 {
				var blocked bytes.Buffer
				if err := step.WriteParabolaMixedCirclesCut(ctx, &blocked, result, name, unit, step.Limits{MaxSolids: 2}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
					t.Fatal("STEP published a partial mixed-circle Cut", err)
				}
				if err := iges.WriteParabolaMixedCirclesCut(ctx, &blocked, result, unit, iges.Limits{MaxControlPoints: controlsForFirstTwo}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
					t.Fatal("IGES published a partial mixed-circle Cut", err)
				}
			}
			checkTwoCirclesExchangeReadback(t, ctx, parts, f.volumes, st.Bytes(), ig.Bytes())
			checkMixedCirclesPinnedJets(t, ctx, "cut", f.axis, f.side, f.order, len(parts), st.Bytes(), ig.Bytes())
		})
	}
}
