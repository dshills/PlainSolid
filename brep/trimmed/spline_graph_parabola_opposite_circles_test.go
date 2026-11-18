package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func oppositeCircleStock() [2]twoCircleStockProbe {
	// The upper circle comes first in free-coordinate order. Native
	// assembly visits the lower enclosing roof before the upper roof.
	return [2]twoCircleStockProbe{twoCircleProbe(2, 6, 1, 8), twoCircleProbe(12, 2, 1, 16)}
}

func TestParabolaOppositeCirclesSourceDiscovery(t *testing.T) {
	ctx := context.Background()
	circles := oppositeCircleStock()
	for axis := range 2 {
		stock := parabolaTwoCirclesStock(t, axis, circles, false)
		for order := range 2 {
			p := parabolaTwoCirclesPatches(t, stock[1+order], stock[2-order])
			proof, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, p[0], p[1], axis, geom.GraphPointContactOptions{})
			if err != nil {
				t.Fatal(axis, order, err)
			}
			signs := [2][4]int{{1, -1, 1, -1}, {-1, 1, -1, 1}}[order]
			if proof.CircleRegions != ([2]int{1, 0}) || proof.CirclesAbove || proof.RegionSides != signs || !proof.Graph.ProvenInjective {
				t.Fatal("wrong source regions for opposite circles", proof)
			}
			for i, circle := range circles {
				c := proof.Circles[i]
				if c.Center[axis].Cmp(circle.x) != 0 || c.Center[1-axis].Cmp(circle.y) != 0 || c.RadiusSquared.Cmp(circle.radiusSquared) != 0 {
					t.Fatal("source proof swapped the circle factors", i, c)
				}
			}
		}
	}
}

func TestNativeParabolaOppositeCirclesEnvelopes(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	circles := oppositeCircleStock()
	disks := 13435460675.0 * math.Pi / 2251799813685248
	for axis := range 2 {
		source := parabolaTwoCirclesStock(t, axis, circles, false)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					a, b := source[1+order], source[2-order]
					r, err := CombineParabolaTwoCirclesSplineGraphs(ctx, source[0], a, b, axis, op, 1e-9, opts)
					if err != nil {
						t.Fatal(err)
					}
					body, h := r.Boundary(), r.History()
					topo := body.Topology()
					if h.EnclosingRoof != -1 || h.CircleEnclosingRoofs != ([2]int{1, 0}) ||
						topo.InnerWires != 2 || topo.EulerCharacteristic != 2 || len(topo.Shells) != 1 {
						t.Fatal("each original open roof must retain its own inner wire", h, topo)
					}
					owners := [4]int{1, 0, 1, 0}
					if (order == 1) != (op == parametric.Union) {
						for i := range owners {
							owners[i] = 1 - owners[i]
						}
					}
					if h.RoofOwners != owners {
						t.Fatal("wrong original roof in one signed region", h)
					}
					for i, f := range h.RoofFaces {
						if body.faces[f].Surface != ([2]*geom.BSplineSurface{a, b})[owners[i]] {
							t.Fatal("the selected roof was replaced", i, h)
						}
					}
					for i, enclosing := range h.CircleEnclosingRoofs {
						f := body.faces[enclosing]
						if len(f.Holes) != 1 {
							t.Fatal("circle was assigned to the wrong original roof", i, h)
						}
						for j, use := range f.Holes[0] {
							if use.Edge != h.CircleEdges[i][j] {
								t.Fatal("hole traversal does not match the original circle ledger", i, h)
							}
						}
						checkNativeOffsetCircleEdges(t, body, axis, h.CircleEdges[i][:], circles[i])
					}
					if axis == order {
						want := 29475217033789.0/14740327759872 - disks
						if op == parametric.Union {
							want = 9839813693153.0/4913442586624 + disks
						}
						checkParabolaTwoCirclesSignedVolume(t, ctx, body, want)
					}
					var raw, again bytes.Buffer
					if err := r.WriteJSON(ctx, &raw); err != nil {
						t.Fatal(err)
					}
					replay, err := ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
					if err != nil || replay.History() != h {
						t.Fatal("replay swapped the opposite hole owners", err)
					}
					checkParabolaTwoCirclesReplaySources(t, r.sources, replay.sources)
					if err := replay.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
						t.Fatal("opposite-roof recipe replay changed canonical bytes", err)
					}
				})
			}
		}
	}
}

func TestNativeParabolaOppositeCirclesCuts(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	circles := oppositeCircleStock()
	opts := ParabolaCircleGraphOptions{}
	upperLens := 102489625.0 * math.Pi / 17592186044416
	lowerLens := 316788675.0 * math.Pi / 2251799813685248
	volumes := [2][2]float64{
		{5438485955.0/14740327759872 + lowerLens, upperLens},
		{12928519905.0/4913442586624 + upperLens, lowerLens},
	}
	for axis := range 2 {
		source := parabolaTwoCirclesStock(t, axis, circles, false)
		for order, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				r, err := CutParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1+order], source[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				parts, claims := r.Components(), r.History()
				if len(parts) != 2 || len(claims) != 2 || claims[0].Region != order || claims[1].Region != 2+order {
					t.Fatal("expected one positive wall layer and the opposite circle lens", claims)
				}
				circleOrder := [2]int{1 - order, order}
				for i, part := range parts {
					h, topo := claims[i], part.Topology()
					if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2*i || topo.InnerWires != 2-2*i ||
						len(h.CircleEdges) != 4 {
						t.Fatal("the wall layer needs one through hole and the separate lens a disk boundary", h, topo)
					}
					checkNativeOffsetCircleEdges(t, part, axis, h.CircleEdges, circles[circleOrder[i]])
					checkParabolaTwoCirclesSignedVolume(t, ctx, part, want[i])
				}
				var raw, again bytes.Buffer
				if err := r.WriteJSON(ctx, &raw); err != nil {
					t.Fatal(err)
				}
				replay, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
				if err != nil || !reflect.DeepEqual(claims, replay.History()) {
					t.Fatal("mixed Cut replay swapped the positive components", err)
				}
				checkParabolaTwoCirclesReplaySources(t, r.sources, replay.sources)
				if err := replay.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
					t.Fatal("mixed Cut replay changed canonical bytes", err)
				}
			})
		}
	}
}

func TestParabolaOppositeCirclesLedgerRefusals(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	s := parabolaTwoCirclesStock(t, 0, oppositeCircleStock(), false)
	r, err := CombineParabolaTwoCirclesSplineGraphs(ctx, s[0], s[1], s[2], 0, parametric.Intersection, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	var envelope parabolaTwoCirclesEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Ledger.CircleEnclosingRoofs = []int{0, 1}
	forged, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("swapped circle owners passed source reproof", err)
	}

	cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, s[0], s[1], s[2], 0, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	raw.Reset()
	if err := cut.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	var ledger parabolaCircleCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &ledger); err != nil {
		t.Fatal(err)
	}
	ledger.Ledger = ledger.Ledger[:1]
	forged, err = json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("one of two positive parts escaped a truncated Cut recipe", err)
	}
	if got, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("two-part Cut yielded output despite the native entity limit", err)
	}
}
