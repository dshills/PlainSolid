package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaCircleCrossingsEnvelope(t *testing.T) {
	ctx := context.Background()
	for axis := range 2 {
		stock := parabolaCircleCrossingsStock(t, axis)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					out, err := CombineParabolaCircleCrossingsSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, op, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					history := out.History()
					body := out.Boundary()
					topo := body.Topology()
					if topo.Faces != 11 || topo.Edges != 25 || topo.Vertices != 16 || topo.EulerCharacteristic != 2 ||
						topo.InnerWires != 0 || len(topo.Shells) != 1 {
						t.Fatal("changed original four-region material topology", topo)
					}
					wantOwners := [4]int{1, 0, 0, 1}
					wantVolume := 1071399207.0 / 536870912
					if op == parametric.Union {
						wantOwners = [4]int{0, 1, 1, 0}
						wantVolume = 5429263229.0 / 2684354560
					}
					for i, owner := range history.RoofOwners {
						if owner != (wantOwners[i]+order)%2 || body.faces[history.RoofFaces[i]].Surface != stock[1+wantOwners[i]] {
							t.Fatal("wrong original upper/lower owner", history)
						}
					}
					for i, node := range history.Junctions {
						expected := geom.Vec3{X: float64(2*i+1) / 4, Y: 49. / 128, Z: 2}
						if axis == 1 {
							expected.X, expected.Y = expected.Y, expected.X
						}
						if body.vertices[node.Vertex] != expected {
							t.Fatal("lost exact native node", body.vertices[node.Vertex])
						}
						for j, id := range node.Rays {
							a, b := node.Sectors[(j+3)%4], node.Sectors[j]
							if !opposedGraphContourUses(body.faces[a].Outer, body.faces[b].Outer, id) {
								t.Fatal("missing original cyclic coedges", i, j, id)
							}
						}
					}
					mass, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
					if err != nil || !(math.Abs(mass.Volume-wantVolume) <= 1e-8) {
						t.Fatal("wrong independent signed envelope mass", mass, wantVolume, err)
					}
					t.Logf("volume %.16g area %.16g", mass.Volume, mass.Area)
					if (axis == 0 && order == 0 && op == parametric.Intersection) ||
						(axis == 1 && order == 1 && op == parametric.Union) {
						checkParabolaCircleCrossingsEnvelopeReplay(t, ctx, out, axis == 0)
					}
				})
			}
		}
	}
}

func checkParabolaCircleCrossingsEnvelopeReplay(t *testing.T, ctx context.Context, result *ParabolaCircleCrossingsEnvelope, negative bool) {
	t.Helper()
	var raw, again bytes.Buffer
	if err := result.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadParabolaCircleCrossingsEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, ParabolaCircleGraphOptions{})
	if err != nil || replayed.History() != result.History() {
		t.Fatal("original envelope replay changed native incidence", err)
	}
	for i, source := range result.sources {
		if !geom.SameSplineSurface(source, replayed.sources[i]) {
			t.Fatal("replay changed an original support", i)
		}
	}
	if err := replayed.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("source replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if replayed, err := ReadParabolaCircleCrossingsEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, ParabolaCircleGraphOptions{}); replayed != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short entity budget published an envelope", err)
	}
	var recipe parabolaCircleCrossingsEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	junction := &recipe.Ledger.Junctions[0]
	junction.Rays[0], junction.Rays[1] = junction.Rays[1], junction.Rays[0]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if replayed, err := ReadParabolaCircleCrossingsEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, ParabolaCircleGraphOptions{}); replayed != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("forged cyclic ray incidence passed replay", err)
	}
}
