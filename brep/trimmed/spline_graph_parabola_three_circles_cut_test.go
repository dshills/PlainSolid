//go:build research

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

func TestNativeParabolaThreeCirclesCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	fixtures := [2]twoCircleStockProbe{twoCircleProbe(16, 6, 1, 32), twoCircleProbe(16, 12, 1, 32)}
	for side, disk := range fixtures {
		for axis := range 2 {
			stock := parabolaThreeCircleStocks(t, axis, disk, false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", side, axis, order), func(t *testing.T) {
					roof := [2]*geom.BSplineSurface{stock[1+order], stock[2-order]}
					cut, err := CutParabolaThreeCirclesSplineGraphs(ctx, stock[0], roof[0], roof[1], axis, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					if cut.Arrangement() != (ParabolaThreeCirclesCutArrangement{CrossingCircles: [2]int{0, 2}, SeparatedCircle: 1, SeparatedRegion: side}) {
						t.Fatal("changed the original factor indices or separated side", cut.Arrangement())
					}
					parts, history := cut.Components(), cut.History()
					regions := []int{order, 3 - order, 5 - order}
					if side != order {
						regions = append(regions, 6)
					}
					if len(parts) != len(regions) || len(parts) != len(history) {
						t.Fatal("wrong positive layer count", len(parts), len(history))
					}
					for i, part := range parts {
						region := regions[i]
						topo, h := part.Topology(), history[i]
						holed := i == 0 && side == order
						chi, holes, walls := 2, 0, 0
						if holed {
							chi, holes = 0, 2
						}
						if i == 0 {
							walls = 3
						}
						if h.Region != region || len(topo.Shells) != 1 || topo.EulerCharacteristic != chi ||
							topo.InnerWires != holes || topo.CollapsedEdges != 0 || len(h.WallFaces) != walls ||
							len(h.DomainSides) != walls {
							t.Fatal("changed embedded source layer topology", h, topo)
						}
						for owner, face := range h.RoofFaces {
							if part.faces[face].Surface != roof[owner] || part.faces[face].Reversed != (owner == 1) {
								t.Fatal("changed original operand or orientation", h)
							}
						}
						for j, id := range h.SeparatedEdges {
							present := holed || region == 6
							if (id >= 0) != present || (h.SeparatedVertices[j] >= 0) != present {
								t.Fatal("separated ring moved to another positive layer", h)
							}
							if present {
								point := part.vertices[h.SeparatedVertices[j]]
								if point.Z != 2 || !edgeVertex(part.edges[id], h.SeparatedVertices[j]) {
									t.Fatal("separated edge lost its original vertex", id, point)
								}
							}
						}
						checkThreeCirclesCutMoments(t, ctx, part, side, axis, region)
					}
					checkThreeCirclesCutContacts(t, cut, parts, axis)
					checkThreeCirclesCutReplay(t, cut, side == 1 && axis == 0 && order == 0)
				})
			}
		}
	}
}

func edgeVertex(edge parametric.Edge, vertex int) bool {
	return edge.Start == vertex || edge.End == vertex
}

func checkThreeCirclesCutContacts(t *testing.T, cut *ParabolaThreeCirclesCut, parts []Solid, axis int) {
	t.Helper()
	xs := [2][2]float64{{3.0 / 16, 5.0 / 16}, {11.0 / 16, 13.0 / 16}}
	ys := [2][2]float64{{57.0 / 128, 41.0 / 128}, {41.0 / 128, 57.0 / 128}}
	all := make(map[geom.Vec3][2]int)
	for circle, group := range cut.Contacts() {
		for i, node := range group {
			point := geom.Vec3{X: xs[circle][i], Y: ys[circle][i], Z: 2}
			if axis == 1 {
				point.X, point.Y = point.Y, point.X
			}
			if node.Components != ([2]int{0, circle + 1}) {
				t.Fatal("changed original exterior/cap pair", node)
			}
			for slot, partID := range node.Components {
				body := parts[partID]
				vertex := node.Vertices[slot]
				if body.vertices[vertex] != point || node.Rays[slot][0] == node.Rays[slot][1] {
					t.Fatal("changed original component-local contact", node)
				}
				for _, id := range node.Rays[slot] {
					if !edgeVertex(body.edges[id], vertex) {
						t.Fatal("recorded contact ray lost its original endpoint", node, id)
					}
				}
			}
			if _, repeated := all[point]; repeated {
				t.Fatal("duplicate source contact", node)
			}
			all[point] = node.Components
		}
	}
	for i, a := range parts {
		for j := i + 1; j < len(parts); j++ {
			for _, ap := range a.vertices {
				for _, bp := range parts[j].vertices {
					if ap == bp && all[ap] != ([2]int{i, j}) {
						t.Fatal("unexpected shared vertex between retained shells", i, j, ap)
					}
				}
			}
		}
	}
}

