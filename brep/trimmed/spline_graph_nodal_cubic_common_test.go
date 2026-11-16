package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func nodalCubicCommonFixture(t *testing.T) [3]*geom.BSplineSurface {
	t.Helper()
	// The original height difference is
	// (3/16)*((v-1/2)^2 - (u-13/16)^2*(u-3/4)).
	return nodalCubicStockFixture(t, [4]float64{-1521.0 / 16384, 403.0 / 16384, -105.0 / 16384, 27.0 / 16384}, 3.0/64)
}

func nodalCubicStockFixture(t *testing.T, uGap [4]float64, vGap float64) [3]*geom.BSplineSurface {
	t.Helper()
	src := [3]*geom.BSplineSurface{}
	roofHeight := func(u, v float64) float64 { return 2 + 3*u/16 + 3*u*v/32 }
	var flat, roofA, roofB []geom.Vec3
	for i := range 2 {
		for j := range 2 {
			u, v := float64(i), float64(j)
			flat = append(flat, geom.Vec3{X: 3 * u, Y: 2 * v, Z: (u + v) / 16})
			roofB = append(roofB, geom.Vec3{X: 3 * (1 - v), Y: 2 * u, Z: roofHeight(1-v, u)})
		}
	}
	for i := range 4 {
		for j := range 3 {
			k, v := float64(i), float64(j)/2
			// The factor 3 in the base law cancels the cubic pole
			// denominator. All model poles here are exact binary64 values.
			z := 2 + k/16 + k*v/32 + ([3]float64{vGap, -vGap, vGap})[j] - uGap[i]
			roofA = append(roofA, geom.Vec3{X: k, Y: float64(j), Z: z})
		}
	}
	for i, data := range []struct {
		poles            []geom.Vec3
		knotsU, knotsV   []float64
		degreeU, degreeV int
	}{
		{flat, []float64{0, 0, 3, 3}, []float64{0, 0, 2, 2}, 1, 1},
		{roofA, []float64{0, 0, 0, 0, 1, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, 3, 2},
		{roofB, []float64{2, 2, 4, 4}, []float64{-3, -3, 1, 1}, 1, 1},
	} {
		var err error
		src[i], err = geom.NewBSplineSurface(data.degreeU, data.degreeV, data.degreeU+1, data.degreeV+1,
			data.poles, data.knotsU, data.knotsV, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return src
}

func TestNativeNodalCubicCommon(t *testing.T) {
	ctx := context.Background()
	src := nodalCubicCommonFixture(t)
	options := NodalCubicCommonOptions{}
	for order := range 2 {
		input := [3]*geom.BSplineSurface{src[0], src[1+order], src[2-order]}
		common, err := CommonNodalCubicSplineGraphs(ctx, input[0], input[1], input[2], order, 1e-9, options)
		if err != nil {
			t.Fatal(order, err)
		}
		checkNativeNodalCubicCommon(t, common, input, order)
		var recipe bytes.Buffer
		if err := common.WriteJSON(ctx, &recipe); err != nil {
			t.Fatal(err)
		}
		reloaded, err := ReadNodalCubicCommonJSON(ctx, bytes.NewReader(recipe.Bytes()), JSONLimits{}, options)
		if err != nil || reloaded.History() != common.History() {
			t.Fatal("original seam and ray identities change across reload", err)
		}
		for i, source := range input {
			if !geom.SameSplineSurface(source, reloaded.sources[i]) {
				t.Fatal("reload changes an original stock", i)
			}
		}
		var replay bytes.Buffer
		if err := reloaded.WriteJSON(ctx, &replay); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
			t.Fatal("reload changes the canonical nodal recipe", err)
		}
	}
}

func checkNativeNodalCubicCommon(t *testing.T, common *NodalCubicCommon, sources [3]*geom.BSplineSurface, order int) {
	t.Helper()
	ctx := context.Background()
	s, h := common.Boundary(), common.History()
	// In canonical XY the loop and arm contributions to the removed
	// height integrate to 1/151388160 and 4619/151388160 before multiplying
	// by the projected determinant six and the height factor 1/4.
	wantVolume, wantArea := 807933.0/65536, nodalCubicCommonSourceArea()
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil || math.Abs(mass.Volume-wantVolume) > 1e-9 || math.Abs(mass.Area-wantArea) > 1e-8 {
		t.Fatal("independent positive Common measures", mass, wantVolume, wantArea, err)
	}
	t.Logf("order %d volume %.15g area %.15g", order, mass.Volume, mass.Area)
	top := s.Topology()
	if len(s.faces) != 12 || len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 ||
		h.RoofOwners != ([4]int{1 ^ order, 1 ^ order, order, order}) || h.NodeSectors != ([4]int{0, 2, 1, 3}) ||
		s.faces[h.Base].Surface != sources[0] {
		t.Fatal("native roof owners or nodal sector incidence", h, top)
	}
	for i, owner := range h.RoofOwners {
		if s.faces[h.RoofFaces[i]].Surface != sources[1+owner] {
			t.Fatal("roof replacement changed an original support", i)
		}
	}
	section := map[int]bool{}
	for i, chain := range h.Chains {
		if (chain[1] >= 0) != (i == 1 || i == 2) {
			t.Fatal("both closed-loop halves require two original restrictions", i, chain)
		}
		for _, id := range chain {
			if id < 0 {
				continue
			}
			if section[id] || id == h.Seam {
				t.Fatal("an original branch shares another branch's native edge", id)
			}
			section[id] = true
		}
	}
	if len(section) != 6 || s.vertices[h.Vertices[0]] != (geom.Vec3{X: 39.0 / 16, Y: 1, Z: 2243.0 / 1024}) {
		t.Fatal("original node or branch mask changed", section, s.vertices[h.Vertices[0]])
	}
	for i, id := range h.NodeRays {
		before, after := h.NodeSectors[(i+3)%4], h.NodeSectors[i]
		if !section[id] || slices.Contains(h.NodeRays[:i], id) ||
			!slices.Equal(nodalCubicIncidentFaces(s, id), []int{min(before, after), max(before, after)}) {
			t.Fatal("wrong original node ray or sector adjacency", i, id)
		}
	}
	seam := s.edges[h.Seam]
	a, b := s.vertices[seam.Start], s.vertices[seam.End]
	if a.X > b.X {
		a, b = b, a
	}
	if a != (geom.Vec3{Y: 1, Z: 2}) || b != (geom.Vec3{X: 9.0 / 4, Y: 1, Z: 557.0 / 256}) ||
		seam.Curve.(surfaceTrimCurve).surface != sources[2-order] ||
		!slices.Equal(nodalCubicIncidentFaces(s, h.Seam), []int{0, 1}) {
		t.Fatal("seam leaves the exact exterior owner fiber", a, b)
	}
}

func nodalCubicCommonSourceArea() float64 {
	// Gauss-Legendre on the original graph laws. In the two interior
	// regions use u=3/4+t² so both original branch ordinates are polynomial.
	points := [5]float64{-.906179845938664, -.538469310105683, 0, .538469310105683, .906179845938664}
	weights := [5]float64{.236926885056189, .478628670499366, .568888888888889, .478628670499366, .236926885056189}
	integrate := func(a, b float64, f func(float64) float64) float64 {
		sum := 0.0
		for panel := range 8 {
			width := (b - a) / 8
			mid := a + (float64(panel)+.5)*width
			for i, p := range points {
				sum += width / 2 * weights[i] * f(mid+width/2*p)
			}
		}
		return sum
	}
	roof := func(u, v float64, originalA bool) float64 {
		du, dv := 3.0/16+3*v/32, 3*u/32
		if originalA {
			x := u - 13.0/16
			du -= (3.0 / 16) * x * (3*x + 1.0/8)
			dv += (3.0 / 8) * (v - .5)
		}
		return math.Sqrt(36 + 4*du*du + 9*dv*dv)
	}
	area := integrate(0, 1, func(u float64) float64 {
		return integrate(0, 1, func(v float64) float64 { return roof(u, v, false) })
	})
	correction := func(t float64) float64 {
		u := .75 + t*t
		width := t * math.Abs(u-13.0/16)
		return 2 * t * integrate(.5-width, .5+width, func(v float64) float64 {
			return roof(u, v, true) - roof(u, v, false)
		})
	}
	area += integrate(0, .25, correction) + integrate(.25, .5, correction)
	baseArea := 6 * math.Sqrt(1+1.0/(48*48)+1.0/(32*32))
	const wallArea = 1315.0/64 - 27.0/65536
	return area + baseArea + wallArea
}

func TestNativeNodalCubicCommonRefusesChangedRecipe(t *testing.T) {
	ctx := context.Background()
	src := nodalCubicCommonFixture(t)
	options := NodalCubicCommonOptions{}
	common, err := CommonNodalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, options)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := common.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*nodalCubicCommonLedger){
		func(h *nodalCubicCommonLedger) { h.Seam = h.NodeRays[0] },
		func(h *nodalCubicCommonLedger) { h.NodeRays[0], h.NodeRays[1] = h.NodeRays[1], h.NodeRays[0] },
		func(h *nodalCubicCommonLedger) { h.RoofOwners[0] ^= 1 },
		func(h *nodalCubicCommonLedger) { h.NodeRays = append(h.NodeRays, h.NodeRays[0]) },
	} {
		var recipe nodalCubicCommonRecipe
		if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
			t.Fatal(err)
		}
		change(recipe.Ledger)
		var changed bytes.Buffer
		if err := writePolynomialGraphRecipe(ctx, &changed, recipe); err != nil {
			t.Fatal(err)
		}
		if out, err := ReadNodalCubicCommonJSON(ctx, &changed, JSONLimits{}, options); !errors.Is(err, geom.ErrConvergence) || out != nil {
			t.Fatal("changed owner, ray or seam accepted", err)
		}
	}
	for _, limits := range []JSONLimits{{MaxBytes: int64(raw.Len() - 1)}, {MaxControlPoints: 4}, {MaxEntities: 8}} {
		if out, err := ReadNodalCubicCommonJSON(ctx, bytes.NewReader(raw.Bytes()), limits, options); err == nil || out != nil {
			t.Fatal("limited native replay published a result", limits, err)
		}
	}
	options.Proof.MaxOperations = 1
	if out, err := CommonNodalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, options); err == nil || out != nil {
		t.Fatal("truncated contact proof published native material", err)
	}
}

