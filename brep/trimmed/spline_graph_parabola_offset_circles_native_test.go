package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
)

func TestNativeParabolaOffsetCirclesEnvelopes(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	circles := offsetCirclesStock()
	diskVolume := 905961714175.0 * math.Pi / 1729382256910270464
	for axis := range 2 {
		src := parabolaTwoCirclesStock(t, axis, circles, false)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					r, err := CombineParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, op, 1e-9, opts)
					if err != nil {
						t.Fatal(err)
					}
					body, h := r.Boundary(), r.History()
					if h.EnclosingRoof != 1 || body.Topology().InnerWires != 2 {
						t.Fatal("offset circles lost their enclosing source roof", h, body.Topology())
					}
					for i, ring := range h.CircleEdges {
						checkNativeOffsetCircleEdges(t, body, axis, ring[:], circles[i])
					}
					if axis == 0 && order == 0 {
						volume := 192847408364983.0/96482145337344 - diskVolume
						if op == parametric.Union {
							volume = 193030413089753.0/96482145337344 + diskVolume
						}
						checkParabolaTwoCirclesSignedVolume(t, ctx, body, volume)
					}
					var raw, again bytes.Buffer
					if err := r.WriteJSON(ctx, &raw); err != nil {
						t.Fatal(err)
					}
					replay, err := ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
					if err != nil || replay.History() != h {
						t.Fatal("offset envelope replay changed original circle ownership", err)
					}
					checkParabolaTwoCirclesReplaySources(t, r.sources, replay.sources)
					if err := replay.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
						t.Fatal("offset envelope replay changed canonical bytes", err)
					}
				})
			}
		}
	}
}

func TestNativeParabolaOffsetCirclesCuts(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	circles := offsetCirclesStock()
	diskVolume := 905961714175.0 * math.Pi / 1729382256910270464
	volumes := [2][]float64{
		{116882309705.0 / 96482145337344, 73873625.0 * math.Pi / 211106232532992, 100262992725.0 * math.Pi / 576460752303423488},
		{66122415065.0/96482145337344 + diskVolume},
	}
	for axis := range 2 {
		src := parabolaTwoCirclesStock(t, axis, circles, false)
		for order, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				parts, histories := cut.Components(), cut.History()
				if len(parts) != len(want) || len(histories) != len(want) {
					t.Fatal("offset Cut mixed independent positive components", histories)
				}
				for i, body := range parts {
					h := histories[i]
					switch {
					case order == 1:
						if body.Topology().EulerCharacteristic != -2 || len(h.CircleEdges) != 8 {
							t.Fatal("reverse Cut lost an offset through hole", h)
						}
						for j, circle := range circles {
							checkNativeOffsetCircleEdges(t, body, axis, h.CircleEdges[4*j:4*j+4], circle)
						}
					case i > 0:
						if h.Region != 1+i || body.Topology().EulerCharacteristic != 2 || len(h.CircleEdges) != 4 {
							t.Fatal("offset lens lost its own source region", h)
						}
						checkNativeOffsetCircleEdges(t, body, axis, h.CircleEdges, circles[i-1])
					}
					if axis == 0 {
						checkParabolaTwoCirclesSignedVolume(t, ctx, body, want[i])
					}
				}
				var raw, again bytes.Buffer
				if err := cut.WriteJSON(ctx, &raw); err != nil {
					t.Fatal(err)
				}
				replay, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
				if err != nil || !reflect.DeepEqual(replay.History(), histories) {
					t.Fatal("offset Cut replay mixed the lens sources", err)
				}
				checkParabolaTwoCirclesReplaySources(t, cut.sources, replay.sources)
				if err := replay.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
					t.Fatal("offset Cut replay changed canonical bytes", err)
				}
			})
		}
	}
}

func TestNativeStackedParabolaCirclesCutOwnerOrder(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	circles := [2]twoCircleStockProbe{twoCircleProbe(8, 9, 1, 16), twoCircleProbe(16, 27, 3, 32)}
	src := parabolaTwoCirclesStock(t, 0, circles, false)
	cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	parts, history := cut.Components(), cut.History()
	if len(parts) != 3 || len(history) != 3 {
		t.Fatal("aligned free centers lost one of the two distinct lenses")
	}
	for i, c := range circles {
		checkNativeOffsetCircleEdges(t, parts[i+1], 0, history[i+1].CircleEdges, c)
	}
}

func checkNativeOffsetCircleEdges(t *testing.T, body Solid, axis int, ring []int, circle twoCircleStockProbe) {
	t.Helper()
	for _, id := range ring {
		edge := body.edges[id]
		for _, index := range [2]int{edge.Start, edge.End} {
			p := body.vertices[index]
			xy := [2]float64{p.X, p.Y}
			x := new(big.Rat).Sub(ratInset(xy[axis]), circle.x)
			y := new(big.Rat).Sub(ratInset(xy[1-axis]), circle.y)
			radius := new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
			if p.Z != 2 || radius.Cmp(circle.radiusSquared) != 0 {
				t.Fatal("native circular edge displaced or swapped a source owner", p, circle)
			}
		}
	}
}
