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

func TestNativeParabolaCirclePocketDiskCut(t *testing.T) {
	ctx := context.Background()
	// Independent exact Green integrals of the authored height law
	// on each original region. The full disk uses polar moments.
	volumes := [7]float64{.030875782339388487, .159320601907153, 2.3438490860857207e-10,
		5.0614246903741087e-10, 5.0614246903741087e-10, .016688146338439059, 3.86470158726482e-15}
	for layout := range 2 {
		for axis := range 2 {
			src := parabolaCirclePocketDiskStocks(t, axis, layout == 1, twoCircleProbe(256, 84, 1, 512), big.NewRat(245, 16), false)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					t.Parallel()
					roofs := [2]*geom.BSplineSurface{src[1+order], src[2-order]}
					out, err := CutParabolaCirclePocketDiskSplineGraphs(ctx, src[0], roofs[0], roofs[1], axis, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					want := []int{1, 2, 3, 4}
					if layout == order {
						want = []int{0, 5, 6}
					}
					bodies, history := out.Components(), out.History()
					if len(bodies) != len(want) || len(history) != len(want) ||
						out.DiskSignedVolumePiCoefficient().Cmp(expectedPocketDiskVolumePiCoefficient(t, layout, order)) != 0 {
						t.Fatal("changed original positive regions or exact signed disk volume")
					}
					for i, part := range bodies {
						region, h := want[i], history[i]
						if h.Region != region || out.circles != ([2]int{1 - layout, layout}) {
							t.Fatal("changed original circle or positive component order", h)
						}
						checkNativePocketDiskCutComponent(t, part, h, roofs, axis, layout == 1)
						props, err := part.MassProperties(ctx, parametric.IntegrationOptions{
							RelativeTolerance: 1e-6, MaxSubdivisions: 128, MaxSamples: 16_000_000,
						})
						ref := volumes[region] / float64(1+layout)
						discrepancy := math.Abs(props.Volume-ref) / ref
						if err != nil || props.Volume <= 0 || discrepancy > 1e-7 {
							t.Fatal("wrong signed material for original positive region", region, props, ref, err)
						}
						t.Logf("region %d signed volume %.17g independent reference %.17g relative discrepancy %.9g",
							region, props.Volume, ref, discrepancy)
					}
					checkNativePocketDiskCutContacts(t, out, axis, layout == 1)
					checkNativePocketDiskCutReplay(t, out, axis == 0 && layout == 0)
				})
			}
		}
	}
}

func checkNativePocketDiskCutComponent(t *testing.T, part Solid, h ParabolaCirclePocketDiskCutHistory,
	roofs [2]*geom.BSplineSurface, axis int, reflected bool) {
	t.Helper()
	topo := part.Topology()
	walls, holes, euler := 0, 0, 2
	switch h.Region {
	case 0, 1:
		walls = 3
	case 2:
		holes, euler = 2, 0
		if topo.Edges != 8 || topo.Vertices != 8 {
			t.Fatal("wrong original annulus topology", topo)
		}
	case 6:
		if topo.Edges != 4 || topo.Vertices != 4 {
			t.Fatal("wrong separate original disk topology", topo)
		}
	}
	if len(topo.Shells) != 1 || topo.CollapsedEdges != 0 || topo.EulerCharacteristic != euler || topo.InnerWires != holes ||
		topo.Faces != 2+walls || len(h.WallFaces) != walls || len(h.DomainSides) != walls || len(h.CircleEdges) != 10 {
		t.Fatal("changed native positive component topology", h.Region, topo)
	}
	for owner, id := range h.RoofFaces {
		face := part.faces[id]
		if face.Surface != roofs[owner] || face.Reversed != (owner == 1) || len(face.Holes) != holes/2 {
			t.Fatal("changed original Cut roof, orientation or hole", h.Region, owner)
		}
	}
	if h.Region != 2 && h.Region != 6 {
		missing := [4]int{-1, -1, -1, -1}
		if h.DiskEdges != missing || h.DiskVertices != missing {
			t.Fatal("introduced a disk rim into a disjoint source region", h)
		}
		return
	}
	var center [2]*big.Rat
	y := big.NewRat(21, 128)
	if reflected {
		y.Sub(big.NewRat(1, 1), y)
	}
	center[axis], center[1-axis] = big.NewRat(1, 2), y
	for i, radial := range [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
		point := [3]*big.Rat{new(big.Rat).Add(center[0], big.NewRat(radial[0], 512)),
			new(big.Rat).Add(center[1], big.NewRat(radial[1], 512)), big.NewRat(2, 1)}
		v, err := exactParabolaCircleCrossingVertex(part, point)
		if err != nil || h.DiskVertices[i] != v {
			t.Fatal("lost original separate circle point", i, v, err)
		}
		id := h.DiskEdges[i]
		edge := part.edges[id]
		if !((edge.Start == v && edge.End == h.DiskVertices[(i+1)%4]) ||
			(edge.End == v && edge.Start == h.DiskVertices[(i+1)%4])) {
			t.Fatal("changed exact original rim incidence", i, id)
		}
		for _, face := range h.RoofFaces {
			loop := part.faces[face].Outer
			if h.Region == 2 {
				loop = part.faces[face].Holes[0]
			}
			if !slices.ContainsFunc(loop, func(use Use) bool { return use.Edge == id }) {
				t.Fatal("missing opposite original rim use", i, face)
			}
		}
	}
}

