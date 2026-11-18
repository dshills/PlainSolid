//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaThreeCrossingCirclesEnvelope(t *testing.T) {
	slowtest.Skip(t)
	original := [2]float64{
		969374254391585495.0/486388759756013568 - 2837911685.0*math.Pi/211106232532992,
		1077841223684269609.0/486388759756013568 + 2837911685.0*math.Pi/211106232532992,
	}
	independentArea := [2][2]float64{{9.871562034071971, 14.521596924453510}, {9.604106574899760, 10.123528027615702}}
	ctx := context.Background()
	for layout := range 2 {
		reflected := layout == 1
		mass := original
		if reflected {
			// Reflect the original regions and negate and halve the height
			// difference. This exchanges the original positive and negative integrals.
			mass = [2]float64{2 - (original[1]-2)/2, 2 - (original[0]-2)/2}
		}
		for axis := range 2 {
			src := allCrossingParabolaStocks(t, axis, allCrossingParabolaCircles(reflected), reflected, false)
			for order := range 2 {
				for i, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", layout, axis, order, i), func(t *testing.T) {
						t.Parallel()
						a, b := src[1+order], src[2-order]
						r, err := CombineParabolaThreeCrossingCirclesSplineGraphs(ctx, src[0], a, b, axis, op, 1e-9, ParabolaCircleGraphOptions{})
						if err != nil {
							t.Fatal(err)
						}
						solid, h := r.Boundary(), r.History()
						topo := solid.Topology()
						if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
							topo.Faces != 15 || topo.Edges != 40 || topo.Vertices != 27 || topo.InnerWires != 0 {
							t.Fatal("wrong eight-region native topology", topo)
						}
						owners := [8]int{1, 0, 0, 1, 0, 1, 0, 1}
						for j := range owners {
							owners[j] ^= order ^ i
						}
						if h.Base != 8 || h.RoofOwners != owners || solid.faces[h.Base].Surface != src[0] {
							t.Fatal("changed original native material assignment", h)
						}
						originals := [2]*geom.BSplineSurface{a, b}
						for j, id := range h.RoofFaces {
							if solid.faces[id].Surface != originals[owners[j]] {
								t.Fatal("native face lost original support", j, id)
							}
						}
						checkThreeCrossingCircleNativeFans(t, solid, h)
						props, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
						if err != nil || math.Abs(props.Volume-mass[i]) > 1e-10 || math.Abs(props.Area-independentArea[layout][i]) > 1e-8 {
							t.Fatal("native shell differs from independently integrated stock", props, mass[i], independentArea[layout][i], err)
						}
						t.Logf("signed volume %.17g, exact reference %.17g, physical area %.17g, independent reference %.17g",
							props.Volume, mass[i], props.Area, independentArea[layout][i])
						if order == axis {
							checkThreeCrossingCircleNativeReplay(t, r, layout == 0 && axis == 0 && i == 0)
						}
					})
				}
			}
		}
	}
}

func checkThreeCrossingCircleNativeFans(t *testing.T, s Solid, h ParabolaThreeCrossingCirclesHistory) {
	t.Helper()
	vertices := make(map[int]bool, 6)
	for i, circle := range h.Junctions {
		if len(h.CircleEdges[i]) != 4 {
			t.Fatal("lost original cardinal circle chain", h.CircleEdges[i])
		}
		for j, node := range circle {
			if vertices[node.Vertex] {
				t.Fatal("two original junctions share a native vertex", i, j)
			}
			vertices[node.Vertex] = true
			sectors := node.Sectors
			slices.Sort(sectors[:])
			if sectors != [4]int{0, 1, 2 + 2*i, 3 + 2*i} {
				t.Fatal("missing native junction sector", i, j, node)
			}
			for k, id := range node.Rays {
				original := h.ParabolaEdges[:]
				if k%2 == 1 {
					original = h.CircleEdges[i]
				}
				before, after := node.Sectors[(k+3)%4], node.Sectors[k]
				edge := s.edges[id]
				if !slices.Contains(original, id) || (edge.Start != node.Vertex && edge.End != node.Vertex) ||
					!opposedGraphContourUses(s.faces[before].Outer, s.faces[after].Outer, id) {
					t.Fatal("changed original native fan ray", i, j, k, id)
				}
			}
		}
	}
}

func checkThreeCrossingCircleNativeReplay(t *testing.T, r *ParabolaThreeCrossingCirclesEnvelope, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaThreeCrossingCirclesEnvelope, error) {
		return ReadParabolaThreeCrossingCirclesEnvelopeJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	replayed, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(replayed.History(), r.History()) {
		t.Fatal("replay changed original native incidence", err)
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
	var recipe parabolaCrossingCirclesEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Ledger.Junctions[1][1].Rays[1] = recipe.Ledger.CircleEdges[2][0]
	bad, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(bad, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a forged original ray", out, err)
	}
	if out, err := read(raw.Bytes(), JSONLimits{MaxControlPoints: 1}); out != nil || err == nil {
		t.Fatal("replay published under an exhausted original pole limit", out, err)
	}
}

func TestNativeParabolaThreeCrossingCirclesUnrepresentableJunction(t *testing.T) {
	ctx := context.Background()
	// Apply the exact similarity scale 2/3 and translation (1/6,-1/24)
	// to the original arrangement. The new unit-chart source poles and
	// parabola exits are exact, but the first crossing station is 7/24.
	circles := []twoCircleStockProbe{
		twoCircleProbe(56, 33, 16, 192), twoCircleProbe(24, 7, 4, 48), twoCircleProbe(136, 33, 16, 192),
	}
	src := parabolaCircleProductStock(t, 0, circles, [3]*big.Rat{big.NewRat(7, 8), big.NewRat(-3, 1), big.NewRat(3, 1)}, big.NewRat(-33075, 8192), false)
	patches := parabolaTwoCirclesPatches(t, src[1], src[2])
	p, err := geom.DiscoverSplineGraphParabolaThreeCrossingCircles(ctx, patches[0], patches[1], 0, geom.GraphPointContactOptions{})
	if err != nil || p.Nodes[0][0].Parameters[0].Cmp(big.NewRat(7, 24)) != 0 {
		t.Fatal("missing exact but non-binary original source geometry", p, err)
	}
	for _, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
		out, err := CombineParabolaThreeCrossingCirclesSplineGraphs(ctx, src[0], src[1], src[2], 0, op, 1e-9, ParabolaCircleGraphOptions{})
		if out != nil || !errors.Is(err, geom.ErrUnrepresentable) {
			t.Fatal("published a shell with a rounded original junction corner", out, err)
		}
	}
}