func TestNativeNodalCubicCommonRefusesUnrepresentedGeometry(t *testing.T) {
	ctx := context.Background()
	// The law (81/256)*((v-1/2)^2-(u-2/3)^2*(u-5/9))
	// has exact rational events on original binary64 supports. The loop
	// turn has model X=5/3 and cannot become a lossless native vertex.
	uGap := [4]float64{-20.0 / 256, 12.0 / 256, -7.0 / 256, 4.0 / 256}
	src := nodalCubicStockFixture(t, uGap, 81.0/1024)
	var patches [2]*geom.SurfacePatch
	for i, source := range src[1:] {
		var err error
		patches[i], err = geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := geom.DiscoverProjectedGraphNodalCubic(ctx, patches[0], patches[1], [2]int{0, 1}, 0, geom.GraphPointContactOptions{})
	if err != nil || p.Vertices[1].Point[0].RatString() != "5/3" {
		t.Fatal("nonbinary control did not prove the original contact", p, err)
	}
	if out, err := CommonNodalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, NodalCubicCommonOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("nonbinary vertex silently rounded into the native shell", err)
	}
}

func TestNativeNodalCubicCommonDisplay(t *testing.T) {
	ctx := context.Background()
	src := nodalCubicCommonFixture(t)
	result, err := CommonNodalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, NodalCubicCommonOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := result.Boundary()
	m, err := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Vertices) < len(s.vertices) || !slices.Equal(m.Vertices[:len(s.vertices)], s.vertices) || math.Abs(m.SignedVolume()-807933.0/65536) > 1e-4 {
		t.Fatal("default display loses original vertex or material coverage", len(m.Vertices), m.SignedVolume())
	}
	if _, err := collision.NewMeshPointLocator(ctx, m, nil, collision.LocationOptions{}); err != nil {
		t.Fatal("default display does not preserve an embedded closed boundary", err)
	}
	t.Logf("default display uses %d triangles", len(m.Triangles))
}
