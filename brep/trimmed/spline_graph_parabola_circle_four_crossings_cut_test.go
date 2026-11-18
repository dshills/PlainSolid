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

func TestNativeParabolaCircleFourCrossingsCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	// Independent exact integrals of F, xF, yF, and 2F+F²/2 over the
	// authored original regions. The two caps retain positive volumes
	// below one billionth even though the exit points are exact.
	volumes := [6]float64{0.0062935814591662947, 0.0088153959330863688, 4.2984944571218707e-9,
		3.5155308778047057e-10, 3.5155308778047057e-10, 0.0032601449111889372}
	centers := [6][3]float64{
		{.5, .12212290182322239, 1.9750346849425124}, {.5, .90711678109650351, 2.0237147942434097},
		{.5, .16729363186420873, 2.0000020702329984}, {.2876823969857627, .24837436506916186, 2.0000006016263092},
		{.7123176030142373, .24837436506916186, 2.0000006016263092}, {.5, .56052399989687907, 1.9923283722439837},
	}
	for layout := range 2 {
		reflected := layout != 0
		for axis := range 2 {
			src := fourCrossingParabolaCircleStock(t, axis, reflected, false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					t.Parallel()
					roofs := [2]*geom.BSplineSurface{src[1+order], src[2-order]}
					r, err := CutParabolaCircleFourCrossingsSplineGraphs(ctx, src[0], roofs[0], roofs[1], axis, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					bodies, history := r.Components(), r.History()
					want := []int{0, 5}
					if order != layout {
						want = []int{1, 2, 3, 4}
					}
					if len(bodies) != len(want) || len(history) != len(want) {
						t.Fatal("changed exact material component count", len(bodies))
					}
					for i, body := range bodies {
						h, region := history[i], want[i]
						walls := 0
						if region < 2 {
							walls = 3
						}
						topo := body.Topology()
						if h.Region != region || len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
							topo.InnerWires != 0 || topo.Faces != 2+walls || len(h.WallFaces) != walls || len(h.CircleEdges) != 10 {
							t.Fatal("changed separate source topology", region, topo, h)
						}
						for i, id := range h.RoofFaces {
							f := body.faces[id]
							if f.Surface != roofs[i] || f.Reversed != (i == 1) {
								t.Fatal("changed original Cut roof owner or orientation", region, i)
							}
						}

						center := centers[region]
						if reflected {
							center[1], center[2] = 1-center[1], 4-center[2]
						}
						if axis == 1 {
							center[0], center[1] = center[1], center[0]
						}
						props, err := body.MassProperties(ctx, parametric.IntegrationOptions{
							RelativeTolerance: 1e-6, MaxSubdivisions: 128, MaxSamples: 16_000_000,
						})
						wantVolume := volumes[region]
						volumeError := math.Abs(props.Volume-wantVolume) / wantVolume
						centroidError := props.Centroid.Distance(geom.Vec3{X: center[0], Y: center[1], Z: center[2]})
						if err != nil || props.Volume <= 0 || volumeError > 1e-7 || centroidError > 1e-7 {
							t.Fatal("native positive layer differs from original exact integral", region, props, wantVolume, center, err)
						}
						t.Logf("region %d, positive volume %.17g reference %.17g, relative discrepancy %.9g, centroid discrepancy %.9g",
							region, props.Volume, wantVolume, volumeError, centroidError)
					}
					checkNativeFourCrossingCircleCutContacts(t, r, axis, reflected)
					negative := !reflected && axis == 0 && order == 1
					if axis == order || negative {
						checkNativeFourCrossingCircleCutReplay(t, r, negative)
					}
				})
			}
		}
	}
}

func checkNativeFourCrossingCircleCutContacts(t *testing.T, r *ParabolaCircleFourCrossingsCut, axis int, reflected bool) {
	t.Helper()
	bodies, h := r.Components(), r.History()
	pairs := [4][2]int{{0, 5}, {0, 5}, {0, 5}, {0, 5}}
	if len(bodies) == 4 {
		pairs = [4][2]int{{1, 3}, {2, 3}, {2, 4}, {1, 4}}
	}
	used := make([][]int, len(bodies))
	for i, c := range r.Contacts() {
		x := big.NewRat([4]int64{15, 23, 41, 49}[i], 64)
		y := big.NewRat([4]int64{77, 51, 51, 77}[i], 256)
		if reflected {
			y.Sub(big.NewRat(1, 1), y)
		}
		if c.Components[0] >= c.Components[1] || [2]int{h[c.Components[0]].Region, h[c.Components[1]].Region} != pairs[i] ||
			c.Parameters[axis].Cmp(x) != 0 || c.Parameters[1-axis].Cmp(y) != 0 ||
			c.Point[axis].Cmp(x) != 0 || c.Point[1-axis].Cmp(y) != 0 || c.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("changed original four-contact incidence", i, c)
		}
		for j, id := range c.Components {
			v, err := exactParabolaCircleCrossingVertex(bodies[id], c.Point)
			if err != nil || v != c.Vertices[j] || slices.Contains(used[id], v) {
				t.Fatal("missing separate original contact vertex", i, j, v, err)
			}
			used[id] = append(used[id], v)
			first, last := c.Rays[j][0], c.Rays[j][1]
			if first == last || !(slices.Contains(h[id].ParabolaEdges[:], first) && slices.Contains(h[id].CircleEdges, last) ||
				slices.Contains(h[id].ParabolaEdges[:], last) && slices.Contains(h[id].CircleEdges, first)) {
				t.Fatal("lost one of the two original sector rays", i, j, c.Rays[j])
			}
		}
	}
	contacts := r.Contacts()
	contacts[1].Point[2].SetInt64(0)
	if r.Contacts()[1].Point[2].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("mutable contact copy altered original source")
	}
}

func checkNativeFourCrossingCircleCutReplay(t *testing.T, r *ParabolaCircleFourCrossingsCut, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaCircleFourCrossingsCut, error) {
		return ReadParabolaCircleFourCrossingsCutJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	replayed, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(replayed.History(), r.History()) || !reflect.DeepEqual(replayed.Contacts(), r.Contacts()) {
		t.Fatal("replay changed original exact contact graph or owners", err)
	}
	for i, original := range r.sources {
		if !geom.SameSplineSurface(original, replayed.sources[i]) {
			t.Fatal("replay changed an original source", i)
		}
	}
	if err := replayed.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	var forged parabolaCrossingCirclesCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &forged); err != nil {
		t.Fatal(err)
	}
	// Forge a contact between the far bulk and the bounded pocket.
	forged.Contacts[0][0].Components = []int{0, 1}
	bad, err := json.Marshal(forged)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(bad, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a forged cycle chord", out, err)
	}
	if out, err := read(raw.Bytes(), JSONLimits{MaxEntities: 1}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay published beyond its native result budget", out, err)
	}
}

func TestNativeParabolaCircleFourCrossingsCutUnrepresentableCap(t *testing.T) {
	src := parabolaCircleCurvatureStock(t, 0, big.NewRat(1, 2), big.NewRat(447, 8192), big.NewRat(42025, 262144), big.NewRat(2, 1), false)
	for order := range 2 {
		out, err := CutParabolaCircleFourCrossingsSplineGraphs(context.Background(), src[0], src[1+order], src[2-order], 0, 1e-9, ParabolaCircleGraphOptions{})
		if out != nil || !errors.Is(err, geom.ErrConvergence) {
			t.Fatal("published a partial Cut with an unresolved original cap", out, err)
		}
	}
}
