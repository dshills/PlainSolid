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
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaCirclePocketDiskEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	volumes := [2]float64{1.9524360713221687, 2.159320603153823}
	areas := [2][2]float64{{9.7482283627751123, 13.592188631621307}, {9.6024932436292865, 10.474984021412073}}
	for layout := range 2 {
		reflected := layout != 0
		volume := volumes
		if reflected {
			volume = [2]float64{2 - (volumes[1]-2)/2, 2 - (volumes[0]-2)/2}
		}
		for axis := range 2 {
			src := parabolaCirclePocketDiskStocks(t, axis, reflected, twoCircleProbe(256, 84, 1, 512), big.NewRat(245, 16), false)
			checkPocketDiskThicknessAtCenter(t, src, axis, reflected)
			for order := range 2 {
				for i, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", layout, axis, order, i), func(t *testing.T) {
						t.Parallel()
						roofs := [2]*geom.BSplineSurface{src[1+order], src[2-order]}
						r, err := CombineParabolaCirclePocketDiskSplineGraphs(ctx, src[0], roofs[0], roofs[1], axis, op, 1e-9, ParabolaCircleGraphOptions{})
						if err != nil {
							t.Fatal(err)
						}
						body, h := r.Boundary(), r.History()
						topo := body.Topology()
						if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
							topo.Faces != 14 || topo.Edges != 40 || topo.Vertices != 29 || topo.InnerWires != 1 ||
							h.Base != 7 || h.PocketCircle != layout || h.CrossingCircle != 1-layout || h.DiskFace != 6 {
							t.Fatal("wrong native pocket and disk topology", topo, h)
						}
						if body.faces[h.Base].Surface != src[0] {
							t.Fatal("lost original base")
						}
						want := [6]int{1, 0, 0, 0, 0, 1}
						for j, id := range h.RoofFaces {
							want[j] ^= i ^ order ^ layout
							holes := 0
							if j == 2 {
								holes = 1
							}
							if h.RoofOwners[j] != want[j] || body.faces[id].Surface != roofs[want[j]] || len(body.faces[id].Holes) != holes {
								t.Fatal("changed original roof owner or original pocket hole", j)
							}
						}
						if h.DiskSignedVolumePiCoefficient.Cmp(expectedPocketDiskVolumePiCoefficient(t, layout, order)) != 0 {
							t.Fatal("changed native disk's exact signed source volume", h.DiskSignedVolumePiCoefficient)
						}
						if h.DiskOwner == h.RoofOwners[2] || body.faces[h.DiskFace].Surface != roofs[h.DiskOwner] {
							t.Fatal("changed exact disk owner", h)
						}
						checkNativeFourCrossingCircleFans(t, body, h.ParabolaCircleFourCrossingsHistory, axis, reflected)
						checkNativePocketDiskSeam(t, body, h, axis, reflected)
						props, err := body.MassProperties(ctx, parametric.IntegrationOptions{
							RelativeTolerance: 1e-6, MaxSubdivisions: 128, MaxSamples: 16_000_000,
						})
						if err != nil || math.Abs(props.Volume-volume[i]) > 1e-10 || math.Abs(props.Area-areas[layout][i]) > 1e-8 {
							t.Fatal("native signed material differs from independently integrated stock", props, volume[i], areas[layout][i], err)
						}
						t.Logf("signed volume %.17g, exact reference %.17g, physical area %.17g, independent reference %.17g",
							props.Volume, volume[i], props.Area, areas[layout][i])
						if axis == order {
							checkNativePocketDiskEnvelopeReplay(t, r, axis == 0 && !reflected && i == 0)
						}
					})
				}
			}
		}
	}
}

