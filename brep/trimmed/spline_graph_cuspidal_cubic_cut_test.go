package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

// For the original roofs A−B=(3/16)*((v−1/2)^2−(u−1/4)^3/3).
// Over the 3×2 footprint, its integral is 33/512. Inside the cusp use
// u=1/4+3*t² with half-width 3*t³ for 0<=t<=1/2. The integral of B−A
// across that width is (3/16)*(4/3)*(3*t³)^3 before the 6*t and 6
// Jacobians. Integration gives the pointed region's exact cut volume.
const cuspidalPointedCutVolume = 243.0 / 22528
const cuspidalExteriorCutVolume = 33.0/512 + cuspidalPointedCutVolume

func TestNativeCuspidalCubicCut(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	o := CuspidalCubicCutOptions{}
	for order, expected := range [2]float64{cuspidalExteriorCutVolume, cuspidalPointedCutVolume} {
		input := [3]*geom.BSplineSurface{src[0], src[1+order], src[2-order]}
		r, err := CutCuspidalCubicSplineGraphs(ctx, input[0], input[1], input[2], order, 1e-9, o)
		if err != nil {
			t.Fatal(order, err)
		}
		checkNativeCuspidalCubicCut(t, r, input, order)
		m, err := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(m.Volume-expected) > 1e-10 {
			t.Fatal("native Cut differs from the independent signed source integral", order, m.Volume, expected, err)
		}
		t.Logf("order %d volume %.15g", order, m.Volume)
		var raw, echo bytes.Buffer
		if err := r.WriteJSON(ctx, &raw); err != nil {
			t.Fatal(err)
		}
		loaded, err := ReadCuspidalCubicCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
		if err != nil || !reflect.DeepEqual(loaded.History(), r.History()) {
			t.Fatal("native Cut cusp incidence changes on replay", order, err)
		}
		for i, source := range input {
			if !geom.SameSplineSurface(source, loaded.sources[i]) {
				t.Fatal("native Cut replay changes an original support", order, i)
			}
		}
		checkNativeCuspidalCubicCut(t, loaded, loaded.sources, order)
		if err := loaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
			t.Fatal("exact Cut cusp recipe changed on replay", order, err)
		}
		h := loaded.History()
		h.WallFaces[0] = -1
		if loaded.History().WallFaces[0] < 0 {
			t.Fatal("returned history aliases the stored wall ledger", order)
		}
	}
}

func checkNativeCuspidalCubicCut(t *testing.T, r *CuspidalCubicCut, input [3]*geom.BSplineSurface, region int) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	topology := s.Topology()
	wantWalls := [2]int{5, 2}[region]
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 ||
		h.Region != region || len(h.WallFaces) != wantWalls || len(s.faces) != wantWalls+2 {
		t.Fatal("signed native Cut region is not one closed ball", h, topology)
	}
	for i, face := range h.RoofFaces {
		if s.faces[face].Surface != input[1+i] {
			t.Fatal("native Cut substituted an original roof", i)
		}
	}
	want := [3]geom.Vec3{
		{X: .75, Y: 1, Z: 527.0 / 256}, {X: 3, Y: .25, Z: 563.0 / 256}, {X: 3, Y: 1.75, Z: 581.0 / 256},
	}
	for i, vertex := range h.Vertices {
		if s.vertices[vertex] != want[i] {
			t.Fatal("native Cut moved the cusp or an exit", i, s.vertices[vertex], want[i])
		}
	}
	for i, id := range h.Germs {
		edge := s.edges[id]
		station, exit := edge.Range.Min, edge.End
		if edge.End == h.Vertices[0] {
			station, exit = edge.Range.Max, edge.Start
		}
		if exit != h.Vertices[i+1] || !slices.Equal(nodalCubicIncidentFaces(s, id), h.RoofFaces[:]) {
			t.Fatal("germ leaves its two original faces or endpoint", i, h)
		}
		if v, err := edge.Curve.Derivative(station); err != nil || v != (geom.Vec3{}) {
			t.Fatal("native Cut changed the zero model derivative", i, v, err)
		}
	}
	for _, id := range h.WallFaces {
		var owners [2]bool
		for _, use := range s.faces[id].Outer {
			if use.Curve == nil {
				continue
			}
			for i := range owners {
				trim, ok := graphWallSourceTrim(use.Curve, i == 0, true)
				if ok && trim.surface == input[1+i] {
					owners[i] = true
				}
			}
		}
		if !owners[0] || !owners[1] {
			t.Fatal("Cut wall lost an original roof coedge", h, id)
		}
	}
}

