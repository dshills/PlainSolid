package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaCircleApexTangencyEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	volumes := [2][2]float64{
		{14065067533.0/7046430720 - 1155.0*math.Pi/134217728, 2876007319.0/1409286144 + 1155.0*math.Pi/134217728},
		{14028780973.0/7046430720 - 1149.0*math.Pi/134217728, 2844557431.0/1409286144 + 1149.0*math.Pi/134217728},
	}
	for axis := range 2 {
		for side, v := range volumes {
			src := parabolaCircleApexTangencyStock(t, axis, side)
			for order := range 2 {
				for index, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", axis, side, order, op), func(t *testing.T) {
						result, err := CombineParabolaCircleApexTangencySplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, op, 1e-9, opts)
						if err != nil {
							t.Fatal(err)
						}
						body, h := result.Boundary(), result.History()
						topo := body.Topology()
						if ([3]int{topo.Faces, topo.Edges, topo.Vertices}) != ([3]int{12, 28, 18}) ||
							topo.EulerCharacteristic != 2 || topo.InnerWires != 0 || len(topo.Shells) != 1 {
							t.Fatal("wrong embedded split-roof envelope", topo)
						}
						owners := [4]int{1, 0, side, 1 - side}
						for i, owner := range owners {
							if index == 1 {
								owner = 1 - owner
							}
							if h.RoofOwners[i] != (owner+order)%2 || body.faces[h.RoofFaces[i]].Surface != src[1+owner] {
								t.Fatal("wrong unmodified roof in selected region", i, h)
							}
						}
						apex := geom.Vec3{X: .5, Y: 3. / 8, Z: 2}
						if axis == 1 {
							apex.X, apex.Y = apex.Y, apex.X
						}
						if body.vertices[h.Vertex] != apex || !opposedGraphContourUses(body.faces[side].Outer, body.faces[3].Outer, h.Bridge) {
							t.Fatal("missing exact tangent vertex or interior same-roof seam", h)
						}
						for branch, rays := range h.Rays {
							expectedFace := 1 - side
							if branch == 1 {
								expectedFace = 2
							}
							for _, id := range rays {
								e := body.edges[id]
								if (e.Start == h.Vertex) == (e.End == h.Vertex) ||
									!slices.Contains(nodalCubicIncidentFaces(body, id), expectedFace) {
									t.Fatal("lost an original directed tangent arc or its owner", branch, id)
								}
							}
						}
						mass, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
						if err != nil || math.Abs(mass.Volume-v[index]) > 1e-10 {
							t.Fatal("envelope does not match independent signed integral", mass, v[index], err)
						}
						t.Logf("signed volume %.17g; exact integral %.17g", mass.Volume, v[index])
						if axis == order && side == index {
							checkParabolaCircleApexEnvelopeReplay(t, ctx, result, axis == 0 && side == 0)
						}
					})
				}
			}
		}
	}
}

func checkParabolaCircleApexEnvelopeReplay(t *testing.T, ctx context.Context, result *ParabolaCircleApexTangencyEnvelope, negative bool) {
	t.Helper()
	opts := ParabolaCircleGraphOptions{}
	var raw, again bytes.Buffer
	if err := result.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadParabolaCircleApexTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
	if err != nil || reloaded.History() != result.History() {
		t.Fatal("native replay changed original cusp-ended roof owners", err)
	}
	for i, src := range result.sources {
		if !geom.SameSplineSurface(reloaded.sources[i], src) {
			t.Fatal("native replay changed original support", i)
		}
	}
	if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("native replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if out, err := ReadParabolaCircleApexTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short recipe budget published an incomplete envelope", err)
	}
	var recipe parabolaCircleApexEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Ledger.Bridge = &recipe.Ledger.Circle[0]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := ReadParabolaCircleApexTangencyEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("a forged original curve substituted for the bridge on replay", err)
	}
}
