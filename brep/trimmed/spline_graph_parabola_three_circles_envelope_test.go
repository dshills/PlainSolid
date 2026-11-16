package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaThreeCirclesEnvelope(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	for _, f := range []struct {
		side int
		disk twoCircleStockProbe
		mass [2]float64
	}{
		{1, twoCircleProbe(16, 12, 1, 32), [2]float64{
			181092723815311532501.0/90954698074374537216 - 1760437879858175.0*math.Pi/110680464442257309696,
			197453610379913036587.0/90954698074374537216 + 1760437879858175.0*math.Pi/110680464442257309696,
		}},
		{0, twoCircleProbe(16, 6, 1, 32), [2]float64{
			2356866147562293938273.0/1182411074966868983808 - 622163803174795.0*math.Pi/36893488147419103232,
			2701138607222016689311.0/1182411074966868983808 + 622163803174795.0*math.Pi/36893488147419103232,
		}},
	} {
		for axis := range 2 {
			stocks := parabolaThreeCircleStocks(t, axis, f.disk, false)
			for order := range 2 {
				for i, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", f.side, axis, order, i), func(t *testing.T) {
						roofs := [2]*geom.BSplineSurface{stocks[1+order], stocks[2-order]}
						r, err := CombineParabolaThreeCirclesSplineGraphs(ctx, stocks[0], roofs[0], roofs[1], axis, op, 1e-9, opts)
						if err != nil {
							t.Fatal(err)
						}
						body, h := r.Boundary(), r.History()
						topo := body.Topology()
						if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.InnerWires != 1 ||
							([3]int{topo.Faces, topo.Edges, topo.Vertices}) != ([3]int{14, 37, 26}) {
							t.Fatal("wrong exact seven-region topology", topo)
						}
						if h.CrossingCircles != [2]int{0, 2} || h.SeparatedCircle != 1 ||
							h.SeparatedRegion != f.side || h.SeparatedFace != 6 ||
							h.SeparatedOwner == h.RoofOwners[f.side] || body.faces[h.SeparatedFace].Surface != roofs[h.SeparatedOwner] {
							t.Fatal("changed original isolated face ownership", h)
						}
						for _, id := range h.SeparatedEdges {
							if !opposedGraphContourUses(body.faces[f.side].Holes[0], body.faces[h.SeparatedFace].Outer, id) {
								t.Fatal("third seam lost an opposed original coedge", id)
							}
						}
						for _, group := range h.Junctions {
							for _, node := range group {
								if slices.Contains(h.SeparatedVertices[:], node.Vertex) {
									t.Fatal("mapped separated circle to a transverse vertex", node)
								}
							}
						}
						props, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
						if err != nil || math.Abs(props.Volume-f.mass[i]) > 1e-10 {
							t.Fatal("native material differs from an independently integrated source volume", props, f.mass[i], err)
						}
						t.Logf("signed volume %.17g; exact integral %.17g", props.Volume, f.mass[i])
						if axis == order {
							checkParabolaThreeCirclesEnvelopeReplay(t, r, f.side == 1 && i == 0 && axis == 0)
						}
					})
				}
			}
		}
	}
}

func checkParabolaThreeCirclesEnvelopeReplay(t *testing.T, r *ParabolaThreeCirclesEnvelope, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaThreeCirclesEnvelope, error) {
		return ReadParabolaThreeCirclesEnvelopeJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	replayed, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(replayed.History(), r.History()) {
		t.Fatal("replay changed the original incidence", err)
	}
	for i, source := range r.sources {
		if !geom.SameSplineSurface(source, replayed.sources[i]) {
			t.Fatal("replay changed an original source", i)
		}
	}
	if err := replayed.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	var recipe parabolaThreeCirclesEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Arrangement.Edges[0] = recipe.Ledger.CircleEdges[0][0]
	bad, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(bad, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a different original separated circle coedge", out, err)
	}
}

func TestNativeParabolaThreeCirclesUnresolvedChordDomain(t *testing.T) {
	ctx := context.Background()
	// This third circle is truly above the parabola, but its lower half
	// crosses the current straight chord of the middle parabola trim.
	stocks := parabolaThreeCircleStocks(t, 0, twoCircleProbe(64, 41, 4, 128), false)
	src := parabolaTwoCirclesPatches(t, stocks[1], stocks[2])
	if _, err := geom.DiscoverSplineGraphParabolaThreeCircles(ctx, src[0], src[1], 0, geom.GraphPointContactOptions{}); err != nil {
		t.Fatal("changed established exact signed geometry", err)
	}
	r, err := CombineParabolaThreeCirclesSplineGraphs(ctx, stocks[0], stocks[1], stocks[2], 0, parametric.Intersection, 1e-9, ParabolaCircleGraphOptions{})
	if r != nil || !errors.Is(err, geom.ErrInvalid) {
		t.Fatal("published an unresolved native chord domain", r, err)
	}
}
