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

func TestNativeParabolaMixedCirclesCut(t *testing.T) {
	ctx := context.Background()
	volumes := [2][2][]float64{
		{
			{2793913668193235.0/8694199080638742528 - 68480808725.0*math.Pi/72057594037927936,
				79148254988215.0/8694199080638742528 + 105062125.0*math.Pi/105553116266496},
			{41843838384235475.0/8694199080638742528 + 105062125.0*math.Pi/105553116266496,
				79148254988215.0/8694199080638742528 - 105062125.0*math.Pi/105553116266496,
				9724805825.0 * math.Pi / 216172782113783808},
		},
		{
			{6874310365806835.0/8694199080638742528 + 88781875.0*math.Pi/105553116266496,
				99498870437015.0/8694199080638742528 - 88781875.0*math.Pi/105553116266496,
				18988606175.0 * math.Pi / 216172782113783808},
			{14006678396434675.0/8694199080638742528 - 54278891275.0*math.Pi/72057594037927936,
				99498870437015.0/8694199080638742528 + 88781875.0*math.Pi/105553116266496},
		},
	}
	for axis := range 2 {
		for side, orders := range volumes {
			src := parabolaMixedCirclesStock(t, axis, mixedCirclePrimaryStock(side), false)
			for order, want := range orders {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, side, order), func(t *testing.T) {
					r, err := CutParabolaMixedCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					parts, history, contacts := r.Components(), r.History(), r.Contacts()
					if len(parts) != len(want) || len(history) != len(parts) {
						t.Fatal("wrong number of positive source shells", len(parts))
					}
					for i, part := range parts {
						expected := [3][5]int{{5, 12, 9, 0, 2}, {2, 3, 3, 0, 2}, {2, 4, 4, 0, 2}}[i]
						if i == 0 && side == order {
							expected = [5]int{5, 16, 13, 2, 0}
						}
						topo := part.Topology()
						if ([5]int{topo.Faces, topo.Edges, topo.Vertices, topo.InnerWires, topo.EulerCharacteristic}) != expected ||
							len(topo.Shells) != 1 || topo.CollapsedEdges != 0 {
							t.Fatal("wrong exact topology for positive source region", i, topo)
						}
						if i == 0 && axis == 1 && side == 0 && order == 0 && part.minimumLevel != 2 {
							t.Fatal("transposed holed cap lost its validated finer seed")
						}
						h := history[i]
						if h.Region != ([3]int{order, 3 - order, 4})[i] {
							t.Fatal("wrong original signed region selected", i, h)
						}
						for owner, id := range h.RoofFaces {
							face := part.faces[id]
							if face.Surface != src[1+(owner+order)%2] || face.Reversed != (owner == 1) {
								t.Fatal("changed original owner of a positive Cut cap", id)
							}
						}
						expectedCircle := i == 2 || (i == 0 && side == order)
						for j, edge := range h.SeparatedEdges {
							if (edge >= 0) != expectedCircle || (h.SeparatedVertices[j] >= 0) != expectedCircle {
								t.Fatal("separated rim assigned to wrong positive component", h)
							}
						}
						mass, err := part.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7, MaxSubdivisions: 32})
						if err != nil || !(mass.Volume > 0) || math.Abs(mass.Volume-want[i]) > 1e-12 {
							t.Fatal("signed component volume differs from independent analytic source", mass, want[i], err)
						}
						t.Logf("component %d volume %.17g; exact integral %.17g", i, mass.Volume, want[i])
					}
					for node, c := range contacts {
						point := geom.Vec3{X: float64(2*node+1) / 4, Y: 49.0 / 128, Z: 2}
						if axis == 1 {
							point.X, point.Y = point.Y, point.X
						}
						if c.Components != ([2]int{0, 1}) || parts[0].vertices[c.Vertices[0]] != point || parts[1].vertices[c.Vertices[1]] != point ||
							(len(parts) == 3 && slices.Contains(parts[2].vertices, point)) {
							t.Fatal("changed the exact component-local crossing graph", c)
						}
					}
					checkMixedCircleCutReplay(t, r, axis == 0 && side == 1 && order == 0)
				})
			}
		}
	}
}

func checkMixedCircleCutReplay(t *testing.T, r *ParabolaMixedCirclesCut, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaMixedCirclesCut, error) {
		return ReadParabolaMixedCirclesCutJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	out, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(out.History(), r.History()) || !reflect.DeepEqual(out.Contacts(), r.Contacts()) {
		t.Fatal("native replay changed contact graph or source owners", err)
	}
	for i, source := range r.sources {
		if !geom.SameSplineSurface(source, out.sources[i]) {
			t.Fatal("native replay changed original source support", i)
		}
	}
	if err := out.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("native replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if out, err := read(raw.Bytes(), JSONLimits{MaxEntities: polynomialGraphEntityCount(r.components[:2])}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("native short budget published a partial Cut", out, err)
	}
	var recipe parabolaMixedCirclesCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Contacts[0].Components[1] = 2
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(forged, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("source replay admitted a false contact to the independent lens", out, err)
	}
}
