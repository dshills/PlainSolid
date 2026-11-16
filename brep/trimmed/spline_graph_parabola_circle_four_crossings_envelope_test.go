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
	"slices"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaCircleFourCrossingsEnvelope(t *testing.T) {
	ctx := context.Background()
	independentVolume := [2]float64{1.9904462736296447, 2.008815400934687}
	independentArea := [2][2]float64{{9.9092364774890704, 10.111133914954703}, {9.916041055442772, 10.122303353816424}}
	for layout := range 2 {
		reflected := layout != 0
		volume := independentVolume
		if reflected {
			volume = [2]float64{4 - volume[1], 4 - volume[0]}
		}
		for axis := range 2 {
			src := fourCrossingParabolaCircleStock(t, axis, reflected, false)
			for order := range 2 {
				for i, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", layout, axis, order, i), func(t *testing.T) {
						t.Parallel()
						a, b := src[1+order], src[2-order]
						r, err := CombineParabolaCircleFourCrossingsSplineGraphs(ctx, src[0], a, b, axis, op, 1e-9, ParabolaCircleGraphOptions{})
						if err != nil {
							t.Fatal(err)
						}
						body, h := r.Boundary(), r.History()
						topo := body.Topology()
						if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 || topo.InnerWires != 0 ||
							topo.Faces != 13 || topo.Edges != 36 || topo.Vertices != 25 || len(h.CircleEdges) != 10 {
							t.Fatal("wrong native pocket-and-caps topology", topo, h)
						}
						want := [6]int{1, 0, 0, 0, 0, 1}
						for j := range want {
							want[j] ^= i ^ order ^ layout
						}
						if h.Base != 6 || body.faces[h.Base].Surface != src[0] || h.RoofOwners != want {
							t.Fatal("changed original roof or base assignment", h)
						}
						roofs := [2]*geom.BSplineSurface{a, b}
						for j, id := range h.RoofFaces {
							if body.faces[id].Surface != roofs[want[j]] {
								t.Fatal("lost original native roof support", j)
							}
						}
						checkNativeFourCrossingCircleFans(t, r, axis, reflected)
						props, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
						if err != nil || math.Abs(props.Volume-volume[i]) > 1e-10 || math.Abs(props.Area-independentArea[layout][i]) > 1e-8 {
							t.Fatal("native shell differs from independently integrated source", props, volume[i], independentArea[layout][i], err)
						}
						t.Logf("signed volume %.17g, exact reference %.17g, physical area %.17g, independent reference %.17g",
							props.Volume, volume[i], props.Area, independentArea[layout][i])
						if axis == order {
							checkNativeFourCrossingCircleReplay(t, r, layout == 0 && axis == 0 && i == 0)
						}
					})
				}
			}
		}
	}
}

func checkNativeFourCrossingCircleFans(t *testing.T, r *ParabolaCircleFourCrossingsEnvelope, axis int, reflected bool) {
	t.Helper()
	body, h := r.Boundary(), r.History()
	const n = 4
	var seen []int
	for i, node := range h.Junctions {
		if slices.Contains(seen, node.Vertex) {
			t.Fatal("native shell welded distinct source junctions", i)
		}
		seen = append(seen, node.Vertex)
		far := 1
		if i == 1 || i == 2 {
			far = 2
		}
		sectors := node.Sectors
		slices.Sort(sectors[:])
		if sectors != [n]int{0, far, 3 + i/2, 5} {
			t.Fatal("native fan misbound pocket or cap", i, node)
		}
		for j, edge := range node.Rays {
			owners := h.ParabolaEdges[:]
			if j%2 == 1 {
				owners = h.CircleEdges
			}
			before, after := node.Sectors[(j+3)%4], node.Sectors[j]
			e := body.edges[edge]
			if !slices.Contains(owners, edge) || (e.Start != node.Vertex && e.End != node.Vertex) ||
				!opposedGraphContourUses(body.faces[before].Outer, body.faces[after].Outer, edge) {
				t.Fatal("native fan lost its original directed coedge", i, j, edge)
			}
		}
	}
	points := [3][2]*big.Rat{{big.NewRat(77, 256), big.NewRat(60, 256)},
		{big.NewRat(179, 256), big.NewRat(60, 256)}, {big.NewRat(1, 2), big.NewRat(327, 2048)}}
	for i, uv := range points {
		if reflected {
			uv[1].Sub(big.NewRat(1, 1), uv[1])
		}
		point := [3]*big.Rat{nil, nil, big.NewRat(2, 1)}
		point[axis], point[1-axis] = uv[0], uv[1]
		vertex, err := exactParabolaCircleCrossingVertex(body, point)
		if err != nil || slices.Contains(seen, vertex) {
			t.Fatal("lost exact cap or pocket subdivision", i, point, err)
		}
	}
}

func checkNativeFourCrossingCircleReplay(t *testing.T, r *ParabolaCircleFourCrossingsEnvelope, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaCircleFourCrossingsEnvelope, error) {
		return ReadParabolaCircleFourCrossingsEnvelopeJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	replayed, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(replayed.History(), r.History()) {
		t.Fatal("native replay changed the original circle, pocket or fans", err)
	}
	for i, original := range r.sources {
		if !geom.SameSplineSurface(original, replayed.sources[i]) {
			t.Fatal("native replay changed an original support", i)
		}
	}
	if err := replayed.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("native replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	var changed parabolaCrossingCirclesEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	changed.Ledger.Junctions[0][1].Sectors[0] = 4
	bad, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(bad, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("native replay accepted a forged cap or pocket sector", out, err)
	}
	if out, err := read(raw.Bytes(), JSONLimits{MaxEntities: 1}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("native replay published an oversized source shell", out, err)
	}
}

func TestNativeParabolaCircleFourCrossingsUnrepresentableCap(t *testing.T) {
	ctx := context.Background()
	// The original circle of radius 205/512 has two consecutive lattice
	// vertices (156, -133) and (164, -123) on its lower-right cap, in
	// units of 1/512 about its center. No binary64 point lies between
	// them on the arc. A finer binary lattice would force both numerators
	// even by reducing the circle equation modulo four.
	for axis := range 2 {
		src := parabolaCircleCurvatureStock(t, axis, big.NewRat(1, 2), big.NewRat(447, 8192), big.NewRat(42025, 262144), big.NewRat(2, 1), false)
		roofs := parabolaTwoCirclesPatches(t, src[1], src[2])
		proof, err := geom.DiscoverSplineGraphParabolaCircleFourCrossings(ctx, roofs[0], roofs[1], axis, geom.GraphPointContactOptions{})
		if err != nil || proof.Nodes[2].Parameters[axis].Cmp(big.NewRat(103, 128)) != 0 {
			t.Fatal("missing independently exact four-crossing circle", proof, err)
		}
		for _, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
			out, err := CombineParabolaCircleFourCrossingsSplineGraphs(ctx, src[0], src[1], src[2], axis, op, 1e-9, ParabolaCircleGraphOptions{})
			if out != nil || !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "exact cap arc subdivision search exhausted") {
				t.Fatal("native shell should refuse an unresolved original cap", out, err)
			}
		}
	}
}
