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

func TestNativeParabolaCircleObliqueTangencyCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	volumes := [2][2]float64{
		{20734973.0 / 1006632960, 36148125.0 * math.Pi / 35184372088832},
		{156503579.0 / 7046430720, 35851875.0 * math.Pi / 35184372088832},
	}
	for axis := range 2 {
		for side, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, side), func(t *testing.T) {
				src := parabolaCircleObliqueStock(t, axis, originalObliqueCircleFixture(side), false)
				result, err := CutParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[2-side], src[1+side], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				parts, h, contact := result.Components(), result.History(), result.Contact()
				if h[0].Region != 1-side || h[1].Region != 2 {
					t.Fatal("wrong separate positive source regions", h)
				}
				point := geom.Vec3{X: .5, Y: .5, Z: 2}
				if axis == 1 {
					point.X, point.Y = point.Y, point.X
				}
				for part, body := range parts {
					topology := body.Topology()
					if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.InnerWires != 0 ||
						([3]int{topology.Faces, topology.Edges, topology.Vertices}) != ([2][3]int{{5, 10, 7}, {2, 4, 4}})[part] ||
						body.vertices[contact.Vertices[part]] != point || contact.Rays[part][0] == contact.Rays[part][1] {
						t.Fatal("wrong embedded component or exact contact identity", part, topology, contact)
					}
					for i, face := range h[part].RoofFaces {
						roof := body.faces[face]
						if roof.Surface != src[([2]int{2 - side, 1 + side})[i]] || roof.Reversed != (i == 1) {
							t.Fatal("changed source roof owner", part, i)
						}
					}
					for _, id := range contact.Rays[part] {
						edge := body.edges[id]
						v := contact.Vertices[part]
						if (edge.Start == v) == (edge.End == v) {
							t.Fatal("lost an original contact arc", part, id)
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
							if ([2]int{i, j}) != contact.Vertices {
								t.Fatal("unexpected shared vertex in independent shells", first)
							}
							shared++
						}
					}
				}
				if shared != 1 {
					t.Fatal("expected exactly one coupled vertex pair", shared)
				}
				checkParabolaCircleObliqueCutReplay(t, ctx, result, axis == side)
				refused, err := CutParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[1+side], src[2-side], axis, 1e-9, opts)
				if refused != nil || !errors.Is(err, geom.ErrDegenerate) {
					t.Fatal("same-shell tangent pinch published partial volume", refused, err)
				}
			})
		}
	}
}

func checkParabolaCircleObliqueCutReplay(t *testing.T, ctx context.Context, result *ParabolaCircleObliqueTangencyCut, negative bool) {
	t.Helper()
	opts := ParabolaCircleGraphOptions{}
	var raw, again bytes.Buffer
	if err := result.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadParabolaCircleObliqueTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
	if err != nil || !reflect.DeepEqual(reloaded.History(), result.History()) || !reflect.DeepEqual(reloaded.Contact(), result.Contact()) {
		t.Fatal("native replay changed contact contact owners", err)
	}
	for i, src := range result.cut.sources {
		if !geom.SameSplineSurface(reloaded.cut.sources[i], src) {
			t.Fatal("native replay changed original roof or base", i)
		}
	}
	if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("native replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if out, err := ReadParabolaCircleObliqueTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{
		MaxEntities: polynomialGraphEntityCount(result.cut.components[:1]),
	}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short recipe budget published a partial contact Cut", err)
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
	if out, err := ReadParabolaCircleObliqueTangencyCutJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("a forged tangent branch direction passed native replay", err)
	}
}
