package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaMixedCirclesEnvelope(t *testing.T) {
	ctx := context.Background()
	options := ParabolaCircleGraphOptions{}
	volumes := [2][2]float64{
		{20262849766147207.0/10133099161583616 - 9724805825.0*math.Pi/216172782113783808,
			6771686537652179.0/3377699720527872 + 9724805825.0*math.Pi/216172782113783808},
		{20258070340374407.0/10133099161583616 - 18988606175.0*math.Pi/216172782113783808,
			6760879696404179.0/3377699720527872 + 18988606175.0*math.Pi/216172782113783808},
	}
	for axis := range 2 {
		for side, v := range volumes {
			src := parabolaMixedCirclesStock(t, axis, mixedCirclePrimaryStock(side), false)
			for order := range 2 {
				for index, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", axis, side, order, op), func(t *testing.T) {
						r, err := CombineParabolaMixedCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, op, 1e-9, options)
						if err != nil {
							t.Fatal(err)
						}
						body, h := r.Boundary(), r.History()
						topo := body.Topology()
						if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.InnerWires != 1 ||
							([3]int{topo.Faces, topo.Edges, topo.Vertices}) != ([3]int{12, 29, 20}) {
							t.Fatal("changed original five-region envelope topology", topo)
						}
						if h.CrossingCircle != side || h.SeparatedEnclosing != side || h.SeparatedFace != 4 ||
							h.SeparatedOwner == h.RoofOwners[side] || body.faces[4].Surface != src[1+((order+h.SeparatedOwner)%2)] {
							t.Fatal("changed separated original support or circle role", h)
						}
						for _, edge := range h.SeparatedEdges {
							if !opposedGraphContourUses(body.faces[side].Holes[0], body.faces[4].Outer, edge) {
								t.Fatal("separated circle lost one original coedge", edge)
							}
						}
						for i, node := range h.Junctions {
							point := geom.Vec3{X: float64(2*i+1) / 4, Y: 49.0 / 128, Z: 2}
							if axis == 1 {
								point.X, point.Y = point.Y, point.X
							}
							if body.vertices[node.Vertex] != point || slices.Contains(h.SeparatedVertices[:], node.Vertex) {
								t.Fatal("crossing node lost exact identity or migrated to separated circle", node)
							}
						}
						mass, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
						if err != nil || math.Abs(mass.Volume-v[index]) > 1e-10 {
							t.Fatal("envelope differs from independent original signed volume", mass, v[index], err)
						}
						t.Logf("signed volume %.17g; exact integral %.17g", mass.Volume, v[index])
						checkMixedCircleEnvelopeReplay(t, r, axis == 0 && side == 0 && order == 0 && index == 0)
					})
				}
			}
		}
	}
}

func checkMixedCircleEnvelopeReplay(t *testing.T, r *ParabolaMixedCirclesEnvelope, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaMixedCirclesEnvelope, error) {
		return ReadParabolaMixedCirclesEnvelopeJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	out, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || out.History() != r.History() {
		t.Fatal("reloaded result changed original incidence", err)
	}
	for i, source := range r.sources {
		if !geom.SameSplineSurface(source, out.sources[i]) {
			t.Fatal("reloaded result changed original owner support", i)
		}
	}
	if err := out.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("reloaded result changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if out, err := read(raw.Bytes(), JSONLimits{MaxEntities: 1}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short native budget published a partial arrangement", out, err)
	}
	var recipe parabolaMixedCirclesEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Separated.Edges[0] = recipe.Ledger.CircleEdges[0]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(forged, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a crossing edge as a separated circle owner", out, err)
	}
}