func checkThreeCirclesCutReplay(t *testing.T, original *ParabolaThreeCirclesCut, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := original.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaThreeCirclesCut, error) {
		return ReadParabolaThreeCirclesCutJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	got, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(got.History(), original.History()) || !reflect.DeepEqual(got.Contacts(), original.Contacts()) ||
		got.Arrangement() != original.Arrangement() {
		t.Fatal("changed material or contact lineage on replay", err)
	}
	for i, source := range original.sources {
		if !geom.SameSplineSurface(source, got.sources[i]) {
			t.Fatal("changed an original ordered roof on replay", i)
		}
	}
	if err := got.WriteJSON(ctx, &again); err != nil || !bytes.Equal(again.Bytes(), raw.Bytes()) {
		t.Fatal("changed canonical bytes", err)
	}
	if !negative {
		return
	}
	var recipe parabolaThreeCirclesCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Contacts[0][0].Components[1] = 3
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := read(forged, JSONLimits{}); result != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a false contact with the independent disk", result, err)
	}
	if result, err := read(raw.Bytes(), JSONLimits{MaxEntities: 1}); result != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay returned partial material at the entity limit", result, err)
	}
}

// Independent polynomial/Green integrals of the original signed height
// and first moment densities. The chart order is free, dependent, height.
var threeCirclesCutMoments = [2][7][4]float64{
	{
		{0.00678122243255969570, 0.5, 0.168188974812030560, 1.95278685208598724},
		{0.284379425887643229, 0.500000000000000000, 0.912120994267185441, 2.76920383603676785},
		{0.0000531525239913044500, 0.145607348187151451, 0.308235419736783278, 2.00105026655797374},
		{1.91551808399345852e-7, 0.253475497085342930, 0.399138631871108292, 1.99997229531724007},
		{0.0000531525239913044500, 0.854392651812848549, 0.308235419736783278, 2.00105026655797374},
		{1.91551808399345852e-7, 0.746524502914657070, 0.399138631871108292, 1.99997229531724007},
		{1.81667256547182025e-8, 0.500000000000000000, 0.184070882767056523, 2.00000412232862397},
	},
	{
		{0.00902861162935336052, 0.500000000000000000, 0.118667221360838677, 1.93522895050430479},
		{0.170850479767487537, 0.500000000000000000, 0.916563708110362181, 2.48719511916285153},
		{0.0000500542166867359336, 0.146191016797282915, 0.301997445297336246, 2.00100210020946927},
		{1.11245215674448245e-7, 0.253177673418470688, 0.398896125020156964, 1.99998401498261847},
		{0.0000500542166867359336, 0.853808983202717085, 0.301997445297336246, 2.00100210020946927},
		{1.11245215674448245e-7, 0.746822326581529312, 0.398896125020156964, 1.99998401498261847},
		{2.59073038281078706e-8, 0.5, 0.376729736982559076, 1.99999429225904779},
	},
}

func checkThreeCirclesCutMoments(t *testing.T, ctx context.Context, part Solid, side, axis, region int) {
	t.Helper()
	reference := threeCirclesCutMoments[side][region]
	expected := geom.Vec3{X: reference[1], Y: reference[2], Z: reference[3]}
	if axis == 1 {
		expected.X, expected.Y = expected.Y, expected.X
	}
	props, err := part.MassProperties(ctx, parametric.IntegrationOptions{
		RelativeTolerance: 1e-6, MaxSubdivisions: 128, MaxSamples: 16000000,
	})
	if err != nil || props.Volume <= 0 || math.Abs(props.Volume-reference[0]) > min(1e-10, 1e-6*reference[0]) ||
		props.Centroid.Sub(expected).Length() > 1e-7 {
		t.Fatal("native positive volume or centroid differs from original signed moments", region, props, reference, err)
	}
	t.Logf("region %d volume %.17g centroid %+v", region, props.Volume, props.Centroid)
}

func TestNativeParabolaThreeCirclesCutSeparatedIndex(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	// This separated disk precedes both crossing circles in source order.
	// Reversing the stock order makes its signed layer positive.
	stock := parabolaThreeCircleStocks(t, 1, twoCircleProbe(2, 4, 1, 32), false)
	cut, err := CutParabolaThreeCirclesSplineGraphs(ctx, stock[0], stock[2], stock[1], 1, 1e-9, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := ParabolaThreeCirclesCutArrangement{CrossingCircles: [2]int{1, 2}, SeparatedCircle: 0, SeparatedRegion: 0}
	parts, history := cut.Components(), cut.History()
	if cut.Arrangement() != want || len(parts) != 4 || history[3].Region != 6 {
		t.Fatal("lost the distinct original separated circle index", cut.Arrangement(), history)
	}
	populated := 0
	for _, edge := range history[3].SeparatedEdges {
		if edge >= 0 {
			populated++
		}
	}
	if populated != 4 {
		t.Fatal("independent separated layer lost an original circular edge", history[3])
	}
	checkThreeCirclesCutContacts(t, cut, parts, 1)
	checkThreeCirclesCutReplay(t, cut, false)
}