func checkPocketDiskThicknessAtCenter(t *testing.T, src [3]*geom.BSplineSurface, axis int, reflected bool) {
	t.Helper()
	// Independent substitution of the nested disk center in the exact
	// source product. The full disk's positive layer magnitude is
	// 1115390675*Pi/906694364710971881029632, halved by reflection.
	want := big.NewRat(-377055, 562949953421312)
	y := big.NewRat(21, 128)
	if reflected {
		y.Sub(big.NewRat(1, 1), y)
		want.Mul(want, big.NewRat(-1, 2))
	}
	var uv [2]*big.Rat
	uv[axis], uv[1-axis] = big.NewRat(1, 2), y
	patches := parabolaTwoCirclesPatches(t, src[1], src[2])
	var height [2]*big.Rat
	for i, p := range patches {
		jet, err := p.EvaluateRational(context.Background(), uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		height[i] = jet.Point[2]
	}
	if new(big.Rat).Sub(height[1], height[0]).Cmp(want) != 0 {
		t.Fatal("thin isolated source disk height changed", height, want)
	}
}

func checkNativePocketDiskSeam(t *testing.T, body Solid, h ParabolaCirclePocketDiskHistory, axis int, reflected bool) {
	t.Helper()
	pocket, disk := body.faces[h.RoofFaces[2]], body.faces[h.DiskFace]
	for _, id := range h.DiskEdges {
		if !opposedGraphContourUses(pocket.Holes[0], disk.Outer, id) ||
			slices.Contains(h.CircleEdges, id) || slices.Contains(h.ParabolaEdges[:], id) {
			t.Fatal("lost exact inner and outer coedge pairing", id)
		}
	}
	center := [2]*big.Rat{}
	center[axis], center[1-axis] = big.NewRat(1, 2), big.NewRat(21, 128)
	if reflected {
		center[1-axis].Sub(big.NewRat(1, 1), center[1-axis])
	}
	for i, direction := range [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
		point := [3]*big.Rat{nil, nil, big.NewRat(2, 1)}
		for j, d := range direction {
			point[j] = new(big.Rat).Add(center[j], big.NewRat(d, 512))
		}
		id, err := exactParabolaCircleCrossingVertex(body, point)
		if err != nil || h.DiskVertices[i] != id {
			t.Fatal("lost an exact original small circle vertex", i, err)
		}
		for _, node := range h.Junctions {
			if node.Vertex == id {
				t.Fatal("welded the disjoint disk to a transverse junction", i, id)
			}
		}
	}
}

func checkNativePocketDiskEnvelopeReplay(t *testing.T, r *ParabolaCirclePocketDiskEnvelope, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaCirclePocketDiskEnvelope, error) {
		return ReadParabolaCirclePocketDiskEnvelopeJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	replayed, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(replayed.History(), r.History()) {
		t.Fatal("replay changed original pocket or disk incidence", err)
	}
	for i, source := range r.sources {
		if !geom.SameSplineSurface(source, replayed.sources[i]) {
			t.Fatal("source replay changed an original roof", i)
		}
	}
	if err := replayed.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("original pocket recipe changed its bytes", err)
	}
	if !negative {
		return
	}
	var recipe parabolaCirclePocketDiskEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	diskEdge := recipe.Arrangement.DiskEdges[0]
	recipe.Arrangement.DiskEdges[0] = recipe.Ledger.CircleEdges[0][0]
	bad, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(bad, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("accepted a substituted circle trim as the pocket disk", out, err)
	}
	recipe.Arrangement.DiskEdges[0] = diskEdge
	recipe.Arrangement.DiskSignedVolumePiCoefficient = "0"
	bad, err = json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(bad, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("accepted a changed thin source volume proof", out, err)
	}
	if out, err := read(raw.Bytes(), JSONLimits{MaxEntities: 1}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("published a partial nested result under a short limit", out, err)
	}
}

func TestNativeParabolaCirclePocketDiskUnrepresentableCap(t *testing.T) {
	ctx := context.Background()
	circles := []twoCircleStockProbe{twoCircleProbe(256, 256, 205, 512), twoCircleProbe(32, 5, 1, 64)}
	p := [3]*big.Rat{big.NewRat(4543, 8192), big.NewRat(-2, 1), big.NewRat(2, 1)}
	src := parabolaCircleProductStock(t, 0, circles, p, big.NewRat(245, 16), false)
	patches := parabolaTwoCirclesPatches(t, src[1], src[2])
	if _, err := geom.DiscoverSplineGraphParabolaCirclePocketDisk(ctx, patches[0], patches[1], 0, geom.GraphPointContactOptions{}); err != nil {
		t.Fatal("expected exact nested source geometry", err)
	}
	for _, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
		out, err := CombineParabolaCirclePocketDiskSplineGraphs(ctx, src[0], src[1], src[2], 0, op, 1e-9, ParabolaCircleGraphOptions{})
		if out != nil || !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "exact cap arc subdivision search exhausted") {
			t.Fatal("published a nested envelope with an unresolved original cap", out, err)
		}
	}
}
