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

// The first roof integrates to 6345/512 on the entire stock. The
// independently integrated B−A gap on the pointed region is 243/22528.
const cuspidalFuseVolume = 6345.0/512 + 243.0/22528

func TestNativeCuspidalCubicFuse(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	o := CuspidalCubicFuseOptions{}
	wantArea := cuspidalCubicEnvelopeSourceArea(true)
	for order := range 2 {
		input := [3]*geom.BSplineSurface{src[0], src[1+order], src[2-order]}
		r, err := FuseCuspidalCubicSplineGraphs(ctx, input[0], input[1], input[2], order, 1e-9, o)
		if err != nil {
			t.Fatal(order, err)
		}
		checkNativeCuspidalCubicShell(t, r.solid, r.history, input, order)
		m, err := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(m.Volume-cuspidalFuseVolume) > 1e-9 || math.Abs(m.Area-wantArea) > 1e-8 {
			t.Fatal("native Fuse differs from its independent source measures", order, m, wantArea, cuspidalFuseVolume, err)
		}
		t.Logf("order %d volume %.15g area %.15g", order, m.Volume, m.Area)
		var raw, echo bytes.Buffer
		if err := r.WriteJSON(ctx, &raw); err != nil {
			t.Fatal(err)
		}
		loaded, err := ReadCuspidalCubicFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
		if err != nil || loaded.History() != r.History() {
			t.Fatal("native Fuse cusp incidence changes across replay", err)
		}
		for i, source := range input {
			if !geom.SameSplineSurface(source, loaded.sources[i]) {
				t.Fatal("native replay changes an original support", i)
			}
		}
		checkNativeCuspidalCubicShell(t, loaded.solid, loaded.history, loaded.sources, order)
		if err := loaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
			t.Fatal("exact Fuse cusp recipe changed on replay", err)
		}
		if out, err := ReadCuspidalCubicCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); err == nil || out != nil {
			t.Fatal("Common reader admitted a Fuse receipt", err)
		}
		for _, edit := range []func(*cuspidalCubicCommonLedger){
			func(l *cuspidalCubicCommonLedger) { l.Germs[0].UVThird[1][0] = "17" },
			func(l *cuspidalCubicCommonLedger) { l.RoofOwners[0] ^= 1 },
		} {
			var receipt cuspidalCubicCommonRecipe
			if err := json.Unmarshal(raw.Bytes(), &receipt); err != nil {
				t.Fatal(err)
			}
			edit(receipt.Ledger)
			var edited bytes.Buffer
			if err := writePolynomialGraphRecipe(ctx, &edited, receipt); err != nil {
				t.Fatal(err)
			}
			if out, err := ReadCuspidalCubicFuseJSON(ctx, &edited, JSONLimits{}, o); !errors.Is(err, geom.ErrConvergence) || out != nil {
				t.Fatal("changed Fuse cusp incidence or owner passed replay", err)
			}
		}
	}
}

func TestNativeCuspidalCubicFuseRefusals(t *testing.T) {
	ctx := context.Background()
	o := CuspidalCubicFuseOptions{}
	unrepresented := nodalCubicStockFixture(t, [4]float64{-125.0 / 4096, 100.0 / 4096, -80.0 / 4096, 64.0 / 4096}, 729.0/16384)
	if out, err := FuseCuspidalCubicSplineGraphs(ctx, unrepresented[0], unrepresented[1], unrepresented[2], 0, 1e-9, o); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("unrepresentable original cusp was rounded into a Fuse shell", err)
	}
	src := cuspidalCubicCommonFixture(t)
	o.Proof.MaxOperations = 1
	if out, err := FuseCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o); err == nil || out != nil {
		t.Fatal("limited proof published a partial Fuse shell", err)
	}
}

func TestNativeCuspidalCubicFuseDisplay(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	for order := range 2 {
		r, err := FuseCuspidalCubicSplineGraphs(ctx, src[0], src[1+order], src[2-order], order, 1e-9, CuspidalCubicFuseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		s := r.Boundary()
		m, err := s.Tessellate(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := collision.NewMeshPointLocator(ctx, m, nil, collision.LocationOptions{}); err != nil {
			t.Fatal("sampled cusp display is not a closed embedded boundary", order, err)
		}
		t.Logf("order %d display %d triangles volume %.15g", order, len(m.Triangles), m.SignedVolume())
		if len(m.Vertices) < len(s.vertices) || !slices.Equal(m.Vertices[:len(s.vertices)], s.vertices) ||
			math.Abs(m.SignedVolume()-cuspidalFuseVolume) > .0002*cuspidalFuseVolume {
			t.Fatal("sampled display changes native vertices or loses material", order, m.SignedVolume())
		}
	}
}
