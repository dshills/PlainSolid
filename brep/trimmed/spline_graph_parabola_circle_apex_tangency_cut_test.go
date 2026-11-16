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

func TestNativeParabolaCircleApexTangencyCut(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	volumes := [2][2]float64{
		{57435031.0 / 1409286144, 1155.0 * math.Pi / 134217728},
		{64080467.0 / 7046430720, 1149.0 * math.Pi / 134217728},
	}
	for axis := range 2 {
		for side, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, side), func(t *testing.T) {
				src := parabolaCircleApexTangencyStock(t, axis, side)
				result, err := CutParabolaCircleApexTangencySplineGraphs(ctx, src[0], src[2-side], src[1+side], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				parts, h, apex := result.Components(), result.History(), result.Contact()
				if h[0].Region != 1-side || h[1].Region != 2 {
					t.Fatal("wrong separate positive source regions", h)
				}
				point := geom.Vec3{X: .5, Y: 3. / 8, Z: 2}
				if axis == 1 {
					point.X, point.Y = point.Y, point.X
				}
				for part, body := range parts {
					topology := body.Topology()
					if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.InnerWires != 0 ||
						([3]int{topology.Faces, topology.Edges, topology.Vertices}) != ([2][3]int{{5, 10, 7}, {2, 4, 4}})[part] ||
						body.vertices[apex.Vertices[part]] != point || apex.Rays[part][0] == apex.Rays[part][1] {
						t.Fatal("wrong embedded component or exact apex identity", part, topology, apex)
					}
					for i, face := range h[part].RoofFaces {
						roof := body.faces[face]
						if roof.Surface != src[([2]int{2 - side, 1 + side})[i]] || roof.Reversed != (i == 1) {
							t.Fatal("changed source roof owner", part, i)
						}
					}
					for _, id := range apex.Rays[part] {
						edge := body.edges[id]
						v := apex.Vertices[part]
						if (edge.Start == v) == (edge.End == v) {
							t.Fatal("lost an original apex arc", part, id)
						}
					}
					mass, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if err != nil || !(mass.Volume > 0 && math.Abs(mass.Volume-want[part]) < max(1e-13, want[part]*1e-9)) {
						t.Fatal("component differs from the independent exact integral", part, mass, want[part], err)
					}
					t.Logf("component %d volume %.17g; exact integral %.17g", part, mass.Volume, want[part])
				}
				shared := 0
				for i, first := range parts[0].vertices {
					for j, second := range parts[1].vertices {
						if first == second {
							if ([2]int{i, j}) != apex.Vertices {
								t.Fatal("unexpected shared vertex in independent shells", first)
							}
							shared++
						}
					}
				}
				if shared != 1 {
					t.Fatal("expected exactly one coupled vertex pair", shared)
				}
				checkParabolaCircleApexCutReplay(t, ctx, result, axis == side)
				refused, err := CutParabolaCircleApexTangencySplineGraphs(ctx, src[0], src[1+side], src[2-side], axis, 1e-9, opts)
				if refused != nil || !errors.Is(err, geom.ErrDegenerate) {
					t.Fatal("same-shell tangent pinch published partial volume", refused, err)
				}
			})
		}
	}
}

func checkParabolaCircleApexCutReplay(t *testing.T, ctx context.Context, result *ParabolaCircleApexTangencyCut, negative bool) {
	t.Helper()
	opts := ParabolaCircleGraphOptions{}
	var raw, again bytes.Buffer
	if err := result.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadParabolaCircleApexTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
	if err != nil || !reflect.DeepEqual(reloaded.History(), result.History()) || !reflect.DeepEqual(reloaded.Contact(), result.Contact()) {
		t.Fatal("native replay changed apex contact owners", err)
	}
	for i, src := range result.sources {
		if !geom.SameSplineSurface(reloaded.sources[i], src) {
			t.Fatal("native replay changed original roof or base", i)
		}
	}
	if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("native replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if out, err := ReadParabolaCircleApexTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{
		MaxEntities: polynomialGraphEntityCount(result.components[:1]),
	}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short recipe budget published a partial apex Cut", err)
	}
	var recipe parabolaCircleApexCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Contact.Rays[1][0], recipe.Contact.Rays[1][1] = recipe.Contact.Rays[1][1], recipe.Contact.Rays[1][0]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := ReadParabolaCircleApexTangencyCutJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("a forged tangent branch direction passed native replay", err)
	}
}
