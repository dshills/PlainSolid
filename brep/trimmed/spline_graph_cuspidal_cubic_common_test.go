package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

const cuspidalCommonVolume = 277485.0 / 22528

func cuspidalCubicCommonFixture(t *testing.T) [3]*geom.BSplineSurface {
	t.Helper()
	// Original A−B is (3/16)*((v-1/2)^2 - (u-1/4)^3/3).
	// The free cubic's Bernstein ordinates are exact on the 3×2 stock.
	return nodalCubicStockFixture(t, [4]float64{-1.0 / 1024, 3.0 / 1024, -9.0 / 1024, 27.0 / 1024}, 3.0/64)
}

func TestNativeCuspidalCubicCommon(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	o := CuspidalCubicCommonOptions{}
	wantArea := cuspidalCubicCommonSourceArea()
	for order := range 2 {
		input := [3]*geom.BSplineSurface{src[0], src[1+order], src[2-order]}
		r, err := CommonCuspidalCubicSplineGraphs(ctx, input[0], input[1], input[2], order, 1e-9, o)
		if err != nil {
			t.Fatal(order, err)
		}
		checkNativeCuspidalCubicShell(t, r, input, order)
		// Integrate the original bilinear stock over the full footprint.
		// Subtract the gap integral (4/3)*(3/16)*w³ vertically between
		// the two germs, where w=t³/k and du=2t/k dt on 0<=t<=1/2.
		m, err := r.solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(m.Volume-cuspidalCommonVolume) > 1e-9 || math.Abs(m.Area-wantArea) > 1e-8 {
			t.Fatal("native Common differs from its independent source measures", order, m, wantArea, cuspidalCommonVolume, err)
		}
		t.Logf("order %d volume %.15g area %.15g", order, m.Volume, m.Area)
		var raw, echo bytes.Buffer
		if err := r.WriteJSON(ctx, &raw); err != nil {
			t.Fatal(err)
		}
		loaded, err := ReadCuspidalCubicCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
		if err != nil || loaded.History() != r.History() {
			t.Fatal("native cusp incidence changes across replay", err)
		}
		for i, source := range input {
			if !geom.SameSplineSurface(source, loaded.sources[i]) {
				t.Fatal("native replay changes an original support", i)
			}
		}
		if err := loaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
			t.Fatal("exact cusp recipe changed on replay", err)
		}
	}
}

func checkNativeCuspidalCubicShell(t *testing.T, r *CuspidalCubicCommon, input [3]*geom.BSplineSurface, order int) {
	t.Helper()
	h, s := r.history, r.solid
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || len(s.faces) != 9 || topology.CollapsedEdges != 0 ||
		h.RoofOwners != ([2]int{order ^ 1, order}) || s.faces[h.Base].Surface != input[0] {
		t.Fatal("signed roof regions or closed stock topology", h, topology)
	}
	for i, face := range h.RoofFaces {
		if s.faces[face].Surface != input[1+h.RoofOwners[i]] {
			t.Fatal("retained roof support changed", i)
		}
	}
	want := [3]geom.Vec3{
		{X: .75, Y: 1, Z: 527.0 / 256}, {X: 3, Y: .25, Z: 563.0 / 256}, {X: 3, Y: 1.75, Z: 581.0 / 256},
	}
	for i, vertex := range h.Vertices {
		if s.vertices[vertex] != want[i] {
			t.Fatal("native cusp or exit is displaced", i, s.vertices[vertex], want[i])
		}
	}
	for _, id := range h.Germs {
		if !slices.Equal(nodalCubicIncidentFaces(s, id), h.RoofFaces[:]) {
			t.Fatal("the same two signed faces must close both cusp link arcs", id)
		}
		edge := s.edges[id]
		station := edge.Range.Min
		if edge.End == h.Vertices[0] {
			station = edge.Range.Max
		} else if edge.Start != h.Vertices[0] {
			t.Fatal("germ does not reach the singular vertex", id)
		}
		if v, err := edge.Curve.Derivative(station); err != nil || v != (geom.Vec3{}) {
			t.Fatal("native model curve substituted a regular first derivative", id, v, err)
		}
	}
}

func cuspidalCubicCommonSourceArea() float64 {
	points := [5]float64{-.906179845938664, -.538469310105683, 0, .538469310105683, .906179845938664}
	weights := [5]float64{.236926885056189, .478628670499366, .568888888888889, .478628670499366, .236926885056189}
	integrate := func(a, b float64, f func(float64) float64) float64 {
		sum := 0.0
		for panel := range 8 {
			width := (b - a) / 8
			center := a + (float64(panel)+.5)*width
			for i, p := range points {
				sum += width / 2 * weights[i] * f(center+width/2*p)
			}
		}
		return sum
	}
	roof := func(u, v float64, lowerInside bool) float64 {
		du, dv := 3.0/16+3*v/32, 3*u/32
		if lowerInside {
			offset := u - .25
			du -= (3.0 / 16) * offset * offset
			dv += (3.0 / 8) * (v - .5)
		}
		return math.Sqrt(36 + 4*du*du + 9*dv*dv)
	}
	area := integrate(0, 1, func(u float64) float64 {
		return integrate(0, 1, func(v float64) float64 { return roof(u, v, false) })
	})
	// u=.25+3*t² keeps the exact cusp thickness 6*t³ in the source area integral.
	area += integrate(0, .5, func(t float64) float64 {
		u, width := .25+3*t*t, 3*t*t*t
		return 6 * t * integrate(.5-width, .5+width, func(v float64) float64 {
			return roof(u, v, true) - roof(u, v, false)
		})
	})
	baseArea := 6 * math.Sqrt(1+1.0/(48*48)+1.0/(32*32))
	const wallArea = 1315.0/64 - 27.0/1024
	return area + baseArea + wallArea
}

