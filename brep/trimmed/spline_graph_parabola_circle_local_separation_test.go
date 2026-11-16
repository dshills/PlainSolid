package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
)

const (
	localUpperLens = 8399735575.0 * math.Pi / 3458764513820540928
	localLowerLens = 36822589825.0 * math.Pi / 442721857769029238784
)

func TestNativeParabolaCircleLocalSeparationEnvelopes(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	circles := localOppositeCircleStock()
	for axis := range 2 {
		src := parabolaTwoCirclesStock(t, axis, circles, false)
		for order := range 2 {
			for _, c := range []struct {
				name  string
				op    parametric.BooleanOp
				owner [4]int
				mass  float64
			}{
				{"common", parametric.Intersection, [4]int{1, 0, 1, 0}, 33957950114251843.0/16980857579372544 - localUpperLens - localLowerLens},
				{"fuse", parametric.Union, [4]int{0, 1, 0, 1}, 34079095217310893.0/16980857579372544 + localUpperLens + localLowerLens},
			} {
				t.Run(fmt.Sprintf("%d/%d/%s", axis, order, c.name), func(t *testing.T) {
					result, err := CombineParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, c.op, 1e-9, opts)
					if err != nil {
						t.Fatal(err)
					}
					body, h := result.Boundary(), result.History()
					topo := body.Topology()
					if h.EnclosingRoof != -1 || h.CircleEnclosingRoofs != ([2]int{1, 0}) || topo.InnerWires != 2 ||
						topo.Faces != 11 || topo.Edges != 27 || topo.Vertices != 20 || topo.EulerCharacteristic != 2 || len(topo.Shells) != 1 {
						t.Fatal("same-ordinate disks changed original opposite roof nesting", h, topo)
					}
					for i, face := range h.RoofFaces {
						if h.RoofOwners[i] != (c.owner[i]+order)%2 || body.faces[face].Surface != src[1+c.owner[i]] {
							t.Fatal("source roof changed", i, face, h)
						}
					}
					for i, edges := range h.CircleEdges {
						checkNativeOffsetCircleEdges(t, body, axis, edges[:], circles[i])
						roof := body.faces[h.CircleEnclosingRoofs[i]]
						if len(roof.Holes) != 1 || len(roof.Holes[0]) != len(edges) {
							t.Fatal("original circular wire assigned to the wrong roof", i, h)
						}
						for j, use := range roof.Holes[0] {
							if use.Edge != edges[j] {
								t.Fatal("original directed circle owner changed", i, j, h)
							}
						}
					}
					checkLocalParabolaCircleVolume(t, ctx, body, c.mass)
					if axis != order {
						return
					}
					var raw, again bytes.Buffer
					if err := result.WriteJSON(ctx, &raw); err != nil {
						t.Fatal(err)
					}
					reloaded, err := ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
					if err != nil || reloaded.History() != h {
						t.Fatal("source replay changed exact local hole ownership", err)
					}
					checkParabolaTwoCirclesReplaySources(t, result.sources, reloaded.sources)
					if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
						t.Fatal("source replay changed canonical envelope bytes", err)
					}
				})
			}
		}
	}
}

func TestNativeParabolaCircleLocalSeparationCuts(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	circles := localOppositeCircleStock()
	wantVolumes := [2][2]float64{
		{3765044493245.0/16980857579372544 + localLowerLens, localUpperLens},
		{117380058565805.0/16980857579372544 + localUpperLens, localLowerLens},
	}
	for axis := range 2 {
		src := parabolaTwoCirclesStock(t, axis, circles, false)
		for order, want := range wantVolumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				result, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				parts, h := result.Components(), result.History()
				if len(parts) != 2 || len(h) != 2 {
					t.Fatal("expected an exterior layer and a separate opposing lens", h)
				}
				for i, part := range parts {
					topo := part.Topology()
					if h[i].Region != ([2]int{order, 2 + order})[i] || len(h[i].CircleEdges) != 4 ||
						len(topo.Shells) != 1 || topo.EulerCharacteristic != 2*i || topo.InnerWires != 2-2*i ||
						topo.Faces != ([2]int{5, 2})[i] || topo.Edges != ([2]int{13, 4})[i] || topo.Vertices != ([2]int{10, 4})[i] {
						t.Fatal("wrong exact local material component", i, h[i], topo)
					}
					for owner, face := range h[i].RoofFaces {
						if part.faces[face].Surface != src[1+(owner+order)%2] || part.faces[face].Reversed != (owner == 1) {
							t.Fatal("source Cut coedge owner changed", i, owner, face)
						}
					}
					checkNativeOffsetCircleEdges(t, part, axis, h[i].CircleEdges, circles[([2]int{1 - order, order})[i]])
					checkLocalParabolaCircleVolume(t, ctx, part, want[i])
				}
				for _, p := range parts[0].vertices {
					for _, q := range parts[1].vertices {
						if p == q {
							t.Fatal("separate source closures unexpectedly share a vertex", p)
						}
					}
				}
				var raw, again bytes.Buffer
				if err := result.WriteJSON(ctx, &raw); err != nil {
					t.Fatal(err)
				}
				reloaded, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
				if err != nil || !reflect.DeepEqual(h, reloaded.History()) {
					t.Fatal("source replay changed exact local Cut ownership", err)
				}
				checkParabolaTwoCirclesReplaySources(t, result.sources, reloaded.sources)
				if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
					t.Fatal("source replay changed canonical Cut bytes", err)
				}
			})
		}
	}
}

func checkLocalParabolaCircleVolume(t *testing.T, ctx context.Context, s Solid, expected float64) {
	t.Helper()
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil || !(mass.Volume > 0 && math.Abs(mass.Volume-expected) <= max(3e-15, expected*1e-10)) {
		t.Fatal("signed material differs from the independent exact integral", mass.Volume, expected, err)
	}
	t.Logf("volume %.17g; exact integral %.17g", mass.Volume, expected)
}
