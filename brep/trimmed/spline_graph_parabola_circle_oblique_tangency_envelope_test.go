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
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaCircleObliqueTangencyEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	volumes := [2][2]float64{
		{2000370691.0/1006632960 - 36148125.0*math.Pi/35184372088832, 2034000893.0/1006632960 + 36148125.0*math.Pi/35184372088832},
		{13936357861.0/7046430720 - 35851875.0*math.Pi/35184372088832, 14175219227.0/7046430720 + 35851875.0*math.Pi/35184372088832},
	}
	for axis := range 2 {
		for side, v := range volumes {
			src := parabolaCircleObliqueStock(t, axis, originalObliqueCircleFixture(side), false)
			for order := range 2 {
				for index, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", axis, side, order, op), func(t *testing.T) {
						result, err := CombineParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, op, 1e-9, opts)
						if err != nil {
							t.Fatal(err)
						}
						body, h := result.Boundary(), result.History()
						topo := body.Topology()
						if ([3]int{topo.Faces, topo.Edges, topo.Vertices}) != ([3]int{12, 28, 18}) ||
							topo.EulerCharacteristic != 2 || topo.InnerWires != 0 || len(topo.Shells) != 1 {
							t.Fatal("wrong embedded split-roof envelope", topo)
						}
						owners := [4]int{1, 0, side, 1 - side}
						for i, owner := range owners {
							if index == 1 {
								owner = 1 - owner
							}
							if h.RoofOwners[i] != (owner+order)%2 || body.faces[h.RoofFaces[i]].Surface != src[1+owner] {
								t.Fatal("wrong unmodified roof in selected region", i, h)
							}
						}
						apex := geom.Vec3{X: .5, Y: .5, Z: 2}
						if axis == 1 {
							apex.X, apex.Y = apex.Y, apex.X
						}
						if body.vertices[h.Vertex] != apex || !opposedGraphContourUses(body.faces[side].Outer, body.faces[3].Outer, h.Bridge) {
							t.Fatal("missing exact tangent vertex or interior same-roof seam", h)
						}
						bridge := body.edges[h.Bridge]
						first, last := body.vertices[bridge.Start], body.vertices[bridge.End]
						if !obliqueBridgeEnds(first, last, axis, side) {
							t.Fatal("bridge did not retain the exact oblique circle extremum and stock wall station", first, last)
						}
						for branch, rays := range h.Rays {
							expectedFace := 1 - side
							if branch == 1 {
								expectedFace = 2
							}
							for _, id := range rays {
								e := body.edges[id]
								if (e.Start == h.Vertex) == (e.End == h.Vertex) ||
									!slices.Contains(nodalCubicIncidentFaces(body, id), expectedFace) {
									t.Fatal("lost an original directed tangent arc or its owner", branch, id)
								}
							}
						}
						mass, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
						if err != nil || math.Abs(mass.Volume-v[index]) > 1e-10 {
							t.Fatal("envelope does not match independent signed integral", mass, v[index], err)
						}
						t.Logf("signed volume %.17g; exact integral %.17g", mass.Volume, v[index])
						if axis == order && side == index {
							checkParabolaCircleObliqueEnvelopeReplay(t, ctx, result, axis == 0 && side == 0)
						}
					})
				}
			}
		}
	}
}

func checkParabolaCircleObliqueEnvelopeReplay(t *testing.T, ctx context.Context, result *ParabolaCircleObliqueTangencyEnvelope, negative bool) {
	t.Helper()
	opts := ParabolaCircleGraphOptions{}
	var raw, again bytes.Buffer
	if err := result.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadParabolaCircleObliqueTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
	if err != nil || reloaded.History() != result.History() {
		t.Fatal("native replay changed original cusp-ended roof owners", err)
	}
	for i, src := range result.envelope.sources {
		if !geom.SameSplineSurface(reloaded.envelope.sources[i], src) {
			t.Fatal("native replay changed original support", i)
		}
	}
	if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("native replay changed canonical bytes", err)
	}
	if !negative {
		return
	}
	if out, err := ReadParabolaCircleObliqueTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short recipe budget published an incomplete envelope", err)
	}
	var recipe parabolaCircleApexEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Ledger.Bridge = &recipe.Ledger.Circle[0]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := ReadParabolaCircleObliqueTangencyEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("a forged original curve substituted for the bridge on replay", err)
	}
}

func obliqueBridgeEnds(a, b geom.Vec3, axis, side int) bool {
	xFar := .5 - float64(2*side-1)*3./32
	yFar := .5 + float64(2*side-1)/8
	xWall := .5 - float64(2*side-1)*3./8
	yWall := float64(side)
	if axis == 1 {
		a.X, a.Y = a.Y, a.X
		b.X, b.Y = b.Y, b.X
	}
	return (a.X == xFar && a.Y == yFar && b.X == xWall && b.Y == yWall) ||
		(b.X == xFar && b.Y == yFar && a.X == xWall && a.Y == yWall)
}

func TestParabolaCircleObliqueBridgeRefusal(t *testing.T) {
	ctx := context.Background()
	o := ParabolaCircleGraphOptions{}
	f := originalObliqueCircleFixture(1)
	f.station, f.ordinate = big.NewRat(15, 32), big.NewRat(3, 8)
	src := parabolaCircleObliqueStock(t, 0, f, false)
	patches := parabolaTwoCirclesPatches(t, src[1], src[2])
	if p, err := geom.DiscoverSplineGraphParabolaCircleObliqueTangency(ctx, patches[0], patches[1], 0, o.Proof); err != nil || p == nil {
		t.Fatal("test must have a true original interior tangent contact", err)
	}
	// The exact normal reaches x=0,y=1, the stock corner.
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
		result, err := CombineParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[1], src[2], 0, op, 1e-9, o)
		if result != nil || !errors.Is(err, geom.ErrDomain) {
			t.Fatal("unsupported corner bridge published a partial envelope", result, err)
		}
	}
}

func TestParabolaCircleObliqueReflectedNative(t *testing.T) {
	ctx := context.Background()
	o := ParabolaCircleGraphOptions{}
	wanted := [2]float64{2000370691.0/1006632960 - 36148125.0*math.Pi/35184372088832,
		13936357861.0/7046430720 - 35851875.0*math.Pi/35184372088832}
	for side, value := range wanted {
		f := originalObliqueCircleFixture(side)
		f.slope = big.NewRat(-3, 4)
		src := parabolaCircleObliqueStock(t, side, f, false)
		r, err := CombineParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[1], src[2], side, parametric.Intersection, 1e-9, o)
		if err != nil {
			t.Fatal(err)
		}
		body := r.Boundary()
		// Reflection preserves the exact material by its unit absolute
		// Jacobian and reverses the bridge's projected free coordinate.
		mass, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(mass.Volume-value) > 1e-10 {
			t.Fatal("reflected original algebraic material changed", mass, value, err)
		}
		edge := body.edges[r.History().Bridge]
		a, b := body.vertices[edge.Start], body.vertices[edge.End]
		if side == 0 {
			a.X, b.X = 1-a.X, 1-b.X
		} else {
			a.Y, b.Y = 1-a.Y, 1-b.Y
		}
		if !obliqueBridgeEnds(a, b, side, side) {
			t.Fatal("reflected bridge changed exact wall provenance", a, b)
		}
	}
}
