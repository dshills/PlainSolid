package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaTwoCrossingCirclesCut(t *testing.T) {
	ctx := context.Background()
	f := parabolaTwoCrossingCirclesStaggered()
	// Independent Green integrals of the original difference polynomial,
	// over the exterior and the two congruent circular/parabolic caps.
	exterior := [2]float64{
		3916874323195565375.0/4451429929287036174336 + 10041325.0*math.Pi/140737488355328,
		29806969429345079615.0/4451429929287036174336 - 10041325.0*math.Pi/422212465065984,
	}
	cap := [2]float64{
		338828817319615.0/8902859858574072348672 - 10041325.0*math.Pi/844424930131968,
		338828817319615.0/8902859858574072348672 + 10041325.0*math.Pi/281474976710656,
	}
	for axis := range 2 {
		stocks := f.stocks(t, axis, false)
		for order := range 2 {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				roof := [2]*geom.BSplineSurface{stocks[1+order], stocks[2-order]}
				r, err := CutParabolaTwoCrossingCirclesSplineGraphs(ctx, stocks[0], roof[0], roof[1], axis, 1e-9, ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, history := r.Components(), r.History()
				regions := [3]int{order, 3 - order, 5 - order}
				for i, part := range parts {
					h := history[i]
					topology := part.Topology()
					if h.Region != regions[i] || len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 ||
						topology.InnerWires != 0 || topology.CollapsedEdges != 0 {
						t.Fatal("changed positive projected component", i, h, topology)
					}
					if (i == 0 && (len(h.WallFaces) != 3 || len(h.DomainSides) != 3)) || (i > 0 && len(h.WallFaces) != 0) {
						t.Fatal("wrong original stock-side wall ownership", h)
					}
					for owner, id := range h.RoofFaces {
						if part.faces[id].Surface != roof[owner] || part.faces[id].Reversed != (owner == 1) {
							t.Fatal("changed original roof or material orientation", h)
						}
					}
					want := cap[order]
					if i == 0 {
						want = exterior[order]
					}
					mass, err := part.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7, MaxSubdivisions: 32})
					if err != nil || mass.Volume <= 0 || math.Abs(mass.Volume-want) > min(want*3e-6, 1e-10) {
						t.Fatal("component differs from the independent signed integral", i, mass, want, err)
					}
					t.Logf("region %d signed volume %.17g; exact integral %.17g", h.Region, mass.Volume, want)
				}
				for circle, contacts := range r.Contacts() {
					for j, contact := range contacts {
						x, _ := f.roots[circle][j].Float64()
						y, _ := f.ordinate(f.roots[circle][j]).Float64()
						point := geom.Vec3{X: x, Y: y, Z: 2}
						if axis == 1 {
							point.X, point.Y = point.Y, point.X
						}
						if contact.Components != [2]int{0, circle + 1} ||
							parts[0].vertices[contact.Vertices[0]] != point || parts[circle+1].vertices[contact.Vertices[1]] != point {
							t.Fatal("changed exact cap/exterior contact incidence", circle, j, contact)
						}
					}
				}
				checkTwoCrossingCirclesCutReplay(t, r, axis == 0 && order == 0)
			})
		}
	}
}

func checkTwoCrossingCirclesCutReplay(t *testing.T, r *ParabolaTwoCrossingCirclesCut, negative bool) {
	t.Helper()
	ctx := context.Background()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, limits JSONLimits) (*ParabolaTwoCrossingCirclesCut, error) {
		return ReadParabolaTwoCrossingCirclesCutJSON(ctx, bytes.NewReader(data), limits, ParabolaCircleGraphOptions{})
	}
	out, err := read(raw.Bytes(), JSONLimits{})
	if err != nil || !reflect.DeepEqual(out.History(), r.History()) || !reflect.DeepEqual(out.Contacts(), r.Contacts()) {
		t.Fatal("reloaded Cut changed source regions or contacts", err)
	}
	for i, source := range r.sources {
		if !geom.SameSplineSurface(source, out.sources[i]) {
			t.Fatal("reloaded Cut changed original stock", i)
		}
	}
	if err := out.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("reloaded Cut changed canonical bytes", err)
	}
	if !negative {
		return
	}
	var recipe parabolaCrossingCirclesCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Contacts[0][0].Components[1] = 2
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := read(forged, JSONLimits{}); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("replay accepted a swapped cap contact", out, err)
	}
}

func TestNativeParabolaTwoCrossingCirclesCutRefusal(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		f    parabolaTwoCrossingsSourceFixture
		opts ParabolaCircleGraphOptions
		want error
	}{
		{"nonbinary original junction", parabolaTwoCrossingCirclesCoincidentProjection(), ParabolaCircleGraphOptions{}, geom.ErrUnrepresentable},
		{"short exact budget", parabolaTwoCrossingCirclesStaggered(), ParabolaCircleGraphOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}, geom.ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.f.stocks(t, 0, false)
			r, err := CutParabolaTwoCrossingCirclesSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, tc.opts)
			if r != nil || !errors.Is(err, tc.want) {
				t.Fatal("published Cut with an unresolved source", r, err)
			}
		})
	}
}