func checkNativePocketDiskCutContacts(t *testing.T, out *ParabolaCirclePocketDiskCut, axis int, reflected bool) {
	t.Helper()
	bodies, history := out.Components(), out.History()
	want := [4][2]int{{0, 5}, {0, 5}, {0, 5}, {0, 5}}
	if len(bodies) == 4 {
		want = [4][2]int{{1, 3}, {2, 3}, {2, 4}, {1, 4}}
	}
	for i, c := range out.Contacts() {
		x := big.NewRat([4]int64{15, 23, 41, 49}[i], 64)
		y := big.NewRat([4]int64{77, 51, 51, 77}[i], 256)
		if reflected {
			y.Sub(big.NewRat(1, 1), y)
		}
		regions := [2]int{history[c.Components[0]].Region, history[c.Components[1]].Region}
		if regions != want[i] || c.Parameters[axis].Cmp(x) != 0 || c.Parameters[1-axis].Cmp(y) != 0 ||
			c.Point[axis].Cmp(x) != 0 || c.Point[1-axis].Cmp(y) != 0 || c.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("wrong original contact, cycle or disk isolation", i, c)
		}
		for j, component := range c.Components {
			v, err := exactParabolaCircleCrossingVertex(bodies[component], c.Point)
			a, b := c.Rays[j][0], c.Rays[j][1]
			h := history[component]
			if err != nil || v != c.Vertices[j] || slices.Contains(h.DiskVertices[:], v) ||
				!(slices.Contains(h.ParabolaEdges[:], a) && slices.Contains(h.CircleEdges, b) ||
					slices.Contains(h.ParabolaEdges[:], b) && slices.Contains(h.CircleEdges, a)) {
				t.Fatal("missing exact separate node and both original occupying rays", i, j, err)
			}
		}
	}
}

func checkNativePocketDiskCutReplay(t *testing.T, out *ParabolaCirclePocketDiskCut, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, canonical bytes.Buffer
	if err := out.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaCirclePocketDiskCut, error) {
		return ReadParabolaCirclePocketDiskCutJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	replay, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(replay.History(), out.History()) || !reflect.DeepEqual(replay.Contacts(), out.Contacts()) ||
		replay.DiskSignedVolumePiCoefficient().Cmp(out.DiskSignedVolumePiCoefficient()) != 0 {
		t.Fatal("failed exact replay of nested Cut ownership, source material or contacts", err)
	}
	for i, source := range out.sources {
		if !geom.SameSplineSurface(source, replay.sources[i]) {
			t.Fatal("changed source during nested Cut replay", i)
		}
	}
	if err := replay.WriteJSON(ctx, &canonical); err != nil || !bytes.Equal(raw.Bytes(), canonical.Bytes()) {
		t.Fatal("changed nested Cut recipe bytes", err)
	}
	if !negative {
		return
	}
	var forged parabolaCirclePocketDiskCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &forged); err != nil {
		t.Fatal(err)
	}
	if len(out.Components()) == 3 {
		// Join the isolated disk to the near exterior.
		forged.Contacts[0][0].Components = []int{0, 2}
	} else {
		// Add a chord from the far exterior to the nested pocket.
		forged.Contacts[0][0].Components = []int{0, 1}
	}
	bad, err := json.Marshal(forged)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := read(bad, JSONLimits{}); got != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a forged original contact", got, err)
	}
	if err := json.Unmarshal(raw.Bytes(), &forged); err != nil {
		t.Fatal(err)
	}
	forged.Arrangement.DiskSignedVolumePiCoefficient = "0"
	bad, err = json.Marshal(forged)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := read(bad, JSONLimits{}); got != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a forged exact thin disk volume", got, err)
	}
	if got, err := read(raw.Bytes(), JSONLimits{MaxEntities: 1}); got != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay published a partial shell assembly beyond its budget", got, err)
	}
}

func TestNativeParabolaCirclePocketDiskCutRefusals(t *testing.T) {
	ctx := context.Background()
	circles := []twoCircleStockProbe{twoCircleProbe(256, 256, 205, 512), twoCircleProbe(32, 5, 1, 64)}
	p := [3]*big.Rat{big.NewRat(4543, 8192), big.NewRat(-2, 1), big.NewRat(2, 1)}
	src := parabolaCircleProductStock(t, 0, circles, p, big.NewRat(245, 16), false)
	for order := range 2 {
		out, err := CutParabolaCirclePocketDiskSplineGraphs(ctx, src[0], src[1+order], src[2-order], 0, 1e-9, ParabolaCircleGraphOptions{})
		if out != nil || !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "exact cap arc subdivision search exhausted") {
			t.Fatal("published a nested difference with an unresolved exact cap", out, err)
		}
	}
	src = parabolaCirclePocketDiskStocks(t, 0, false, twoCircleProbe(256, 84, 1, 512), big.NewRat(245, 16), false)
	options := ParabolaCircleGraphOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 32}}
	out, err := CutParabolaCirclePocketDiskSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, options)
	if out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("published beyond its exact proof budget", out, err)
	}
}
