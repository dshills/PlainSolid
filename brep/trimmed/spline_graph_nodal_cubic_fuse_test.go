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

func TestNativeNodalCubicFuse(t *testing.T) {
	ctx := context.Background()
	src := nodalCubicCommonFixture(t)
	options := NodalCubicFuseOptions{}
	for order := range 2 {
		input := [3]*geom.BSplineSurface{src[0], src[1+order], src[2-order]}
		r, err := FuseNodalCubicSplineGraphs(ctx, input[0], input[1], input[2], order, 1e-9, options)
		if err != nil {
			t.Fatal(order, err)
		}
		checkNativeNodalCubicEnvelope(t, r.Boundary(), r.History(), input, order, true)
		var raw, replay bytes.Buffer
		if err := r.WriteJSON(ctx, &raw); err != nil {
			t.Fatal(err)
		}
		if out, err := ReadNodalCubicCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, options); !errors.Is(err, geom.ErrInvalid) || out != nil {
			t.Fatal("upper selection was read as Common", err)
		}
		loaded, err := ReadNodalCubicFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, options)
		if err != nil || loaded.History() != r.History() {
			t.Fatal("original seams and contacts changed after Fuse reload", err)
		}
		if err := loaded.WriteJSON(ctx, &replay); err != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
			t.Fatal("Fuse source recipe changed on reload", err)
		}
		for i, source := range input {
			if !geom.SameSplineSurface(source, loaded.sources[i]) {
				t.Fatal("Fuse reload changed an original stock", i)
			}
		}

		var changed nodalCubicCommonRecipe
		if err := json.Unmarshal(raw.Bytes(), &changed); err != nil {
			t.Fatal(err)
		}
		changed.Ledger.RoofOwners[0] ^= 1
		replay.Reset()
		if err := writePolynomialGraphRecipe(ctx, &replay, changed); err != nil {
			t.Fatal(err)
		}
		if out, err := ReadNodalCubicFuseJSON(ctx, &replay, JSONLimits{}, options); !errors.Is(err, geom.ErrConvergence) || out != nil {
			t.Fatal("a changed exterior owner was admitted on reload", err)
		}
		solid := loaded.Boundary()
		m, err := solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000})
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Vertices) < len(solid.vertices) || !slices.Equal(m.Vertices[:len(solid.vertices)], solid.vertices) ||
			math.Abs(m.SignedVolume()-821259.0/65536) > .001*(821259.0/65536) {
			t.Fatal("Fuse display changed original vertex or material coverage", len(m.Vertices), m.SignedVolume())
		}
		if _, err := collision.NewMeshPointLocator(ctx, m, nil, collision.LocationOptions{}); err != nil {
			t.Fatal("Fuse display does not preserve an embedded closed shell", err)
		}
		t.Logf("Fuse order %d displays %d triangles, volume %.15g", order, len(m.Triangles), m.SignedVolume())
	}
}

func TestNativeNodalCubicFuseRefusesUnrepresentedGeometry(t *testing.T) {
	ctx := context.Background()
	src := nodalCubicStockFixture(t, [4]float64{-20.0 / 256, 12.0 / 256, -7.0 / 256, 4.0 / 256}, 81.0/1024)
	if out, err := FuseNodalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, NodalCubicFuseOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("unrepresentable turn was rounded into a Fuse", err)
	}
}
