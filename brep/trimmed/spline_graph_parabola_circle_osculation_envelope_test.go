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

func TestNativeParabolaCircleOsculationEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	volumes := [2][2]float64{
		{54382063.0/27525120 - 33.0*math.Pi/131072, 55181489.0/27525120 + 33.0*math.Pi/131072},
		{54918991.0/27525120 - 33.0*math.Pi/131072, 55718417.0/27525120 + 33.0*math.Pi/131072},
	}
	for axis := range 2 {
		for side, v := range volumes {
			src := parabolaCircleOsculationStock(t, axis, originalOsculationFixture(side), false)
			for order := range 2 {
				for index, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", axis, side, order, op), func(t *testing.T) {
						result, err := CombineParabolaCircleOsculationSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, op, 1e-9, opts)
						if err != nil {
							t.Fatal(err)
						}
						body, h := result.Boundary(), result.History()
						topo := body.Topology()
						if ([3]int{topo.Faces, topo.Edges, topo.Vertices}) != ([3]int{12, 30, 20}) ||
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
						apex := geom.Vec3{X: .5, Y: float64(7-6*side) / 8, Z: 2}
						if axis == 1 {
							apex.X, apex.Y = apex.Y, apex.X
						}
						if body.vertices[h.Vertex] != apex || !opposedGraphContourUses(body.faces[side].Outer, body.faces[3].Outer, h.Bridge) || result.jets != fixtureOsculationJets(side) {
							t.Fatal("missing exact tangent vertex or interior same-roof seam", h)
						}
						for _, x := range [2]float64{.25, .75} {
							y := [2]float64{.75, .25}[side]
							midpoint := geom.Vec3{X: x, Y: y, Z: 2}
							if axis == 1 {
								midpoint.X, midpoint.Y = midpoint.Y, midpoint.X
							}
							if !slices.Contains(body.vertices, midpoint) {
								t.Fatal("lost an exact parabolic subdivision", midpoint)
							}
						}
						bridge := body.edges[h.Bridge]
						first, last := body.vertices[bridge.Start], body.vertices[bridge.End]
						if !osculationBridgeEnds(first, last, axis, side) {
							t.Fatal("bridge did not retain the exact osculating circle extremum and stock wall station", first, last)
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
							checkParabolaCircleOsculationEnvelopeReplay(t, ctx, result, axis == 0 && side == 0)
						}
					})
				}
			}
		}
	}
}

func checkParabolaCircleOsculationEnvelopeReplay(t *testing.T, ctx context.Context, result *ParabolaCircleOsculationEnvelope, negative bool) {
	t.Helper()
	opts := ParabolaCircleGraphOptions{}
	var raw, again bytes.Buffer
	if err := result.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadParabolaCircleOsculationEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
	if err != nil || reloaded.History() != result.History() || reloaded.jets != result.jets {
		t.Fatal("native replay changed original cusp-ended roof owners", err)
	}
	for i, src := range result.envelope.sources {
		if !geom.SameSplineSurface(reloaded.envelope.sources[i], src) {
			t.Fatal("native replay changed original support", i)
		}
	}
	if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("native replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if out, err := ReadParabolaCircleOsculationEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short recipe budget published an incomplete envelope", err)
	}
	var recipe parabolaCircleOsculationEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Osculation.CircleFourth = "0"
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := ReadParabolaCircleOsculationEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("a forged fourth-order contact survived source replay", err)
	}
}

func osculationBridgeEnds(a, b geom.Vec3, axis, side int) bool {
	yFar := float64(3+2*side) / 8
	yWall := float64(side)
	if axis == 1 {
		a.X, a.Y = a.Y, a.X
		b.X, b.Y = b.Y, b.X
	}
	return (a.X == .5 && a.Y == yFar && b.X == .5 && b.Y == yWall) ||
		(b.X == .5 && b.Y == yFar && a.X == .5 && a.Y == yWall)
}

func fixtureOsculationJets(side int) parabolaCircleOsculationRecipeJets {
	sign := 2*side - 1
	return parabolaCircleOsculationRecipeJets{
		SharedSecond: fmt.Sprint(4 * sign), CircleFourth: fmt.Sprint(192 * sign), SubstitutedFourth: "96",
	}
}
