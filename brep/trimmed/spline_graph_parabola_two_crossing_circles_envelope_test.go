package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaTwoCrossingCirclesEnvelope(t *testing.T) {
	ctx := context.Background()
	f := parabolaTwoCrossingCirclesStaggered()
	// Independently integrate the authored polynomial on the stock,
	// the parabolic strip, and the circular segments using Green's theorem.
	volumes := [2]float64{1483157107570343243947.0/741904988214506029056 - 10041325.0*math.Pi/211106232532992,
		1488777861138705791317.0/741904988214506029056 + 10041325.0*math.Pi/211106232532992}
	for axis := range 2 {
		stocks := f.stocks(t, axis, false)
		for order := range 2 {
			for i, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					r, err := CombineParabolaTwoCrossingCirclesSplineGraphs(ctx, stocks[0], stocks[1+order], stocks[2-order], axis,
						op, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					body, h := r.Boundary(), r.History()
					topo := body.Topology()
					if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.InnerWires != 0 ||
						([3]int{topo.Faces, topo.Edges, topo.Vertices}) != ([3]int{13, 33, 22}) {
						t.Fatal("changed exact envelope topology", topo)
					}
					owners := [6]int{1, 0, 0, 1, 0, 1}
					for region, who := range owners {
						want := (who + order + i) % 2
						if h.RoofOwners[region] != want || body.faces[h.RoofFaces[region]].Surface != stocks[1+(order+want)%2] {
							t.Fatal("changed original region owner", region, h)
						}
					}
					for c, nodes := range h.Junctions {
						if len(h.CircleEdges[c]) != 4 {
							t.Fatal("changed exact circle subdivisions", h)
						}
						for j, node := range nodes {
							x, _ := f.roots[c][j].Float64()
							y, _ := f.ordinate(f.roots[c][j]).Float64()
							want := geom.Vec3{X: x, Y: y, Z: 2}
							if axis == 1 {
								want.X, want.Y = want.Y, want.X
							}
							if body.vertices[node.Vertex] != want {
								t.Fatal("changed exact source junction", node, want)
							}
						}
					}
					mass, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if err != nil || math.Abs(mass.Volume-volumes[i]) > 1e-10 {
						t.Fatal("envelope differs from independent signed volume", mass, volumes[i], err)
					}
					t.Logf("signed volume %.17g; exact integral %.17g", mass.Volume, volumes[i])
					checkTwoCrossingCirclesEnvelopeReplay(t, r, axis == 0 && order == 0 && i == 0)
				})
			}
		}
	}
}

func checkTwoCrossingCirclesEnvelopeReplay(t *testing.T, r *ParabolaTwoCrossingCirclesEnvelope, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaTwoCrossingCirclesEnvelope, error) {
		return ReadParabolaTwoCrossingCirclesEnvelopeJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	out, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(out.History(), r.History()) {
		t.Fatal("reloaded result changed exact incidence", err)
	}
	for i, source := range r.sources {
		if !geom.SameSplineSurface(source, out.sources[i]) {
			t.Fatal("reloaded result changed the original source", i)
		}
	}
	if err := out.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("reloaded result changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if out, err := read(raw.Bytes(), JSONLimits{MaxEntities: 1}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short native budget published a partial arrangement", out, err)
	}
	var recipe parabolaCrossingCirclesEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Ledger.CircleEdges[0], recipe.Ledger.CircleEdges[1] = recipe.Ledger.CircleEdges[1], recipe.Ledger.CircleEdges[0]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(forged, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted the other circle's original coedges", out, err)
	}
}

func TestNativeParabolaTwoCrossingCirclesUnrepresentableJunction(t *testing.T) {
	stocks := parabolaTwoCrossingCirclesCoincidentProjection().stocks(t, 0, false)
	out, err := CombineParabolaTwoCrossingCirclesSplineGraphs(context.Background(), stocks[0], stocks[1], stocks[2], 0,
		parametric.Intersection, 1e-9, ParabolaCircleGraphOptions{})
	if out != nil || !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("published unrepresentable exact junction", out, err)
	}
}
