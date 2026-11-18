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

func TestNativeParabolaThreeCrossingCirclesCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	// Independent exact Green integrals of the original signed height
	// difference and of xF, yF, and 2F + F²/2 over each source region.
	volumes := [8]float64{-0.00703729405055491067, 0.215964571105412792, 0.0000423399559057812389, -1.07368103148297229e-7,
		6.54578781458051735e-7, -0.00000172990403775222024, 0.0000423399559057812389, -1.07368103148297229e-7}
	centroids := [8][3]float64{
		{.5, .142435316492205880, 1.94789058425296224}, {.5, .915056360245264939, 2.60345804940508890},
		{.143585762824483886, .305878649639524423, 2.00086536119669024}, {.251672037424203111, .400610519249148634, 1.99998409083664786},
		{.5, .206047121278161370, 2.00002567784678283}, {.5, .332840483667458644, 1.99995546368029996},
		{.856414237175516114, .305878649639524423, 2.00086536119669024}, {.748327962575796889, .400610519249148634, 1.99998409083664786},
	}
	for layout := range 2 {
		reflected := layout == 1
		for axis := range 2 {
			src := allCrossingParabolaStocks(t, axis, allCrossingParabolaCircles(reflected), reflected, false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					t.Parallel()
					roofs := [2]*geom.BSplineSurface{src[1+order], src[2-order]}
					r, err := CutParabolaThreeCrossingCirclesSplineGraphs(ctx, src[0], roofs[0], roofs[1], axis, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					bodies, histories := r.Components(), r.History()
					regions := [4]int{order, 3 - order, 5 - order, 7 - order}
					for k, body := range bodies {
						h, region := histories[k], regions[k]
						topo := body.Topology()
						if h.Region != region || len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
							topo.InnerWires != 0 || topo.Faces != 2+len(h.WallFaces) ||
							(k == 0 && len(h.WallFaces) != 3) || (k > 0 && len(h.WallFaces) != 0) {
							t.Fatal("changed positive native source topology", k, topo, h)
						}
						for i, id := range h.RoofFaces {
							roof := body.faces[id]
							if roof.Surface != roofs[i] || roof.Reversed != (i == 1) {
								t.Fatal("lost original difference roof owner", k, i)
							}
						}
						volume, centroid := math.Abs(volumes[region]), centroids[region]
						if reflected {
							volume, centroid = math.Abs(volumes[region^1])/2, centroids[region^1]
							centroid[1], centroid[2] = 1-centroid[1], 3-centroid[2]/2
						}
						if axis == 1 {
							centroid[0], centroid[1] = centroid[1], centroid[0]
						}
						props, err := body.MassProperties(ctx, parametric.IntegrationOptions{
							RelativeTolerance: 1e-6, MaxSubdivisions: 128, MaxSamples: 16_000_000,
						})
						center := geom.Vec3{X: centroid[0], Y: centroid[1], Z: centroid[2]}
						if err != nil || props.Volume <= 0 || math.Abs(props.Volume-volume) > volume*1e-7 ||
							props.Centroid.Distance(center) > 1e-7 {
							t.Fatal("native Cut layer differs from independent exact source integrals", k, props, volume, center, err)
						}
						t.Logf("region %d, positive volume %.17g reference %.17g, relative discrepancy %.9g, centroid discrepancy %.9g",
							region, props.Volume, volume, math.Abs(props.Volume-volume)/volume, props.Centroid.Distance(center))
					}
					checkThreeCrossingCircleCutContacts(t, r, axis, reflected)
					if order == axis {
						checkThreeCrossingCircleCutReplay(t, r, axis == 0 && !reflected)
					}
				})
			}
		}
	}
}

func checkThreeCrossingCircleCutContacts(t *testing.T, r *ParabolaThreeCrossingCirclesCut, axis int, reflected bool) {
	t.Helper()
	components, h := r.Components(), r.History()
	positions := [3][2][2]int64{{{3, 57}, {5, 41}}, {{6, 36}, {10, 36}}, {{11, 41}, {13, 57}}}
	var exterior []int
	for i, pair := range r.Contacts() {
		for j, c := range pair {
			x, y := big.NewRat(positions[i][j][0], 16), big.NewRat(positions[i][j][1], 128)
			if reflected {
				y.Sub(big.NewRat(1, 1), y)
			}
			if c.Components != [2]int{0, i + 1} || c.Parameters[axis].Cmp(x) != 0 || c.Parameters[1-axis].Cmp(y) != 0 ||
				c.Point[axis].Cmp(x) != 0 || c.Point[1-axis].Cmp(y) != 0 || c.Point[2].Cmp(big.NewRat(2, 1)) != 0 ||
				slices.Contains(exterior, c.Vertices[0]) || (j == 1 && c.Vertices[1] == pair[0].Vertices[1]) {
				t.Fatal("wrong original separate contact identity", i, j, c)
			}
			exterior = append(exterior, c.Vertices[0])
			for owner, body := range c.Components {
				vertex, err := exactParabolaCircleCrossingVertex(components[body], c.Point)
				if err != nil || c.Vertices[owner] != vertex {
					t.Fatal("contact missing exact original vertex", i, j, owner, err)
				}
				rays := c.Rays[owner]
				if rays[0] == rays[1] || !(slices.Contains(h[body].ParabolaEdges[:], rays[0]) && slices.Contains(h[body].CircleEdges[i], rays[1]) ||
					slices.Contains(h[body].ParabolaEdges[:], rays[1]) && slices.Contains(h[body].CircleEdges[i], rays[0])) {
					t.Fatal("contact lost one of its original cap or exterior coedges", i, j, owner, rays)
				}
			}
		}
	}
	contacts := r.Contacts()
	contacts[0][0].Point[2].SetInt64(0)
	if r.Contacts()[0][0].Point[2].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("contact accessor changed immutable source")
	}
}

func checkThreeCrossingCircleCutReplay(t *testing.T, r *ParabolaThreeCrossingCirclesCut, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte) (*ParabolaThreeCrossingCirclesCut, error) {
		return ReadParabolaThreeCrossingCirclesCutJSON(ctx, bytes.NewReader(data), JSONLimits{}, ParabolaCircleGraphOptions{})
	}
	replayed, err := read(raw.Bytes())
	if err != nil || !reflect.DeepEqual(replayed.History(), r.History()) || !reflect.DeepEqual(replayed.Contacts(), r.Contacts()) {
		t.Fatal("replay changed original native components or contacts", err)
	}
	for i, source := range r.sources {
		if !geom.SameSplineSurface(source, replayed.sources[i]) {
			t.Fatal("replay changed original stock", i)
		}
	}
	if err := replayed.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("replay changed canonical original bytes", err)
	}
	if !negative {
		return
	}
	var recipe parabolaCrossingCirclesCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Contacts[2][1].Components[1] = 2
	bad, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(bad); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a forged original component contact", out, err)
	}
}