func TestNativeCuspidalCubicCommonRefusals(t *testing.T) {
	ctx := context.Background()
	o := CuspidalCubicCommonOptions{}
	node := nodalCubicCommonFixture(t)
	if out, err := CommonCuspidalCubicSplineGraphs(ctx, node[0], node[1], node[2], 0, 1e-9, o); !errors.Is(err, geom.ErrConvergence) || out != nil {
		t.Fatal("a transverse node was admitted as a cusp", err)
	}
	// (729/4096)*((v-1/2)^2-(u-5/9)^3) has exact rational events on
	// the original binary64 stock. Model X at its cusp is 5/3.
	unrepresented := nodalCubicStockFixture(t, [4]float64{-125.0 / 4096, 100.0 / 4096, -80.0 / 4096, 64.0 / 4096}, 729.0/16384)
	if out, err := CommonCuspidalCubicSplineGraphs(ctx, unrepresented[0], unrepresented[1], unrepresented[2], 0, 1e-9, o); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("unrepresentable original cusp was rounded into a shell", err)
	}
	src := cuspidalCubicCommonFixture(t)
	r, err := CommonCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	changed := r.solid
	changed.faces = r.solid.Faces()
	face := &changed.faces[r.history.RoofFaces[0]]
	for i, use := range face.Outer {
		if use.Edge != r.history.Germs[0] {
			continue
		}
		c := use.Curve.(ExactBezierUV).HomogeneousControls()
		j := 1
		edge := changed.edges[use.Edge]
		end := edge.End
		if use.Reversed {
			end = edge.Start
		}
		// Alter the control adjacent to the cusp, leaving both
		// endpoints and the shared model edge ID intact.
		if end == r.history.Vertices[0] {
			j = 2
		}
		c[j][1].Add(c[j][1], big.NewRat(1, 1024))
		face.Outer[i].Curve, err = NewExactBezierUV(c)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := bindCuspidalCubicGermCoedges(changed, r.history, r.section, 0); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("native coedge proof accepted a falsified regular cusp derivative", err)
	}
	var raw bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*cuspidalCubicCommonLedger){
		func(l *cuspidalCubicCommonLedger) { l.Germs[0].UVThird[1][0] = "0" },
		func(l *cuspidalCubicCommonLedger) { l.Germs[1].Edge = l.Germs[0].Edge },
		func(l *cuspidalCubicCommonLedger) { l.RoofOwners[0] ^= 1 },
	} {
		var recipe cuspidalCubicCommonRecipe
		if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
			t.Fatal(err)
		}
		edit(recipe.Ledger)
		var change bytes.Buffer
		if err := writePolynomialGraphRecipe(ctx, &change, recipe); err != nil {
			t.Fatal(err)
		}
		if out, err := ReadCuspidalCubicCommonJSON(ctx, &change, JSONLimits{}, o); !errors.Is(err, geom.ErrConvergence) || out != nil {
			t.Fatal("changed original cusp or owner incidence passed replay", err)
		}
	}
	o.Proof.MaxOperations = 1
	if out, err := CommonCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o); err == nil || out != nil {
		t.Fatal("limited proof published a partial shell", err)
	}
}

func TestNativeCuspidalCubicCommonDisplay(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	for order := range 2 {
		r, err := CommonCuspidalCubicSplineGraphs(ctx, src[0], src[1+order], src[2-order], order, 1e-9, CuspidalCubicCommonOptions{})
		if err != nil {
			t.Fatal(err)
		}
		s := r.Boundary()
		m, err := s.Tessellate(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Vertices) < len(s.vertices) || !slices.Equal(m.Vertices[:len(s.vertices)], s.vertices) ||
			math.Abs(m.SignedVolume()-cuspidalCommonVolume) > .0002*cuspidalCommonVolume {
			t.Fatal("sampled display changes native vertices or loses material", order, m.SignedVolume())
		}
		if _, err := collision.NewMeshPointLocator(ctx, m, nil, collision.LocationOptions{}); err != nil {
			t.Fatal("sampled cusp display is not a closed embedded boundary", err)
		}
		t.Logf("order %d display %d triangles volume %.15g", order, len(m.Triangles), m.SignedVolume())
	}
}