func TestNativeCuspidalCubicCutRefusals(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	o := CuspidalCubicCutOptions{}
	r, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*cuspidalCubicCutLedger){
		func(l *cuspidalCubicCutLedger) { *l.Region ^= 1 },
		func(l *cuspidalCubicCutLedger) { l.WallFaces[0] = l.WallFaces[1] },
		func(l *cuspidalCubicCutLedger) { l.RoofFaces[0], l.RoofFaces[1] = l.RoofFaces[1], l.RoofFaces[0] },
		func(l *cuspidalCubicCutLedger) { l.Germs[0].UVThird[1][0] = "17" },
		func(l *cuspidalCubicCutLedger) { l.Germs[0].Edge = l.Germs[1].Edge },
	} {
		var recipe cuspidalCubicCutRecipe
		if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
			t.Fatal(err)
		}
		edit(recipe.Ledger)
		var changed bytes.Buffer
		if err := writePolynomialGraphRecipe(ctx, &changed, recipe); err != nil {
			t.Fatal(err)
		}
		if out, err := ReadCuspidalCubicCutJSON(ctx, &changed, JSONLimits{}, o); !errors.Is(err, geom.ErrConvergence) || out != nil {
			t.Fatal("native replay accepted a changed region, owner, or germ", err)
		}
	}
	var changedSource cuspidalCubicCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &changedSource); err != nil {
		t.Fatal(err)
	}
	changedSource.Sources[2] = changedSource.Sources[1]
	var changed bytes.Buffer
	if err := writePolynomialGraphRecipe(ctx, &changed, changedSource); err != nil {
		t.Fatal(err)
	}
	if out, err := ReadCuspidalCubicCutJSON(ctx, &changed, JSONLimits{}, o); err == nil || out != nil {
		t.Fatal("native replay accepted coincident original roofs", err)
	}
	if out, err := ReadCuspidalCubicCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, o); err == nil || out != nil {
		t.Fatal("bounded replay published a shell above its entity limit", err)
	}
	if out, err := ReadCuspidalCubicFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); err == nil || out != nil {
		t.Fatal("Fuse reader admitted a Cut receipt", err)
	}
	nonbinary := nodalCubicStockFixture(t, [4]float64{-125.0 / 4096, 100.0 / 4096, -80.0 / 4096, 64.0 / 4096}, 729.0/16384)
	if out, err := CutCuspidalCubicSplineGraphs(ctx, nonbinary[0], nonbinary[1], nonbinary[2], 0, 1e-9, o); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("native Cut rounded a nonrepresentable cusp", err)
	}
	o.Proof.MaxOperations = 1
	if out, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o); err == nil || out != nil {
		t.Fatal("limited proof published a partial Cut", err)
	}
}

func TestNativeCuspidalCubicCutGenericDisplay(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	o := CuspidalCubicCutOptions{}
	display := parametric.TessellationOptions{Deflection: .005, Angle: .1, MaxSubdivisions: 32, MaxTriangles: 80_000}
	pointed, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	s := pointed.Boundary()
	mesh, err := s.Tessellate(ctx, display)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collision.NewMeshPointLocator(ctx, mesh, nil, collision.LocationOptions{}); err != nil {
		t.Fatal("pointed display is not closed and embedded", err)
	}
	relativeError := math.Abs(mesh.SignedVolume()/cuspidalPointedCutVolume - 1)
	if len(mesh.Vertices) < len(s.vertices) || !slices.Equal(mesh.Vertices[:len(s.vertices)], s.vertices) || relativeError > .01 {
		t.Fatal("pointed display changed native vertices or differs from the source volume by over 1%", mesh.SignedVolume(), relativeError)
	}
	t.Logf("pointed preview %d triangles volume %.15g, %.3f%% relative error",
		len(mesh.Triangles), mesh.SignedVolume(), 100*relativeError)
	exterior, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	display.Deflection, display.Angle = .05, .15
	failed, err := exterior.Boundary().Tessellate(ctx, display)
	if err == nil || len(failed.Triangles) != 0 {
		t.Fatal("exterior display published a mesh without qualified closure", err)
	}
	t.Logf("exterior preview refused: %v", err)
}
