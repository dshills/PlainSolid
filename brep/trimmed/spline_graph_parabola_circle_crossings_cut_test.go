package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaCircleCrossingsCut(t *testing.T) {
	ctx := context.Background()
	const piTerm = 15.0 * math.Pi / 4194304
	volumes := [2][2]float64{
		{237703427.0/56371445760 - piTerm, 4135679.0/28185722880 + piTerm},
		{1263364931.0/56371445760 + piTerm, 4135679.0/28185722880 - piTerm},
	}
	for axis := range 2 {
		stock := parabolaCircleCrossingsStock(t, axis)
		for order, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				result, err := CutParabolaCircleCrossingsSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, 1e-9, ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, history, contacts := result.Components(), result.History(), result.Contacts()
				for i, part := range parts {
					h := history[i]
					region := [2]int{order, 3 - order}[i]
					topo := part.Topology()
					counts := [3]int{topo.Faces, topo.Edges, topo.Vertices}
					if h.Region != region || counts != ([2][3]int{{5, 12, 9}, {2, 3, 3}})[i] ||
						topo.EulerCharacteristic != 2 || len(topo.Shells) != 1 || topo.InnerWires != 0 {
						t.Fatal("wrong positive native region or layer topology", i, h, topo)
					}
					for side, face := range h.RoofFaces {
						if part.faces[face].Surface != stock[1+(order+side)%2] || part.faces[face].Reversed != (side == 1) {
							t.Fatal("component lost original oriented roof support", i, side)
						}
					}
					mass, err := part.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if err != nil || !(mass.Volume > 0 && math.Abs(mass.Volume-want[i]) <= max(1e-12, want[i]*1e-9)) {
						t.Fatal("component disagrees with independent positive integral", i, mass, want[i], err)
					}
					t.Logf("component %d region %d volume %.16g area %.16g", i, h.Region, mass.Volume, mass.Area)
				}
				for i, node := range contacts {
					wantStation := big.NewRat(int64(1+2*i), 4)
					if node.Parameters[axis].Cmp(wantStation) != 0 || node.Parameters[1-axis].Cmp(big.NewRat(49, 128)) != 0 ||
						node.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
						t.Fatal("wrong shared original rational node", node)
					}
					for j, v := range node.Vertices {
						wantPoint := geom.Vec3{X: float64(1+2*i) / 4, Y: 49. / 128, Z: 2}
						if axis == 1 {
							wantPoint.X, wantPoint.Y = wantPoint.Y, wantPoint.X
						}
						if parts[j].vertices[v] != wantPoint || node.Rays[j][0] == node.Rays[j][1] {
							t.Fatal("changed separate vertex or incident sectors", i, j, node)
						}
						for _, id := range node.Rays[j] {
							edge := parts[j].edges[id]
							if (edge.Start == v) == (edge.End == v) {
								t.Fatal("unbound original local ray", i, j, id)
							}
						}
					}
				}
				pairs := 0
				for i, a := range parts[0].vertices {
					for j, b := range parts[1].vertices {
						if a == b {
							pair := [2]int{i, j}
							if pair != contacts[0].Vertices && pair != contacts[1].Vertices {
								t.Fatal("unexpected shared native vertex", a, pair)
							}
							pairs++
						}
					}
				}
				if pairs != 2 {
					t.Fatal("expected exactly two local vertex pairings", pairs)
				}
				checkParabolaCircleCrossingsCutReplay(t, ctx, result, axis == 0 && order == 0)
			})
		}
	}
}

func checkParabolaCircleCrossingsCutReplay(t *testing.T, ctx context.Context, result *ParabolaCircleCrossingsCut, negative bool) {
	t.Helper()
	opts := ParabolaCircleGraphOptions{}
	var raw, again bytes.Buffer
	if err := result.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadParabolaCircleCrossingsCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
	if err != nil || replayed.History() != result.History() || !reflect.DeepEqual(replayed.Contacts(), result.Contacts()) {
		t.Fatal("original Cut replay changed the component contact graph", err)
	}
	for i, surface := range result.sources {
		if !geom.SameSplineSurface(surface, replayed.sources[i]) {
			t.Fatal("Cut replay changed an original support", i)
		}
	}
	if err := replayed.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("replayed Cut changed canonical recipe bytes", err)
	}
	if !negative {
		return
	}
	if replayed, err := ReadParabolaCircleCrossingsCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{
		MaxEntities: polynomialGraphEntityCount(result.components[:1]),
	}, opts); replayed != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short entity budget published one of two retained solids", err)
	}
	var recipe parabolaCircleCrossingsCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Contacts[1].Vertices[0] = recipe.Contacts[0].Vertices[0]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if replayed, err := ReadParabolaCircleCrossingsCutJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); replayed != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("a forged contact linking different source nodes passed replay", err)
	}
}
